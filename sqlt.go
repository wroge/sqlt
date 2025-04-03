package sqlt

import (
	"context"
	"database/sql"
	"encoding"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
	"time"

	"github.com/cespare/xxhash/v2"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jba/templatecheck"
)

type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error)
}

type Option interface {
	Configure(config *Config)
}

type Config struct {
	Placeholder Placeholder
	Templates   []Template
	Log         Log
	Cache       *Cache
	Hasher      Hasher
}

func (c Config) Configure(config *Config) {
	if c.Placeholder != "" {
		config.Placeholder = c.Placeholder
	}

	if len(c.Templates) > 0 {
		config.Templates = append(config.Templates, c.Templates...)
	}

	if c.Log != nil {
		config.Log = c.Log
	}

	if c.Cache != nil {
		config.Cache = c.Cache
	}

	if c.Hasher != nil {
		config.Hasher = c.Hasher
	}
}

// Cache controls expression caching.
// Size ≤ 0 means unlimited cache.
// Expiration ≤ 0 prevents expiration.
type Cache struct {
	Size       int
	Expiration time.Duration
}

// Configure applies Config settings.
func (c Cache) Configure(config *Config) {
	config.Cache = &c
}

// NoCache disables caching.
func NoCache() *Cache {
	return nil
}

// NoExpirationCache enables a non-expiring cache.
func NoExpirationCache(size int) *Cache {
	return &Cache{
		Size:       size,
		Expiration: 0,
	}
}

// UnlimitedSizeCache enables an unlimited-size cache.
func UnlimitedSizeCache(expiration time.Duration) *Cache {
	return &Cache{
		Size:       0,
		Expiration: expiration,
	}
}

// Hasher generates cache keys for parameters.
type Hasher func(param any, writer io.Writer) error

// Configure applies Config settings.
func (h Hasher) Configure(config *Config) {
	config.Hasher = h
}

// DefaultHasher encodes parameters as JSON for caching.
func DefaultHasher() Hasher {
	return func(param any, writer io.Writer) error {
		return json.NewEncoder(writer).Encode(param)
	}
}

// Placeholder defines static or positional (`%d`) placeholders.
// Default: `'?'`.
type Placeholder string

// Configure applies Config settings.
func (p Placeholder) Configure(config *Config) {
	config.Placeholder = p
}

const (
	// Question is the default placeholder.
	Question Placeholder = "?"
	// Dollar uses positional placeholders ($1, $2).
	Dollar Placeholder = "$%d"
	// Colon uses positional placeholders (:1, :2).
	Colon Placeholder = ":%d"
	// AtP uses positional placeholders (@p1, @p2).
	AtP Placeholder = "@p%d"
)

// Template modifies a text/template.Template.
type Template func(t *template.Template) (*template.Template, error)

// Configure applies Config settings.
func (to Template) Configure(config *Config) {
	config.Templates = append(config.Templates, to)
}

// Name creates a named template.
func Name(name string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.New(name), nil
	}
}

// Parse parses a template string.
func Parse(text string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Parse(text)
	}
}

// ParseFS loads templates from a filesystem.
func ParseFS(fs fs.FS, patterns ...string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFS(fs, patterns...)
	}
}

// ParseFiles loads templates from files.
func ParseFiles(filenames ...string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFiles(filenames...)
	}
}

// ParseGlob loads templates matching a pattern.
func ParseGlob(pattern string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseGlob(pattern)
	}
}

// Funcs adds custom functions to a template.
func Funcs(fm template.FuncMap) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Funcs(fm), nil
	}
}

// MissingKeyInvalid treats missing keys as errors.
func MissingKeyInvalid() Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=invalid"), nil
	}
}

// MissingKeyZero replaces missing keys with zero values.
func MissingKeyZero() Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=zero"), nil
	}
}

// MissingKeyError throws an error on missing keys.
func MissingKeyError() Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=error"), nil
	}
}

// Lookup retrieves a named template.
func Lookup(name string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		tpl = tpl.Lookup(name)
		if tpl == nil {
			return nil, fmt.Errorf("template %s not found", name)
		}

		return tpl, nil
	}
}

// Log can be used to apply logging.
type Log func(ctx context.Context, info Info)

// Configure applies Config settings.
func (l Log) Configure(config *Config) {
	config.Log = l
}

// Info contains loggable execution details.
type Info struct {
	Duration    time.Duration
	Mode        Mode
	Template    string
	Location    string
	SQL         string
	Args        []any
	Err         error
	Cached      bool
	Transaction bool
}

// Mode identifies SQL statement types.
type Mode string

const (
	// ExecMode for 'Exec' statements.
	ExecMode Mode = "Exec"
	// QueryRowMode for 'QueryRow' statements.
	QueryRowMode Mode = "QueryRow"
	// QueryMode for 'Query' statements.
	QueryMode Mode = "Query"
	// FirstMode for 'First' statements.
	FirstMode Mode = "First"
	// OneMode for 'One' statements.
	OneMode Mode = "One"
	// AllMode for 'All' statements.
	AllMode Mode = "All"
)

type Expression[Dest any] struct {
	SQL      string
	Args     []any
	Scanners []Scanner[Dest]
}

func (e Expression[Dest]) DestMapper(rows *sql.Rows) ([]any, func(dest *Dest) error, error) {
	if len(e.Scanners) == 0 {
		columns, err := rows.ColumnTypes()
		if err != nil {
			return nil, nil, err
		}

		e.Scanners = make([]Scanner[Dest], len(columns))

		for i, c := range columns {
			a, err := NewAccessor[Dest](c.Name())
			if err != nil {
				if len(columns) != 1 {
					return nil, nil, err
				}

				a, err = NewAccessor[Dest]("")
				if err != nil {
					return nil, nil, err
				}
			}

			nullable, _ := c.Nullable()

			e.Scanners[i], err = a.ScanColumn(nullable)
			if err != nil {
				return nil, nil, err
			}
		}
	}

	var (
		values  = make([]any, len(e.Scanners))
		mappers = make([]func(*Dest) error, len(e.Scanners))
	)

	for i, d := range e.Scanners {
		values[i], mappers[i] = d()
	}

	return values, func(dest *Dest) error {
		for _, m := range mappers {
			if m != nil {
				if err := m(dest); err != nil {
					return err
				}
			}
		}

		return nil
	}, nil
}

func (e Expression[Dest]) First(ctx context.Context, db DB) (first Dest, err error) {
	if len(e.SQL) == 0 {
		return first, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return first, err
	}

	defer func() {
		err = errors.Join(err, rows.Close(), rows.Err())
	}()

	if !rows.Next() {
		return first, sql.ErrNoRows
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return first, err
	}

	if err = rows.Scan(values...); err != nil {
		return first, err
	}

	if err = mapper(&first); err != nil {
		return first, err
	}

	return first, nil
}

var ErrTooManyRows = errors.New("too many rows")

func (e Expression[Dest]) One(ctx context.Context, db DB) (one Dest, err error) {
	if len(e.SQL) == 0 {
		return one, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return one, err
	}

	defer func() {
		err = errors.Join(err, rows.Close(), rows.Err())
	}()

	if !rows.Next() {
		return one, sql.ErrNoRows
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return one, err
	}

	if err = rows.Scan(values...); err != nil {
		return one, err
	}

	if err = mapper(&one); err != nil {
		return one, err
	}

	if rows.Next() {
		return one, ErrTooManyRows
	}

	return one, nil
}

func (e Expression[Dest]) All(ctx context.Context, db DB) (all []Dest, err error) {
	if len(e.SQL) == 0 {
		return nil, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = errors.Join(err, rows.Close(), rows.Err())
	}()

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return nil, err
	}

	for rows.Next() {
		if err = rows.Scan(values...); err != nil {
			return nil, err
		}

		var dest Dest

		if err = mapper(&dest); err != nil {
			return nil, err
		}

		all = append(all, dest)
	}

	return all, nil
}

type Scanner[Dest any] func() (any, func(dest *Dest) error)

type Raw string

type ContextKey string

type ContextStatement[Param any] interface {
	ExecContext(ctx context.Context, db DB, param Param) (result context.Context, err error)
}

func Transaction[Param any](txOpts *sql.TxOptions, stmts ...ContextStatement[Param]) *TransactionStatement[Param] {
	return &TransactionStatement[Param]{
		txOpts: txOpts,
		stmts:  stmts,
	}
}

type TransactionStatement[Param any] struct {
	txOpts *sql.TxOptions
	stmts  []ContextStatement[Param]
}

type TxBeginner interface {
	BeginTx(ctx context.Context, opts *sql.TxOptions) (*sql.Tx, error)
}

func (ts *TransactionStatement[Param]) Exec(ctx context.Context, db TxBeginner, param Param) (result context.Context, err error) {
	tx, err := db.BeginTx(ctx, ts.txOpts)
	if err != nil {
		return result, err
	}

	defer func() {
		if err != nil {
			err = errors.Join(err, tx.Rollback())
		} else {
			err = tx.Commit()
		}
	}()

	result = ctx

	for _, d := range ts.stmts {
		ctx, err = d.ExecContext(result, tx, param)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			return result, err
		}

		if ctx != result {
			result = ctx
		}
	}

	return result, nil
}

func Exec[Param any](opts ...Option) *Statement[Param, any, sql.Result] {
	return Stmt[Param](getLocation(), ExecMode, func(ctx context.Context, db DB, expr Expression[any]) (sql.Result, error) {
		return db.ExecContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

func QueryRow[Param any](opts ...Option) *Statement[Param, any, *sql.Row] {
	return Stmt[Param](getLocation(), QueryRowMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Row, error) {
		return db.QueryRowContext(ctx, expr.SQL, expr.Args...), nil
	}, opts...)
}

func Query[Param any](opts ...Option) *Statement[Param, any, *sql.Rows] {
	return Stmt[Param](getLocation(), QueryMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Rows, error) {
		return db.QueryContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

func First[Param any, Dest any](opts ...Option) *Statement[Param, Dest, Dest] {
	return Stmt[Param](getLocation(), FirstMode, func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.First(ctx, db)
	}, opts...)
}

func One[Param any, Dest any](opts ...Option) *Statement[Param, Dest, Dest] {
	return Stmt[Param](getLocation(), OneMode, func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.One(ctx, db)
	}, opts...)
}

func All[Param any, Dest any](opts ...Option) *Statement[Param, Dest, []Dest] {
	return Stmt[Param](getLocation(), AllMode, func(ctx context.Context, db DB, expr Expression[Dest]) ([]Dest, error) {
		return expr.All(ctx, db)
	}, opts...)
}

func Stmt[Param any, Dest any, Result any](location string, mode Mode, exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), opts ...Option) *Statement[Param, Dest, Result] {
	if location == "" {
		location = getLocation()
	}

	config := &Config{
		Placeholder: Question,
		Hasher:      DefaultHasher(),
	}

	for _, o := range opts {
		o.Configure(config)
	}

	var (
		d = NewDestinator[Dest]()

		t = template.New("").Funcs(template.FuncMap{
			"Raw": func(sql string) Raw { return Raw(sql) },
			"Context": func(key string) any {
				return ContextKey(key)
			},
			"Scan":               d.Scan,
			"ScanString":         d.ScanString,
			"ScanNullString":     d.ScanNullString,
			"ScanInt64":          d.ScanInt64,
			"ScanNullInt64":      d.ScanNullInt64,
			"ScanUint64":         d.ScanUint64,
			"ScanNullUint64":     d.ScanNullUint64,
			"ScanFloat64":        d.ScanFloat64,
			"ScanNullFloat64":    d.ScanNullFloat64,
			"ScanBool":           d.ScanBool,
			"ScanNullBool":       d.ScanNullBool,
			"ScanTime":           d.ScanTime,
			"ScanNullTime":       d.ScanNullTime,
			"ScanStringSlice":    d.ScanStringSlice,
			"ScanStringTime":     d.ScanStringTime,
			"ScanNullStringTime": d.ScanNullStringTime,
			"ScanBinary":         d.ScanBinary,
			"ScanText":           d.ScanText,
			"ScanJSON":           d.ScanJSON,
		})
		err error
	)

	for _, to := range config.Templates {
		t, err = to(t)
		if err != nil {
			panic(fmt.Errorf("parse template at %s: %w", location, err))
		}
	}

	if err = templatecheck.CheckText(t, *new(Param)); err != nil {
		panic(fmt.Errorf("check template at %s: %w", location, err))
	}

	if err = escapeNode[Dest](t, t.Tree.Root); err != nil {
		panic(fmt.Errorf("escape template at %s: %w", location, err))
	}

	t, err = t.Clone()
	if err != nil {
		panic(err)
	}

	var (
		placeholder = string(config.Placeholder)
		positional  = strings.Contains(placeholder, "%d")
	)

	pool := &sync.Pool{
		New: func() any {
			tc, _ := t.Clone()

			r := &runner[Param, Dest]{
				ctx:       context.Background(),
				tpl:       tc,
				sqlWriter: &sqlWriter{},
			}

			r.tpl.Funcs(template.FuncMap{
				"Context": func(key string) any {
					switch value := r.ctx.Value(ContextKey(key)).(type) {
					case *any:
						return *value
					default:
						return value
					}
				},
				ident: func(arg any) Raw {
					switch a := arg.(type) {
					case Raw:
						return a
					case Scanner[Dest]:
						r.scanners = append(r.scanners, a)

						return Raw("")
					default:
						r.args = append(r.args, arg)

						if positional {
							return Raw(fmt.Sprintf(placeholder, len(r.args)))
						}

						return Raw(placeholder)
					}
				},
			})

			return r
		},
	}

	var cache *expirable.LRU[uint64, Expression[Dest]]

	if config.Cache != nil {
		cache = expirable.NewLRU[uint64, Expression[Dest]](config.Cache.Size, nil, config.Cache.Expiration)
	}

	return &Statement[Param, Dest, Result]{
		name:     t.Name(),
		location: location,
		mode:     mode,
		hasher:   config.Hasher,
		cache:    cache,
		pool:     pool,
		log:      config.Log,
		exec:     exec,
	}
}

type Statement[Param any, Dest any, Result any] struct {
	name     string
	location string
	mode     Mode
	hasher   Hasher
	cache    *expirable.LRU[uint64, Expression[Dest]]
	exec     func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error)
	pool     *sync.Pool
	log      Log
}

func (d *Statement[Param, Dest, Result]) ExecContext(ctx context.Context, db DB, param Param) (result context.Context, err error) {
	res, err := d.Exec(ctx, db, param)
	if err != nil {
		return result, err
	}

	switch r := any(res).(type) {
	case context.Context:
		return r, nil
	case *sql.Rows:
		defer func() {
			err = errors.Join(err, r.Close())
		}()

		var result []any

		for r.Next() {
			var data any

			if err = r.Scan(&data); err != nil {
				return nil, err
			}

			result = append(result, data)
		}

		return context.WithValue(ctx, ContextKey(d.name), result), nil
	case *sql.Row:
		var data any

		if err = r.Scan(&data); err != nil {
			return nil, err
		}

		return context.WithValue(ctx, ContextKey(d.name), data), nil
	}

	return context.WithValue(ctx, ContextKey(d.name), res), nil
}

func (d *Statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (result Result, err error) {
	var (
		expr   Expression[Dest]
		hash   uint64
		cached bool
	)

	if d.log != nil {
		now := time.Now()

		_, inTx := db.(*sql.Tx)

		defer func() {
			d.log(ctx, Info{
				Template:    d.name,
				Location:    d.location,
				Duration:    time.Since(now),
				Mode:        d.mode,
				SQL:         expr.SQL,
				Args:        expr.Args,
				Err:         err,
				Cached:      cached,
				Transaction: inTx,
			})
		}()
	}

	if d.cache != nil {
		hasher := hashPool.Get().(*xxhash.Digest)
		defer func() {
			hasher.Reset()
			hashPool.Put(hasher)
		}()

		err = d.hasher(param, hasher)
		if err != nil {
			return result, fmt.Errorf("statement at %s: %w", d.location, err)
		}

		hash = hasher.Sum64()

		expr, cached = d.cache.Get(hash)
		if cached {
			result, err = d.exec(ctx, db, expr)
			if err != nil {
				return result, fmt.Errorf("statement at %s: %w", d.location, err)
			}

			return result, nil
		}
	}

	r := d.pool.Get().(*runner[Param, Dest])
	defer func() {
		r.reset()
		d.pool.Put(r)
	}()

	r.ctx = ctx

	expr, err = r.expr(param)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", d.location, err)
	}

	if d.cache != nil {
		_ = d.cache.Add(hash, expr)
	}

	result, err = d.exec(ctx, db, expr)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", d.location, err)
	}

	return result, nil
}

func NewDestinator[Dest any]() *Destinator[Dest] {
	return &Destinator[Dest]{
		store: map[string]Scanner[Dest]{},
		typ:   reflect.TypeFor[Dest](),
	}
}

type Destinator[Dest any] struct {
	mu    sync.RWMutex
	store map[string]Scanner[Dest]
	typ   reflect.Type
}

type Accessor[Dest any] struct {
	Type   reflect.Type
	Access func(*Dest) reflect.Value
}

func NewAccessor[Dest any](field string) (Accessor[Dest], error) {
	t := reflect.TypeFor[Dest]()

	indices := []int{}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()

		indices = append(indices, -1)

		continue
	}

	if field != "" {

		parts := strings.Split(field, ".")

		for _, part := range parts {
			switch t.Kind() {
			default:
				return Accessor[Dest]{}, fmt.Errorf("invalid field access on type %s", t.Name())
			case reflect.Struct:
				sf, found := t.FieldByName(part)
				if !found {
					part = strings.ReplaceAll(part, "_", "")

					for i := range t.NumField() {
						sf = t.Field(i)

						if strings.ToLower(sf.Name) == part {
							found = true

							break
						}
					}

					if !found {
						return Accessor[Dest]{}, fmt.Errorf("field %s not found in struct %s", field, t.Name())
					}
				}

				if !sf.IsExported() {
					return Accessor[Dest]{}, fmt.Errorf("field %s in struct %s is not exported", field, t.Name())
				}

				indices = append(indices, sf.Index[0])
				t = sf.Type
			}

			for t.Kind() == reflect.Pointer {
				t = t.Elem()

				indices = append(indices, -1)

				continue
			}
		}
	}

	return Accessor[Dest]{
		Type: t,
		Access: func(d *Dest) reflect.Value {
			v := reflect.ValueOf(d).Elem()

			for _, idx := range indices {
				if idx < 0 {
					if v.IsNil() {
						v.Set(reflect.New(v.Type().Elem()))
					}

					v = v.Elem()

					continue
				}

				v = v.Field(idx)
			}

			return v
		},
	}, nil
}

func (d *Destinator[Dest]) Cache(key string, field string, f func(a Accessor[Dest]) (Scanner[Dest], error)) (Scanner[Dest], error) {
	d.mu.RLock()
	scanner, ok := d.store[key]
	if ok {
		d.mu.RUnlock()
		return scanner, nil
	}
	d.mu.RUnlock()

	a, err := NewAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	scanner, err = f(a)
	if err != nil {
		return nil, err
	}

	d.mu.Lock()
	d.store[key] = scanner
	d.mu.Unlock()

	return scanner, nil
}

func (d *Destinator[Dest]) ScanColumn(field string, nullable bool) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Column:%s:%t", field, nullable), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanColumn(nullable)
	})
}

var scannerType = reflect.TypeFor[sql.Scanner]()

func (a Accessor[Dest]) ScanColumn(nullable bool) (Scanner[Dest], error) {
	pointerType := reflect.PointerTo(a.Type)

	if pointerType.Implements(scannerType) {
		return a.Scan()
	}

	switch a.Type.Kind() {
	case reflect.String:
		return a.ScanString(nullable, "")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.ScanInt64(nullable, 0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.ScanUint64(nullable, 0)
	case reflect.Float32, reflect.Float64:
		return a.ScanFloat64(nullable, 0)
	case reflect.Bool:
		return a.ScanBool(nullable, false)
	}

	if pointerType.Implements(textUnmarshalerType) {
		return a.ScanText()
	}

	if pointerType.Implements(binaryUnmarshalerType) {
		return a.ScanBinary()
	}

	return a.ScanJSON()
}

func (d *Destinator[Dest]) Scan(field string) (scanner Scanner[Dest], err error) {
	return d.Cache(field, field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.Scan()
	})
}

func (a Accessor[Dest]) Scan() (Scanner[Dest], error) {
	pointerType := reflect.PointerTo(a.Type)

	if !pointerType.Implements(scannerType) {
		return nil, fmt.Errorf("type %s doesn't implement sql.Scanner", a.Type)
	}

	return func() (any, func(dest *Dest) error) {
		var src any

		return &src, func(dest *Dest) error {
			return a.Access(dest).Addr().Interface().(sql.Scanner).Scan(src)
		}
	}, nil
}

func (d *Destinator[Dest]) ScanString(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("String:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanString(false, "")
	})
}

func (d *Destinator[Dest]) ScanNullString(field string, def string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullString:%s:%s", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanString(true, def)
	})
}

func (a Accessor[Dest]) ScanString(nullable bool, def string) (Scanner[Dest], error) {
	if a.Type.Kind() != reflect.String {
		return nil, fmt.Errorf("cannot set string in type %s", a.Type)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *string

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).SetString(def)

					return nil
				}

				a.Access(dest).SetString(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src string

		return &src, func(dest *Dest) error {
			a.Access(dest).SetString(src)

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanInt64(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Int64:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanInt64(false, 0)
	})
}

func (d *Destinator[Dest]) ScanNullInt64(field string, def int64) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullInt64:%s:%d", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanInt64(true, def)
	})
}

func (a Accessor[Dest]) ScanInt64(nullable bool, def int64) (Scanner[Dest], error) {
	switch a.Type.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	default:
		return nil, fmt.Errorf("cannot set int64 in type %s", a.Type)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *int64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).SetInt(def)

					return nil
				}

				a.Access(dest).SetInt(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src int64

		return &src, func(dest *Dest) error {
			a.Access(dest).SetInt(src)

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanUint64(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Uint64:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanUint64(false, 0)
	})
}

func (d *Destinator[Dest]) ScanNullUint64(field string, def uint64) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullUint64:%s:%d", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanUint64(true, def)
	})
}

func (a Accessor[Dest]) ScanUint64(nullable bool, def uint64) (Scanner[Dest], error) {
	switch a.Type.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return nil, fmt.Errorf("cannot set uint64 in type %s", a.Type)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *uint64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).SetUint(def)

					return nil
				}

				a.Access(dest).SetUint(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src uint64

		return &src, func(dest *Dest) error {
			a.Access(dest).SetUint(src)

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanFloat64(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Float64:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanFloat64(false, 0)
	})
}

func (d *Destinator[Dest]) ScanNullFloat64(field string, def float64) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullFloat64:%s:%f", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanFloat64(true, def)
	})
}

func (a Accessor[Dest]) ScanFloat64(nullable bool, def float64) (Scanner[Dest], error) {
	switch a.Type.Kind() {
	case reflect.Float32, reflect.Float64:
	default:
		return nil, fmt.Errorf("cannot set float64 in type %s", a.Type)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *float64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).SetFloat(def)

					return nil
				}

				a.Access(dest).SetFloat(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src float64

		return &src, func(dest *Dest) error {
			a.Access(dest).SetFloat(src)

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanBool(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Bool:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanBool(false, false)
	})
}

func (d *Destinator[Dest]) ScanNullBool(field string, def bool) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullBool:%s:%t", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanBool(true, def)
	})
}

func (a Accessor[Dest]) ScanBool(nullable bool, def bool) (Scanner[Dest], error) {
	if a.Type.Kind() != reflect.Bool {
		return nil, fmt.Errorf("cannot set bool in type %s", a.Type)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *bool

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).SetBool(def)

					return nil
				}

				a.Access(dest).SetBool(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src bool

		return &src, func(dest *Dest) error {
			a.Access(dest).SetBool(src)

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanTime(field string) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("Time:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanTime(false, time.Time{})
	})
}

func (d *Destinator[Dest]) ScanNullTime(field string, def time.Time) (Scanner[Dest], error) {
	return d.Cache(fmt.Sprintf("NullTime:%s:%s", field, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanTime(true, def)
	})
}

var timeType = reflect.TypeFor[time.Time]()

func (a Accessor[Dest]) ScanTime(nullable bool, def time.Time) (Scanner[Dest], error) {
	if a.Type != timeType {
		return nil, fmt.Errorf("type %s is not time.Time", a.Type)
	}

	if nullable {
		value := reflect.ValueOf(def)

		return func() (any, func(dest *Dest) error) {
			var src *time.Time

			return &src, func(dest *Dest) error {
				if src == nil {
					a.Access(dest).Set(value)

					return nil
				}

				a.Access(dest).Set(reflect.ValueOf(*src))

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src time.Time

		return &src, func(dest *Dest) error {
			a.Access(dest).Set(reflect.ValueOf(src))

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanJSON(field string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("JSON:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanJSON()
	})
}

var byteSliceType = reflect.TypeFor[[]byte]()

func (a Accessor[Dest]) ScanJSON() (scanner Scanner[Dest], err error) {
	if a.Type == byteSliceType {
		return func() (any, func(dest *Dest) error) {
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				var raw json.RawMessage

				if err := json.Unmarshal(src, &raw); err != nil {
					return err
				}

				a.Access(dest).SetBytes(raw)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src []byte

		return &src, func(dest *Dest) error {
			if len(src) == 0 {
				return nil
			}

			return json.Unmarshal(src, a.Access(dest).Addr().Interface())
		}
	}, nil
}

func (d *Destinator[Dest]) ScanBinary(field string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("Binary:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanBinary()
	})
}

var binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()

func (a Accessor[Dest]) ScanBinary() (scanner Scanner[Dest], err error) {
	pointerType := reflect.PointerTo(a.Type)

	if pointerType.Implements(binaryUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				return a.Access(dest).Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(src)
			}
		}, nil
	}

	return nil, fmt.Errorf("type %s doesn't implement encoding.BinaryUnmarshaler", a.Type)
}

func (d *Destinator[Dest]) ScanText(field string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("Text:%s", field), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanText()
	})
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

func (a Accessor[Dest]) ScanText() (Scanner[Dest], error) {
	pointerType := reflect.PointerTo(a.Type)

	if pointerType.Implements(textUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				return a.Access(dest).Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(src.V)
			}
		}, nil
	}

	return nil, fmt.Errorf("type %s doesn't implement encoding.TextUnmarshaler", a.Type)
}

func (d *Destinator[Dest]) ScanStringSlice(field string, sep string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("StringSlice:%s:%s", field, sep), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanStringSlice(sep)
	})
}

func (a Accessor[Dest]) ScanStringSlice(sep string) (Scanner[Dest], error) {
	if a.Type.Kind() != reflect.Slice || a.Type.Elem().Kind() != reflect.String {
		return nil, fmt.Errorf("cannot set []string in type %s", a.Type)
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[string]

		return &src, func(dest *Dest) error {
			if !src.Valid || src.V == "" {
				return nil
			}

			split := slices.DeleteFunc(strings.Split(src.V, sep), func(d string) bool {
				return d == ""
			})

			value := a.Access(dest)

			value.Set(reflect.MakeSlice(a.Type, len(split), len(split)))

			for i, v := range split {
				value.Index(i).SetString(v)
			}

			return nil
		}
	}, nil
}

func (d *Destinator[Dest]) ScanStringTime(field string, layout string, location string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("StringTime:%s:%s:%s", field, layout, location), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanStringTime(layout, location, false, "")
	})
}

func (d *Destinator[Dest]) ScanNullStringTime(field string, layout string, location string, def string) (scanner Scanner[Dest], err error) {
	return d.Cache(fmt.Sprintf("NullStringTime:%s:%s:%s:%s", field, layout, location, def), field, func(a Accessor[Dest]) (Scanner[Dest], error) {
		return a.ScanStringTime(layout, location, true, def)
	})
}

func (a Accessor[Dest]) ScanStringTime(layout string, location string, nullable bool, def string) (Scanner[Dest], error) {
	if a.Type != timeType {
		return nil, fmt.Errorf("type %s is not time.Time", a.Type)
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, err
	}

	switch layout {
	case "Layout":
		layout = time.Layout
	case "ANSIC":
		layout = time.ANSIC
	case "UnixDate":
		layout = time.UnixDate
	case "RubyDate":
		layout = time.RubyDate
	case "RFC822":
		layout = time.RFC822
	case "RFC822Z":
		layout = time.RFC822Z
	case "RFC850":
		layout = time.RFC850
	case "RFC1123":
		layout = time.RFC1123
	case "RFC1123Z":
		layout = time.RFC1123Z
	case "RFC3339":
		layout = time.RFC3339
	case "RFC3339Nano":
		layout = time.RFC3339Nano
	case "Kitchen":
		layout = time.Kitchen
	case "Stamp":
		layout = time.Stamp
	case "StampMilli":
		layout = time.StampMilli
	case "StampMicro":
		layout = time.StampMicro
	case "StampNano":
		layout = time.StampNano
	case "DateTime":
		layout = time.DateTime
	case "DateOnly":
		layout = time.DateOnly
	case "TimeOnly":
		layout = time.TimeOnly
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *string

			return &src, func(dest *Dest) error {
				if src == nil {
					if def == "" {
						return nil
					}

					*src = def
				}

				t, err := time.ParseInLocation(layout, *src, loc)
				if err != nil {
					return err
				}

				a.Access(dest).Set(reflect.ValueOf(t))

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src string

		return &src, func(dest *Dest) error {
			t, err := time.ParseInLocation(layout, src, loc)
			if err != nil {
				return err
			}

			a.Access(dest).Set(reflect.ValueOf(t))

			return nil
		}
	}, nil
}

// idea is stolen from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func escapeNode[Dest any](t *template.Template, n parse.Node) error {
	switch v := n.(type) {
	case *parse.ActionNode:
		return escapeNode[Dest](t, v.Pipe)
	case *parse.IfNode:
		return errors.Join(
			escapeNode[Dest](t, v.List),
			escapeNode[Dest](t, v.ElseList),
		)
	case *parse.ListNode:
		if v == nil {
			return nil
		}

		for _, n := range v.Nodes {
			if err := escapeNode[Dest](t, n); err != nil {
				return err
			}
		}
	case *parse.PipeNode:
		if len(v.Decl) > 0 {
			return nil
		}

		if len(v.Cmds) < 1 {
			return nil
		}

		for _, cmd := range v.Cmds {
			if !strings.HasPrefix(cmd.Args[0].String(), "Scan") {
				continue
			}

			if len(cmd.Args) < 2 {
				continue
			}

			node, ok := cmd.Args[1].(*parse.StringNode)
			if !ok {
				continue
			}

			_, err := NewAccessor[Dest](node.Text)
			if err != nil {
				return err
			}
		}

		cmd := v.Cmds[len(v.Cmds)-1]
		if len(cmd.Args) == 1 && cmd.Args[0].Type() == parse.NodeIdentifier && cmd.Args[0].(*parse.IdentifierNode).Ident == ident {
			return nil
		}

		v.Cmds = append(v.Cmds, &parse.CommandNode{
			NodeType: parse.NodeCommand,
			Args:     []parse.Node{parse.NewIdentifier(ident).SetTree(t.Tree).SetPos(cmd.Pos)},
		})
	case *parse.RangeNode:
		return errors.Join(
			escapeNode[Dest](t, v.List),
			escapeNode[Dest](t, v.ElseList),
		)
	case *parse.WithNode:
		return errors.Join(
			escapeNode[Dest](t, v.List),
			escapeNode[Dest](t, v.ElseList),
		)
	case *parse.TemplateNode:
		tpl := t.Lookup(v.Name)

		return escapeNode[Dest](tpl, tpl.Tree.Root)
	}

	return nil
}

var hashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

func getLocation() string {
	_, file, line, _ := runtime.Caller(2)

	return fmt.Sprintf("%s:%d", file, line)
}

var ident = "__sqlt__"

type runner[Param any, Dest any] struct {
	ctx       context.Context
	tpl       *template.Template
	sqlWriter *sqlWriter
	args      []any
	scanners  []Scanner[Dest]
}

func (r *runner[Param, Dest]) reset() {
	r.ctx = context.Background()
	r.sqlWriter.reset()
	r.args = r.args[:0]
	r.scanners = r.scanners[:0]
}

func (r *runner[Param, Dest]) expr(param Param) (Expression[Dest], error) {
	if err := r.tpl.Execute(r.sqlWriter, param); err != nil {
		return Expression[Dest]{}, err
	}

	return Expression[Dest]{
		SQL:      r.sqlWriter.toString(),
		Args:     slices.Clone(r.args),
		Scanners: slices.Clone(r.scanners),
	}, nil
}

type sqlWriter struct {
	data []byte
}

func (d *sqlWriter) reset() {
	d.data = d.data[:0]
}

// Write implements io.Wr	iter.
func (d *sqlWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		switch b {
		case ' ', '\n', '\r', '\t':
			if len(d.data) > 0 && d.data[len(d.data)-1] != ' ' {
				d.data = append(d.data, ' ')
			}
		default:
			d.data = append(d.data, b)
		}
	}

	return len(data), nil
}

func (d *sqlWriter) toString() string {
	if len(d.data) == 0 {
		return ""
	}

	if d.data[len(d.data)-1] == ' ' {
		d.data = d.data[:len(d.data)-1]
	}

	return string(d.data)
}
