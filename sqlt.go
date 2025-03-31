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
func (c *Cache) Configure(config *Config) {
	config.Cache = c
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
			return nil, fmt.Errorf("template '%s' not found", name)
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

func (e Expression[Dest]) DestMapper() ([]any, func(dest *Dest) error, error) {
	if len(e.Scanners) == 0 {
		scan, err := scan[Dest]("")
		if err != nil {
			return nil, nil, err
		}

		e.Scanners = []Scanner[Dest]{scan}
	}

	var (
		values  = make([]any, len(e.Scanners))
		mappers = make([]func(*Dest) error, len(e.Scanners))
	)

	for i, s := range e.Scanners {
		values[i], mappers[i] = s()
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

	row := db.QueryRowContext(ctx, e.SQL, e.Args...)

	values, mapper, err := e.DestMapper()
	if err != nil {
		return first, err
	}

	if err = row.Scan(values...); err != nil {
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

	values, mapper, err := e.DestMapper()
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

	values, mapper, err := e.DestMapper()
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

	for _, s := range ts.stmts {
		ctx, err = s.ExecContext(result, tx, param)
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
		scannerStore = &scannerStore[Dest]{
			store: map[string]Scanner[Dest]{},
		}

		t = template.New("").Funcs(template.FuncMap{
			"Raw": func(sql string) Raw { return Raw(sql) },
			"Context": func(key string) any {
				return ContextKey(key)
			},
			"Scan":        scannerStore.scan,
			"ScanJSON":    scannerStore.scanJSON,
			"ScanBinary":  scannerStore.scanBinary,
			"ScanText":    scannerStore.scanText,
			"ScanDefault": scannerStore.scanDefault,
			"ScanSplit":   scannerStore.scanSplit,
			"ScanBitmask": scannerStore.scanBitmask,
			"ScanEnum":    scannerStore.scanEnum,
			"ScanBool":    scannerStore.scanBool,
			"ScanTime":    scannerStore.scanTime,
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

func (s *Statement[Param, Dest, Result]) ExecContext(ctx context.Context, db DB, param Param) (result context.Context, err error) {
	res, err := s.Exec(ctx, db, param)
	if err != nil {
		return result, err
	}

	switch r := any(res).(type) {
	case context.Context:
		return r, nil
	case *sql.Rows:
		data, err := scanRows(r)
		if err != nil {
			return nil, err
		}

		return context.WithValue(ctx, ContextKey(s.name), data), nil
	case *sql.Row:
		var data any

		if err = r.Scan(&data); err != nil {
			return nil, err
		}

		return context.WithValue(ctx, ContextKey(s.name), data), nil
	}

	return context.WithValue(ctx, ContextKey(s.name), res), nil
}

func scanRows(rows *sql.Rows) (result []any, err error) {
	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	cols, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	if len(cols) == 1 {
		var data any

		for rows.Next() {
			if err = rows.Scan(&data); err != nil {
				return nil, err
			}

			result = append(result, data)
		}

		return result, nil
	}

	for rows.Next() {
		items := make([]any, len(cols))
		for i := range items {
			items[i] = new(any)
		}

		if err := rows.Scan(items...); err != nil {
			return nil, err
		}

		row := make(map[string]any, len(cols))

		for i, c := range cols {
			vv := items[i].(*any)
			row[c] = *vv
		}

		result = append(result, row)
	}

	return result, nil
}

// Exec executes and optionally scans rows into the result.
func (s *Statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (result Result, err error) {
	var (
		expr   Expression[Dest]
		hash   uint64
		cached bool
	)

	if s.log != nil {
		now := time.Now()

		_, inTx := db.(*sql.Tx)

		defer func() {
			s.log(ctx, Info{
				Template:    s.name,
				Location:    s.location,
				Duration:    time.Since(now),
				Mode:        s.mode,
				SQL:         expr.SQL,
				Args:        expr.Args,
				Err:         err,
				Cached:      cached,
				Transaction: inTx,
			})
		}()
	}

	if s.cache != nil {
		hasher := hashPool.Get().(*xxhash.Digest)
		defer func() {
			hasher.Reset()
			hashPool.Put(hasher)
		}()

		err = s.hasher(param, hasher)
		if err != nil {
			return result, fmt.Errorf("statement at %s: %w", s.location, err)
		}

		hash = hasher.Sum64()

		expr, cached = s.cache.Get(hash)
		if cached {
			result, err = s.exec(ctx, db, expr)
			if err != nil {
				return result, fmt.Errorf("statement at %s: %w", s.location, err)
			}

			return result, nil
		}
	}

	r := s.pool.Get().(*runner[Param, Dest])
	defer func() {
		r.reset()
		s.pool.Put(r)
	}()

	r.ctx = ctx

	expr, err = r.expr(param)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", s.location, err)
	}

	if s.cache != nil {
		_ = s.cache.Add(hash, expr)
	}

	result, err = s.exec(ctx, db, expr)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", s.location, err)
	}

	return result, nil
}

func getDestAccessor[Dest any](field string) (reflect.Type, func(*Dest) reflect.Value, error) {
	t, acc, err := getTypeAccessor(reflect.TypeFor[Dest](), field)
	if err != nil {
		return t, nil, err
	}

	return t, func(d *Dest) reflect.Value {
		return acc(reflect.ValueOf(d).Elem())
	}, nil
}

func getTypeAccessor(t reflect.Type, field string) (reflect.Type, func(reflect.Value) reflect.Value, error) {
	indices := []int{}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()

		indices = append(indices, -1)

		continue
	}

	if field == "" {
		return t, getAccessor(indices), nil
	}

	parts := strings.Split(field, ".")

	for _, part := range parts {
		switch t.Kind() {
		default:
			return t, nil, fmt.Errorf("invalid field access on type %s", t.Name())
		case reflect.Struct:
			sf, found := t.FieldByName(part)
			if !found {
				return t, nil, fmt.Errorf("field %s not found in struct %s", field, t.Name())
			}

			if !sf.IsExported() {
				return t, nil, fmt.Errorf("field %s in struct %s is not exported", field, t.Name())
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

	return t, getAccessor(indices), nil
}

func getAccessor(indices []int) func(reflect.Value) reflect.Value {
	return func(v reflect.Value) reflect.Value {
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
	}
}

var (
	scannerType = reflect.TypeFor[sql.Scanner]()
	timeType    = reflect.TypeFor[time.Time]()
)

type scannerStore[Dest any] struct {
	mu        sync.RWMutex
	store     map[string]Scanner[Dest]
}

func (s *scannerStore[Dest]) get(key string) (Scanner[Dest], bool) {
	s.mu.RLock()
	scanner, ok := s.store[key]
	s.mu.RUnlock()

	return scanner, ok
}

func (s *scannerStore[Dest]) set(key string, scanner Scanner[Dest]) {
	s.mu.Lock()
	s.store[key] = scanner
	s.mu.Unlock()
}

func scan[Dest any](field string) (scanner Scanner[Dest], err error) {
	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	pointerType := reflect.PointerTo(typ)

	if pointerType.Implements(scannerType) {
		return func() (any, func(dest *Dest) error) {
			var src any

			return &src, func(dest *Dest) error {
				return acc(dest).Addr().Interface().(sql.Scanner).Scan(src)
			}
		}, nil
	}

	switch typ.Kind() {
	case reflect.String:
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[string]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).SetString(src.V)

				return nil
			}
		}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[int64]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).SetInt(src.V)

				return nil
			}
		}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[uint64]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).SetUint(src.V)

				return nil
			}
		}, nil
	case reflect.Float32, reflect.Float64:
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[float64]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).SetFloat(src.V)

				return nil
			}
		}, nil
	case reflect.Bool:
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[bool]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).SetBool(src.V)

				return nil
			}
		}, nil
	}

	if typ == timeType {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[time.Time]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				acc(dest).Set(reflect.ValueOf(src.V).Convert(typ))

				return nil
			}
		}, nil
	}

	return nil, fmt.Errorf("invalid type %s for Scan: want string|int|float|bool|time.Time|sql.Scanner", typ)
}

func (s *scannerStore[Dest]) scan(field string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = "scan:" + field
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	return scan[Dest](field)
}

var byteSliceType = reflect.TypeFor[[]byte]()

func (s *scannerStore[Dest]) scanJSON(field string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = "scanJSON:" + field
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ == byteSliceType {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				var raw json.RawMessage

				if err := json.Unmarshal(src.V, &raw); err != nil {
					return err
				}

				acc(dest).SetBytes(raw)

				return nil
			}
		}, nil
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[[]byte]

		return &src, func(dest *Dest) error {
			if !src.Valid {
				return nil
			}

			return json.Unmarshal(src.V, acc(dest).Addr().Interface())
		}
	}, nil
}

var binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()

func (s *scannerStore[Dest]) scanBinary(field string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = "scanBinary:" + field
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	pointerType := reflect.PointerTo(typ)

	if pointerType.Implements(binaryUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				return acc(dest).Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(src.V)
			}
		}, nil
	}

	return nil, fmt.Errorf("invalid type %s for ScanBinary: want encoding.BinaryUnmarshaler", typ)
}

var textUnmarshalerType = reflect.TypeFor[encoding.TextUnmarshaler]()

func (s *scannerStore[Dest]) scanText(field string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = "scanText:" + field
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	pointerType := reflect.PointerTo(typ)

	if pointerType.Implements(textUnmarshalerType) {
		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				return acc(dest).Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(src.V)
			}
		}, nil
	}

	return nil, fmt.Errorf("invalid type %s for ScanText: want encoding.TextUnmarshaler", typ)
}

func (s *scannerStore[Dest]) scanDefault(field string, value reflect.Value) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanDefault:%s:%v", field, value.Interface())
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	switch typ.Kind() {
	case reflect.String:
		if value.Kind() != reflect.String {
			return nil, fmt.Errorf("invalid default value %s for ScanDefault: want string", value)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[string]

			return &src, func(dest *Dest) error {
				if src.Valid {
					acc(dest).SetString(src.V)

					return nil
				}

				acc(dest).SetString(value.String())

				return nil
			}
		}, nil
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		if !value.CanInt() {
			return nil, fmt.Errorf("invalid default value %s for ScanDefault: want int", value)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[int64]

			return &src, func(dest *Dest) error {
				if src.Valid {
					acc(dest).SetInt(src.V)

					return nil
				}

				acc(dest).SetInt(value.Int())

				return nil
			}
		}, nil
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		if !value.CanUint() {
			return nil, fmt.Errorf("invalid default value %s for ScanDefault: want uint", value)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[uint64]

			return &src, func(dest *Dest) error {
				if src.Valid {
					acc(dest).SetUint(src.V)

					return nil
				}

				acc(dest).SetUint(value.Uint())

				return nil
			}
		}, nil
	case reflect.Float32, reflect.Float64:
		if !value.CanFloat() {
			return nil, fmt.Errorf("invalid default value %s for ScanDefault: want float", value)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[float64]

			return &src, func(dest *Dest) error {
				if src.Valid {
					acc(dest).SetFloat(src.V)

					return nil
				}

				acc(dest).SetFloat(value.Float())

				return nil
			}
		}, nil
	case reflect.Bool:
		if value.Kind() != reflect.Bool {
			return nil, fmt.Errorf("invalid default value %s for ScanDefault: want bool", value)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[bool]

			return &src, func(dest *Dest) error {
				if src.Valid {
					acc(dest).SetBool(src.V)

					return nil
				}

				acc(dest).SetBool(value.Bool())

				return nil
			}
		}, nil
	}

	return nil, fmt.Errorf("invalid type %s for ScanNull: want string|int|float|bool", typ)
}

func (s *scannerStore[Dest]) scanSplit(field string, sep string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanSplit:%s:%s", field, sep)
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ.Kind() != reflect.Slice || typ.Elem().Kind() != reflect.String {
		return nil, fmt.Errorf("invalid type %s for ScanSplit: want []string", typ)
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[string]

		return &src, func(dest *Dest) error {
			if !src.Valid {
				return nil
			}

			split := strings.Split(src.V, sep)
			value := acc(dest)

			value.Set(reflect.MakeSlice(typ, len(split), len(split)))

			for i, v := range split {
				value.Index(i).SetString(v)
			}

			return nil
		}
	}, nil
}

func (s *scannerStore[Dest]) scanBitmask(field string, values ...string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanBitmask:%s:%v", field, values)
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ.Kind() != reflect.Slice || typ.Elem().Kind() != reflect.String {
		return nil, fmt.Errorf("invalid type %s for ScanBitmask: want []string", typ)
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[int]

		return &src, func(dest *Dest) error {
			if !src.Valid {
				return nil
			}

			value := acc(dest)

			collect := reflect.MakeSlice(typ, 0, 0)

			for i, v := range values {
				if src.V&(i+1) != 0 {
					collect = reflect.Append(collect, reflect.ValueOf(v).Convert(typ.Elem()))
				}
			}

			value.Set(collect)

			return nil
		}
	}, nil
}

func (s *scannerStore[Dest]) scanEnum(field string, def string, values ...string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanEnum:%s:%s:%v", field, def, values)
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ.Kind() != reflect.String {
		return nil, fmt.Errorf("invalid type %s for ScanEnum: want string", typ)
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[string]

		return &src, func(dest *Dest) error {
			if !src.Valid || src.V == def || src.V == "" {
				acc(dest).SetString(def)

				return nil
			}

			for _, v := range values {
				if v == src.V {
					acc(dest).SetString(v)

					return nil
				}
			}

			return fmt.Errorf("invalid value %s for ScanEnum: want %v", src.V, append(values, def))
		}
	}, nil
}

func (s *scannerStore[Dest]) scanBool(field string, trueValue, falseValue, nullValue string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanBool:%s:%v:%v:%v", field, trueValue, falseValue, nullValue)
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ.Kind() != reflect.String {
		return nil, fmt.Errorf("invalid type %s for ScanBool: want string", typ)
	}

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[bool]

		return &src, func(dest *Dest) error {
			if !src.Valid {
				acc(dest).SetString(nullValue)

				return nil
			}

			if src.V {
				acc(dest).SetString(trueValue)

				return nil
			}

			acc(dest).SetString(falseValue)

			return nil
		}
	}, nil
}

func (s *scannerStore[Dest]) scanTime(field string, layout string, location string) (scanner Scanner[Dest], err error) {
	var (
		ok  bool
		key = fmt.Sprintf("scanTime:%s:%s:%s", field, layout, location)
	)

	scanner, ok = s.get(key)
	if ok {
		return scanner, nil
	}

	defer func() {
		if err == nil {
			s.set(key, scanner)
		}
	}()

	typ, acc, err := getDestAccessor[Dest](field)
	if err != nil {
		return nil, err
	}

	if typ != timeType {
		return nil, fmt.Errorf("invalid type %s for ScanTime: want time.Time", typ)
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

	return func() (any, func(dest *Dest) error) {
		var src sql.Null[string]

		return &src, func(dest *Dest) error {
			if !src.Valid {
				return nil
			}

			t, err := time.ParseInLocation(layout, src.V, loc)
			if err != nil {
				return err
			}

			acc(dest).Set(reflect.ValueOf(t))

			return nil
		}
	}, nil
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

func (s *sqlWriter) reset() {
	s.data = s.data[:0]
}

// Write implements io.Wr	iter.
func (s *sqlWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		switch b {
		case ' ', '\n', '\r', '\t':
			if len(s.data) > 0 && s.data[len(s.data)-1] != ' ' {
				s.data = append(s.data, ' ')
			}
		default:
			s.data = append(s.data, b)
		}
	}

	return len(data), nil
}

func (s *sqlWriter) toString() string {
	if len(s.data) == 0 {
		return ""
	}

	if s.data[len(s.data)-1] == ' ' {
		s.data = s.data[:len(s.data)-1]
	}

	return string(s.data)
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

			_, _, err := getDestAccessor[Dest](node.Text)
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
