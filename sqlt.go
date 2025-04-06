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
	"strconv"
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

		t := reflect.TypeFor[Dest]()

		for i, c := range columns {
			a, err := makeAccessor[Dest](t, c.Name())
			if err != nil {
				if len(columns) != 1 {
					return nil, nil, err
				}

				a, err = makeAccessor[Dest](t, "")
				if err != nil {
					return nil, nil, err
				}
			}

			nullable, known := c.Nullable()

			e.Scanners[i], err = a.scanColumn(c.ScanType(), nullable || !known)
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

func (e Expression[Dest]) First(ctx context.Context, db DB) (Dest, error) {
	var first Dest

	if len(e.SQL) == 0 {
		return first, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return first, err
	}

	if !rows.Next() {
		return first, errors.Join(sql.ErrNoRows, rows.Close())
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return first, errors.Join(err, rows.Close())
	}

	if err = rows.Scan(values...); err != nil {
		return first, errors.Join(err, rows.Close())
	}

	if err = mapper(&first); err != nil {
		return first, errors.Join(err, rows.Close())
	}

	return first, errors.Join(rows.Close(), rows.Err())
}

var ErrTooManyRows = errors.New("too many rows")

func (e Expression[Dest]) One(ctx context.Context, db DB) (Dest, error) {
	var one Dest

	if len(e.SQL) == 0 {
		return one, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return one, err
	}

	if !rows.Next() {
		return one, errors.Join(sql.ErrNoRows, rows.Close())
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return one, errors.Join(err, rows.Close())
	}

	if err = rows.Scan(values...); err != nil {
		return one, errors.Join(err, rows.Close())
	}

	if err = mapper(&one); err != nil {
		return one, errors.Join(err, rows.Close())
	}

	if rows.Next() {
		return one, errors.Join(ErrTooManyRows, rows.Close())
	}

	return one, errors.Join(rows.Close(), rows.Err())
}

func (e Expression[Dest]) All(ctx context.Context, db DB) ([]Dest, error) {
	if len(e.SQL) == 0 {
		return nil, sql.ErrNoRows
	}

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return nil, err
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		return nil, errors.Join(err, rows.Close())
	}

	var all []Dest

	for rows.Next() {
		if err = rows.Scan(values...); err != nil {
			return nil, errors.Join(err, rows.Close())
		}

		var dest Dest

		if err = mapper(&dest); err != nil {
			return nil, errors.Join(err, rows.Close())
		}

		all = append(all, dest)
	}

	return all, errors.Join(rows.Close(), rows.Err())
}

type Scanner[Dest any] func() (any, func(dest *Dest) error)

type Raw string

type ContextKey string

type ContextStatement[Param any] interface {
	ExecContext(ctx context.Context, db DB, param Param) (context.Context, error)
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

func (ts *TransactionStatement[Param]) Exec(ctx context.Context, db TxBeginner, param Param) (context.Context, error) {
	tx, err := db.BeginTx(ctx, ts.txOpts)
	if err != nil {
		return ctx, err
	}

	for _, d := range ts.stmts {
		ctx, err = d.ExecContext(ctx, tx, param)
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				continue
			}

			return ctx, errors.Join(err, tx.Rollback())
		}
	}

	return ctx, tx.Commit()
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
	}

	for _, o := range opts {
		o.Configure(config)
	}

	var (
		d = newDestinator[Dest]()

		t = template.New("").Funcs(template.FuncMap{
			"Raw": func(sql string) Raw { return Raw(sql) },
			"Context": func(key string) any {
				return ContextKey(key)
			},
			"Scan":               d.scan,
			"ScanString":         d.scanString,
			"ScanNullString":     d.scanNullString,
			"ScanInt64":          d.scanInt64,
			"ScanNullInt64":      d.scanNullInt64,
			"ScanUint64":         d.scanUint64,
			"ScanNullUint64":     d.scanNullUint64,
			"ScanFloat64":        d.scanFloat64,
			"ScanNullFloat64":    d.scanNullFloat64,
			"ScanBool":           d.scanBool,
			"ScanNullBool":       d.scanNullBool,
			"ScanTime":           d.scanTime,
			"ScanNullTime":       d.scanNullTime,
			"ScanStringSlice":    d.scanStringSlice,
			"ScanStringTime":     d.scanStringTime,
			"ScanNullStringTime": d.scanNullStringTime,
			"ScanBinary":         d.scanBinary,
			"ScanText":           d.scanText,
			"ScanJSON":           d.scanJSON,
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

	if err = d.escapeNode(t, t.Tree.Root); err != nil {
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
					return r.ctx.Value(ContextKey(key))
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
							return Raw(placeholder + strconv.Itoa(len(r.args)))
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

		if config.Hasher == nil {
			config.Hasher = DefaultHasher()
		}
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

func (d *Statement[Param, Dest, Result]) ExecContext(ctx context.Context, db DB, param Param) (context.Context, error) {
	res, err := d.execOptions(ctx, db, param, false)
	if err != nil {
		return ctx, err
	}

	switch r := any(res).(type) {
	case context.Context:
		return r, nil
	case *sql.Rows:
		var result []any

		for r.Next() {
			var data any

			if err = r.Scan(&data); err != nil {
				return nil, errors.Join(err, r.Close())
			}

			result = append(result, data)
		}

		return context.WithValue(ctx, ContextKey(d.name), result), errors.Join(r.Close(), r.Err())
	case *sql.Row:
		var data any

		if err = r.Scan(&data); err != nil {
			return nil, err
		}

		return context.WithValue(ctx, ContextKey(d.name), data), nil
	}

	return context.WithValue(ctx, ContextKey(d.name), res), nil
}

func (d *Statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (Result, error) {
	return d.execOptions(ctx, db, param, d.cache != nil && d.hasher != nil)
}

func (d *Statement[Param, Dest, Result]) execOptions(ctx context.Context, db DB, param Param, withCache bool) (result Result, err error) {
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

	if withCache {
		hasher := hashPool.Get().(*xxhash.Digest)

		hasher.Reset()

		err = d.hasher(param, hasher)
		if err != nil {
			return result, fmt.Errorf("statement at %s: %w", d.location, err)
		}

		hash = hasher.Sum64()

		hashPool.Put(hasher)

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

	r.ctx = ctx

	expr, err = r.expr(param)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", d.location, err)
	}

	d.pool.Put(r)

	if withCache {
		_ = d.cache.Add(hash, expr)
	}

	result, err = d.exec(ctx, db, expr)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", d.location, err)
	}

	return result, nil
}

func makeAccessor[Dest any](t reflect.Type, field string) (accessor[Dest], error) {
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
				return accessor[Dest]{}, fmt.Errorf("invalid field access on type %s", t.Name())
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
						return accessor[Dest]{}, fmt.Errorf("field %s not found in struct %s", field, t.Name())
					}
				}

				if !sf.IsExported() {
					return accessor[Dest]{}, fmt.Errorf("field %s in struct %s is not exported", field, t.Name())
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

	a := accessor[Dest]{
		typ:         t,
		pointerType: reflect.PointerTo(t),
		access: func(d *Dest) reflect.Value {
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
	}

	return a, nil
}

type accessor[Dest any] struct {
	typ         reflect.Type
	pointerType reflect.Type
	access      func(*Dest) reflect.Value
}

var scannerType = reflect.TypeFor[sql.Scanner]()

func (a accessor[Dest]) scanColumn(scanType reflect.Type, nullable bool) (Scanner[Dest], error) {
	if a.pointerType.Implements(scannerType) {
		return a.scan()
	}

	switch a.typ.Kind() {
	case reflect.String:
		return a.scanString(nullable, "")
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return a.scanInt64(nullable, 0)
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return a.scanUint64(nullable, 0)
	case reflect.Float32, reflect.Float64:
		return a.scanFloat64(nullable, 0)
	case reflect.Bool:
		return a.scanBool(nullable, false)
	}

	if a.typ == timeType {
		if scanType == timeType {
			return a.scanTime(nullable, time.Time{})
		}

		return a.scanStringTime("", "", nullable, time.Now().Format(time.RFC3339Nano))
	}

	if a.pointerType.Implements(textUnmarshalerType) {
		return a.scanText()
	}

	if a.pointerType.Implements(binaryUnmarshalerType) {
		return a.scanBinary()
	}

	return a.scanJSON()
}

func (a accessor[Dest]) scan() (Scanner[Dest], error) {
	if !a.pointerType.Implements(scannerType) {
		return nil, fmt.Errorf("type %s doesn't implement sql.Scanner", a.typ)
	}

	return func() (any, func(dest *Dest) error) {
		var src any

		return &src, func(dest *Dest) error {
			return a.access(dest).Addr().Interface().(sql.Scanner).Scan(src)
		}
	}, nil
}

func (a accessor[Dest]) scanString(nullable bool, def string) (Scanner[Dest], error) {
	if a.typ.Kind() != reflect.String {
		return nil, fmt.Errorf("cannot set string in type %s", a.typ)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *string

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).SetString(def)

					return nil
				}

				a.access(dest).SetString(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src string

		return &src, func(dest *Dest) error {
			a.access(dest).SetString(src)

			return nil
		}
	}, nil
}

func (a accessor[Dest]) scanInt64(nullable bool, def int64) (Scanner[Dest], error) {
	switch a.typ.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
	default:
		return nil, fmt.Errorf("cannot set int64 in type %s", a.typ)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *int64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).SetInt(def)

					return nil
				}

				a.access(dest).SetInt(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src int64

		return &src, func(dest *Dest) error {
			a.access(dest).SetInt(src)

			return nil
		}
	}, nil
}

func (a accessor[Dest]) scanUint64(nullable bool, def uint64) (Scanner[Dest], error) {
	switch a.typ.Kind() {
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
	default:
		return nil, fmt.Errorf("cannot set uint64 in type %s", a.typ)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *uint64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).SetUint(def)

					return nil
				}

				a.access(dest).SetUint(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src uint64

		return &src, func(dest *Dest) error {
			a.access(dest).SetUint(src)

			return nil
		}
	}, nil
}

func (a accessor[Dest]) scanFloat64(nullable bool, def float64) (Scanner[Dest], error) {
	switch a.typ.Kind() {
	case reflect.Float32, reflect.Float64:
	default:
		return nil, fmt.Errorf("cannot set float64 in type %s", a.typ)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *float64

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).SetFloat(def)

					return nil
				}

				a.access(dest).SetFloat(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src float64

		return &src, func(dest *Dest) error {
			a.access(dest).SetFloat(src)

			return nil
		}
	}, nil
}

func (a accessor[Dest]) scanBool(nullable bool, def bool) (Scanner[Dest], error) {
	if a.typ.Kind() != reflect.Bool {
		return nil, fmt.Errorf("cannot set bool in type %s", a.typ)
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *bool

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).SetBool(def)

					return nil
				}

				a.access(dest).SetBool(*src)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src bool

		return &src, func(dest *Dest) error {
			a.access(dest).SetBool(src)

			return nil
		}
	}, nil
}

var timeType = reflect.TypeFor[time.Time]()

func (a accessor[Dest]) scanTime(nullable bool, def time.Time) (Scanner[Dest], error) {
	if a.typ != timeType {
		return nil, fmt.Errorf("type %s is not time.Time", a.typ)
	}

	if nullable {
		value := reflect.ValueOf(def)

		return func() (any, func(dest *Dest) error) {
			var src *time.Time

			return &src, func(dest *Dest) error {
				if src == nil {
					a.access(dest).Set(value)

					return nil
				}

				a.access(dest).Set(reflect.ValueOf(*src))

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src time.Time

		return &src, func(dest *Dest) error {
			a.access(dest).Set(reflect.ValueOf(src))

			return nil
		}
	}, nil
}

var byteSliceType = reflect.TypeFor[[]byte]()

func (a accessor[Dest]) scanJSON() (Scanner[Dest], error) {
	if a.typ == byteSliceType {
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

				a.access(dest).SetBytes(raw)

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

			return json.Unmarshal(src, a.access(dest).Addr().Interface())
		}
	}, nil
}

var binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()

func (a accessor[Dest]) scanBinary() (Scanner[Dest], error) {
	if a.pointerType.Implements(binaryUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				return a.access(dest).Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(src)
			}
		}, nil
	}

	return nil, fmt.Errorf("type %s doesn't implement encoding.BinaryUnmarshaler", a.typ)
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

func (a accessor[Dest]) scanText() (Scanner[Dest], error) {
	if a.pointerType.Implements(textUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				return a.access(dest).Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(src.V)
			}
		}, nil
	}

	return nil, fmt.Errorf("type %s doesn't implement encoding.TextUnmarshaler", a.typ)
}

func (a accessor[Dest]) scanStringSlice(sep string) (Scanner[Dest], error) {
	if a.typ.Kind() != reflect.Slice || a.typ.Elem().Kind() != reflect.String {
		return nil, fmt.Errorf("cannot set []string in type %s", a.typ)
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

			value := a.access(dest)

			value.Set(reflect.MakeSlice(a.typ, len(split), len(split)))

			for i, v := range split {
				value.Index(i).SetString(v)
			}

			return nil
		}
	}, nil
}

var layoutMap = map[string]string{
	"DateTime":    time.DateTime,
	"DateOnly":    time.DateOnly,
	"TimeOnly":    time.TimeOnly,
	"RFC3339":     time.RFC3339,
	"RFC3339Nano": time.RFC3339Nano,
	"Layout":      time.Layout,
	"ANSIC":       time.ANSIC,
	"UnixDate":    time.UnixDate,
	"RubyDate":    time.RubyDate,
	"RFC822":      time.RFC822,
	"RFC822Z":     time.RFC822Z,
	"RFC850":      time.RFC850,
	"RFC1123":     time.RFC1123,
	"RFC1123Z":    time.RFC1123Z,
	"Kitchen":     time.Kitchen,
	"Stamp":       time.Stamp,
	"StampMilli":  time.StampMilli,
	"StampMicro":  time.StampMicro,
	"StampNano":   time.StampNano,
}

func (a accessor[Dest]) scanStringTime(layout string, location string, nullable bool, def string) (Scanner[Dest], error) {
	if a.typ != timeType {
		return nil, fmt.Errorf("type %s is not time.Time", a.typ)
	}

	loc, err := time.LoadLocation(location)
	if err != nil {
		return nil, err
	}

	if l, ok := layoutMap[layout]; ok {
		layout = l
	}

	convert := func(str string) (time.Time, error) {
		return time.ParseInLocation(layout, str, loc)
	}

	if layout == "" {
		convert = func(str string) (time.Time, error) {
			for _, l := range layoutMap {
				t, err := time.ParseInLocation(l, str, loc)
				if err == nil {
					return t, nil
				}
			}

			return time.Time{}, fmt.Errorf("cannot parse '%s' into time.Time", str)
		}
	}

	if nullable {
		return func() (any, func(dest *Dest) error) {
			var src *string

			return &src, func(dest *Dest) error {
				if src == nil {
					if def == "" {
						return nil
					}

					src = &def
				}

				t, err := convert(*src)
				if err != nil {
					return err
				}

				a.access(dest).Set(reflect.ValueOf(t))

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src string

		return &src, func(dest *Dest) error {
			t, err := convert(src)
			if err != nil {
				return err
			}

			a.access(dest).Set(reflect.ValueOf(t))

			return nil
		}
	}, nil
}

func newDestinator[Dest any]() *destinator[Dest] {
	return &destinator[Dest]{
		store: map[string]Scanner[Dest]{},
		typ:   reflect.TypeFor[Dest](),
	}
}

type destinator[Dest any] struct {
	mu    sync.RWMutex
	store map[string]Scanner[Dest]
	typ   reflect.Type
}

func (d *destinator[Dest]) Cache(key string, field string, f func(a accessor[Dest]) (Scanner[Dest], error)) (Scanner[Dest], error) {
	d.mu.RLock()

	scanner, ok := d.store[key]
	if ok {
		d.mu.RUnlock()

		return scanner, nil
	}

	d.mu.RUnlock()

	a, err := makeAccessor[Dest](d.typ, field)
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

func (d *destinator[Dest]) scan(field string) (Scanner[Dest], error) {
	return d.Cache(field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scan()
	})
}

func (d *destinator[Dest]) scanString(field string) (Scanner[Dest], error) {
	return d.Cache("String:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanString(false, "")
	})
}

func (d *destinator[Dest]) scanNullString(field string, def string) (Scanner[Dest], error) {
	return d.Cache("NullString:"+field+":"+def, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanString(true, def)
	})
}

func (d *destinator[Dest]) scanInt64(field string) (Scanner[Dest], error) {
	return d.Cache("Int64:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanInt64(false, 0)
	})
}

func (d *destinator[Dest]) scanNullInt64(field string, def int64) (Scanner[Dest], error) {
	return d.Cache("NullInt:"+field+":"+strconv.FormatInt(def, 10), field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanInt64(true, def)
	})
}

func (d *destinator[Dest]) scanUint64(field string) (Scanner[Dest], error) {
	return d.Cache("Uint64:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanUint64(false, 0)
	})
}

func (d *destinator[Dest]) scanNullUint64(field string, def uint64) (Scanner[Dest], error) {
	return d.Cache("NullUint64:"+field+":"+strconv.FormatUint(def, 10), field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanUint64(true, def)
	})
}

func (d *destinator[Dest]) scanFloat64(field string) (Scanner[Dest], error) {
	return d.Cache("Float64:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanFloat64(false, 0)
	})
}

func (d *destinator[Dest]) scanNullFloat64(field string, def float64) (Scanner[Dest], error) {
	return d.Cache("NullFloat64:"+field+":"+strconv.FormatFloat(def, 'g', -1, 64), field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanFloat64(true, def)
	})
}

func (d *destinator[Dest]) scanBool(field string) (Scanner[Dest], error) {
	return d.Cache("Bool:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanBool(false, false)
	})
}

func (d *destinator[Dest]) scanNullBool(field string, def bool) (Scanner[Dest], error) {
	return d.Cache("NullBool:"+strconv.FormatBool(def), field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanBool(true, def)
	})
}

func (d *destinator[Dest]) scanTime(field string) (Scanner[Dest], error) {
	return d.Cache("Time:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanTime(false, time.Time{})
	})
}

func (d *destinator[Dest]) scanNullTime(field string, def time.Time) (Scanner[Dest], error) {
	return d.Cache("NullTime:"+field+":"+def.String(), field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanTime(true, def)
	})
}

func (d *destinator[Dest]) scanJSON(field string) (Scanner[Dest], error) {
	return d.Cache("JSON:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanJSON()
	})
}

func (d *destinator[Dest]) scanBinary(field string) (Scanner[Dest], error) {
	return d.Cache("Binary:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanBinary()
	})
}

func (d *destinator[Dest]) scanText(field string) (Scanner[Dest], error) {
	return d.Cache("Text:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanText()
	})
}

func (d *destinator[Dest]) scanStringSlice(field string, sep string) (Scanner[Dest], error) {
	return d.Cache("StringSlice:"+field+":"+sep, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanStringSlice(sep)
	})
}

func (d *destinator[Dest]) scanStringTime(field string, layout string, location string) (Scanner[Dest], error) {
	return d.Cache("StringTime:"+field+":"+layout+":"+location, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanStringTime(layout, location, false, "")
	})
}

func (d *destinator[Dest]) scanNullStringTime(field string, layout string, location string, def string) (Scanner[Dest], error) {
	return d.Cache("NullStringTime:"+field+":"+layout+":"+location+":"+def, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		return a.scanStringTime(layout, location, true, def)
	})
}

// idea is stolen from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func (d *destinator[Dest]) escapeNode(t *template.Template, n parse.Node) error {
	switch v := n.(type) {
	case *parse.ActionNode:
		return d.escapeNode(t, v.Pipe)
	case *parse.IfNode:
		return errors.Join(
			d.escapeNode(t, v.List),
			d.escapeNode(t, v.ElseList),
		)
	case *parse.ListNode:
		if v == nil {
			return nil
		}

		for _, n := range v.Nodes {
			if err := d.escapeNode(t, n); err != nil {
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

			_, err := makeAccessor[Dest](d.typ, node.Text)
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
			d.escapeNode(t, v.List),
			d.escapeNode(t, v.ElseList),
		)
	case *parse.WithNode:
		return errors.Join(
			d.escapeNode(t, v.List),
			d.escapeNode(t, v.ElseList),
		)
	case *parse.TemplateNode:
		tpl := t.Lookup(v.Name)

		return d.escapeNode(tpl, tpl.Tree.Root)
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

	return file + ":" + strconv.Itoa(line)
}

var ident = "__sqlt__"

type runner[Param any, Dest any] struct {
	ctx       context.Context
	tpl       *template.Template
	sqlWriter *sqlWriter
	args      []any
	scanners  []Scanner[Dest]
}

func (r *runner[Param, Dest]) expr(param Param) (Expression[Dest], error) {
	if err := r.tpl.Execute(r.sqlWriter, param); err != nil {
		return Expression[Dest]{}, err
	}

	expr := Expression[Dest]{
		SQL:      r.sqlWriter.toString(),
		Args:     r.args,
		Scanners: r.scanners,
	}

	r.ctx = context.Background()
	r.args = make([]any, 0, len(expr.Args))
	r.scanners = make([]Scanner[Dest], 0, len(expr.Scanners))

	return expr, nil
}

type sqlWriter struct {
	data []byte
}

// Write implements io.Writer.
func (w *sqlWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		switch b {
		case ' ', '\n', '\r', '\t':
			if len(w.data) > 0 && w.data[len(w.data)-1] != ' ' {
				w.data = append(w.data, ' ')
			}
		default:
			w.data = append(w.data, b)
		}
	}

	return len(data), nil
}

func (w *sqlWriter) toString() string {
	n := len(w.data)

	if n == 0 {
		return ""
	} else if w.data[n-1] == ' ' {
		n--
	}

	s := string(w.data[:n])

	w.data = w.data[:0]

	return s
}
