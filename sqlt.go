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

// DB abstracts a minimal SQL interface for querying and executing statements.
// It is compatible with both *sql.DB and *sql.Tx.
type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error)
}

// Option allows for flexible configuration of a statement or engine.
// Implementations apply settings to the provided Config.
type Option interface {
	Configure(config *Config)
}

// Configure returns a Config instance with all provided Options applied.
func Configure(opts ...Option) Config {
	var config = Config{
		Placeholder: Question,
	}

	for _, o := range opts {
		o.Configure(&config)
	}

	return config
}

// Config holds settings for statement generation, caching, logging, and template behavior.
type Config struct {
	Placeholder Placeholder
	Templates   []Template
	Log         Log
	Cache       *Cache
	Hasher      Hasher
}

// Configure applies the non-zero settings from this Config onto another Config.
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

// Cache configures the behavior of the expression result cache.
// A Size ≤ 0 disables size limiting. An Expiration ≤ 0 disables entry expiration.
type Cache struct {
	Size       int
	Expiration time.Duration
}

// Configure sets this Cache into the provided Config.
func (c *Cache) Configure(config *Config) {
	config.Cache = c
}

// NoCache returns nil, indicating that expression caching should be disabled.
func NoCache() *Cache {
	return nil
}

// NoExpirationCache creates a cache with a fixed size and no expiration.
func NoExpirationCache(size int) *Cache {
	return &Cache{
		Size:       size,
		Expiration: 0,
	}
}

// UnlimitedSizeCache creates a cache without size constraints but with expiration.
func UnlimitedSizeCache(expiration time.Duration) *Cache {
	return &Cache{
		Size:       0,
		Expiration: expiration,
	}
}

// Hasher defines a function that writes a unique, deterministic representation
// of the given parameter to the provided writer, typically for caching purposes.
type Hasher func(param any, writer io.Writer) error

// Configure sets this Hasher into the provided Config.
func (h Hasher) Configure(config *Config) {
	config.Hasher = h
}

// DefaultHasher returns a Hasher that serializes parameters as JSON.
// This provides stable and consistent cache keys.
func DefaultHasher() Hasher {
	return func(param any, writer io.Writer) error {
		return json.NewEncoder(writer).Encode(param)
	}
}

// Placeholder defines the syntax used to replace SQL parameters in templates.
// Can be a constant (e.g. '?') or positional (e.g. '$%d').
type Placeholder string

// Configure sets this Placeholder into the provided Config.
func (p Placeholder) Configure(config *Config) {
	config.Placeholder = p
}

const (
	// Question is the default SQL placeholder ('?') for anonymous parameters.
	Question Placeholder = "?"
	// Dollar uses PostgreSQL-style positional placeholders ($1, $2, ...).
	Dollar Placeholder = "$%d"
	// Colon uses Oracle-style positional placeholders (:1, :2, ...).
	Colon Placeholder = ":%d"
	// AtP uses T-SQL-style placeholders (@p1, @p2, ...).
	AtP Placeholder = "@p%d"
)

// Template is a functional option that transforms or extends a text/template.Template.
// Useful for parsing, naming, or registering custom functions.
type Template func(t *template.Template) (*template.Template, error)

// Configure appends this Template modifier to the Config’s template list.
func (to Template) Configure(config *Config) {
	config.Templates = append(config.Templates, to)
}

// Name creates a Template option that defines a new named template.
func Name(name string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.New(name), nil
	}
}

// Parse returns a Template option that parses and adds the provided template text.
func Parse(text string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Parse(text)
	}
}

// ParseFS returns a Template option that loads and parses templates from the given fs.FS using patterns.
func ParseFS(fs fs.FS, patterns ...string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFS(fs, patterns...)
	}
}

// ParseFiles returns a Template option that loads and parses templates from the specified files.
func ParseFiles(filenames ...string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFiles(filenames...)
	}
}

// ParseGlob returns a Template option that loads templates matching a glob pattern.
func ParseGlob(pattern string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseGlob(pattern)
	}
}

// Funcs returns a Template option that registers the provided functions to the template.
func Funcs(fm template.FuncMap) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Funcs(fm), nil
	}
}

// Lookup returns a Template option that retrieves a named template from the template set.
func Lookup(name string) Template {
	return func(tpl *template.Template) (*template.Template, error) {
		tpl = tpl.Lookup(name)
		if tpl == nil {
			return nil, fmt.Errorf("template %s not found", name)
		}

		return tpl, nil
	}
}

// Log defines a function used to log execution metadata for each SQL operation.
type Log func(ctx context.Context, info Info)

// Configure sets this Log function into the provided Config.
func (l Log) Configure(config *Config) {
	config.Log = l
}

// Info holds metadata about an executed SQL statement, including duration, mode, parameters, and errors.
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

// Mode describes the type of SQL operation being executed (e.g. query, exec).
type Mode string

const (
	// ExecMode indicates a statement that executes (e.g. CREATE, INSERT, UPDATE) without returning rows.
	ExecMode Mode = "Exec"
	// QueryRowMode indicates a query that returns a single row using QueryRowContext.
	QueryRowMode Mode = "QueryRow"
	// QueryMode indicates a query that returns multiple rows using QueryContext.
	QueryMode Mode = "Query"
	// FirstMode indicates a query expecting the first row, if available.
	FirstMode Mode = "First"
	// OneMode expects exactly one result row; returns an error if zero or multiple rows.
	OneMode Mode = "One"
	// AllMode retrieves all rows matching the query.
	AllMode Mode = "All"
)

// Expression represents a compiled SQL statement with its arguments and destination mappers.
// It is generated from a template and ready for execution.
type Expression[Dest any] struct {
	SQL      string
	Args     []any
	Scanners []Scanner[Dest]
}

// DestMapper returns a slice of values for scanning and a function to map the scanned data into a destination.
// It handles dynamic scanners that transform raw row data into the final result.
func (e Expression[Dest]) DestMapper(rows *sql.Rows) ([]any, func(dest *Dest) error, error) {
	if len(e.Scanners) == 0 {
		e.Scanners = []Scanner[Dest]{
			func() (any, func(dest *Dest) error) {
				var value Dest

				return &value, func(dest *Dest) error {
					*dest = value

					return nil
				}
			},
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

// First executes the SQL expression and returns the first result row, if any.
// Returns sql.ErrNoRows if no rows are found.
func (e Expression[Dest]) First(ctx context.Context, db DB) (Dest, error) {
	return e.fetchOne(ctx, db, false)
}

// ErrTooManyRows is returned when more than one row is found where only one was expected.
var ErrTooManyRows = errors.New("too many rows")

// One executes the expression and expects exactly one row.
// Returns sql.ErrNoRows if no rows are found.
// Returns ErrTooManyRows if more than one row is found.
func (e Expression[Dest]) One(ctx context.Context, db DB) (Dest, error) {
	return e.fetchOne(ctx, db, true)
}

// fetchOne is a shared internal helper to retrieve one result from the DB,
// with an optional enforcement of exactly one result.
func (e Expression[Dest]) fetchOne(ctx context.Context, db DB, enforeOne bool) (Dest, error) {
	var one Dest

	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return one, err
	}

	if !rows.Next() {
		return one, errors.Join(sql.ErrNoRows, rows.Close())
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		//nolint:sqlclosecheck
		return one, errors.Join(err, rows.Close())
	}

	if err = rows.Scan(values...); err != nil {
		return one, errors.Join(err, rows.Close())
	}

	if err = mapper(&one); err != nil {
		return one, errors.Join(err, rows.Close())
	}

	if enforeOne && rows.Next() {
		return one, errors.Join(ErrTooManyRows, rows.Close())
	}

	return one, errors.Join(rows.Close(), rows.Err())
}

// All executes the expression and returns all matching rows mapped into Dest values.
func (e Expression[Dest]) All(ctx context.Context, db DB) ([]Dest, error) {
	rows, err := db.QueryContext(ctx, e.SQL, e.Args...)
	if err != nil {
		return nil, err
	}

	values, mapper, err := e.DestMapper(rows)
	if err != nil {
		//nolint:sqlclosecheck
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

// Scanner defines a two-part function that produces a scan target and a function
// to assign the scanned value into a destination structure.
type Scanner[Dest any] func() (any, func(dest *Dest) error)

// Raw marks a string as raw SQL that should be inserted as-is into the template output,
// bypassing placeholder replacement.
type Raw string

// Exec creates a Statement that executes an SQL command (e.g. CREATE, INSERT, UPDATE) and returns the sql.Result.
func Exec[Param any](opts ...Option) Statement[Param, sql.Result] {
	return newStmt[Param](ExecMode, func(ctx context.Context, db DB, expr Expression[any]) (sql.Result, error) {
		return db.ExecContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

// QueryRow creates a Statement that returns a single *sql.Row from the query.
func QueryRow[Param any](opts ...Option) Statement[Param, *sql.Row] {
	return newStmt[Param](QueryRowMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Row, error) {
		return db.QueryRowContext(ctx, expr.SQL, expr.Args...), nil
	}, opts...)
}

// Query creates a Statement that returns *sql.Rows for result iteration.
func Query[Param any](opts ...Option) Statement[Param, *sql.Rows] {
	return newStmt[Param](QueryMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Rows, error) {
		return db.QueryContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

// First creates a Statement that retrieves the first matching row mapped to Dest.
func First[Param any, Dest any](opts ...Option) Statement[Param, Dest] {
	return newStmt[Param](FirstMode, func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.First(ctx, db)
	}, opts...)
}

// One creates a Statement that expects exactly one row mapped into Dest.
// Returns an error if zero or more than one row is returned.
func One[Param any, Dest any](opts ...Option) Statement[Param, Dest] {
	return newStmt[Param](OneMode, func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.One(ctx, db)
	}, opts...)
}

// All creates a Statement that retrieves all matching rows mapped into a slice of Dest.
func All[Param any, Dest any](opts ...Option) Statement[Param, []Dest] {
	return newStmt[Param](AllMode, func(ctx context.Context, db DB, expr Expression[Dest]) ([]Dest, error) {
		return expr.All(ctx, db)
	}, opts...)
}

// Statement represents a compiled, executable SQL statement.
type Statement[Param, Result any] interface {
	Exec(ctx context.Context, db DB, param Param) (Result, error)
}

// Stmt constructs a customizable Statement with explicit mode, executor, and options.
func Stmt[Param any, Dest any, Result any](mode Mode, exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), opts ...Option) Statement[Param, Result] {
	return newStmt[Param](mode, exec, opts...)
}

// newStmt creates a new statement with template parsing, validation, and caching.
// It returns a reusable, thread-safe Statement.
func newStmt[Param any, Dest any, Result any](mode Mode, exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), opts ...Option) Statement[Param, Result] {
	_, file, line, _ := runtime.Caller(2)

	location := file + ":" + strconv.Itoa(line)

	config := Configure(opts...)

	var (
		d = newDestinator[Dest]()

		t = template.New("").Funcs(template.FuncMap{
			"Raw":             func(sql string) Raw { return Raw(sql) },
			"Scan":            d.scan,
			"ScanJSON":        d.scanJSON,
			"ScanBinary":      d.scanBinary,
			"ScanText":        d.scanText,
			"ScanStringSlice": d.scanStringSlice,
			"ScanStringTime":  d.scanStringTime,
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

	if err = d.escapeNode(t, t.Root); err != nil {
		panic(fmt.Errorf("escape template at %s: %w", location, err))
	}

	t, err = t.Clone()
	if err != nil {
		panic(fmt.Errorf("clone template at %s: %w", location, err))
	}

	var (
		placeholder = string(config.Placeholder)
		positional  = strings.Contains(placeholder, "%d")
	)

	pool := &sync.Pool{
		New: func() any {
			tc, _ := t.Clone()

			r := &runner[Param, Dest]{
				tpl:       tc,
				sqlWriter: &sqlWriter{},
			}

			r.tpl.Funcs(template.FuncMap{
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

		if config.Hasher == nil {
			config.Hasher = DefaultHasher()
		}
	}

	return &statement[Param, Dest, Result]{
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

// statement is the internal implementation of Statement.
// It holds configuration, compiled templates, a cache (optional), and the execution function.
type statement[Param any, Dest any, Result any] struct {
	name     string
	location string
	mode     Mode
	hasher   Hasher
	cache    *expirable.LRU[uint64, Expression[Dest]]
	exec     func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error)
	pool     *sync.Pool
	log      Log
}

// Exec renders the template with the given param, applies caching (if enabled),
// and executes the resulting SQL expression using the provided DB.
func (s *statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (result Result, err error) {
	var (
		expr      Expression[Dest]
		hash      uint64
		withCache = s.cache != nil && s.hasher != nil
		cached    bool
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

	if withCache {
		hasher := hashPool.Get().(*xxhash.Digest)

		hasher.Reset()

		err = s.hasher(param, hasher)
		if err != nil {
			return result, fmt.Errorf("statement at %s: %w", s.location, err)
		}

		hash = hasher.Sum64()

		hashPool.Put(hasher)

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

	r.ctx = ctx

	expr, err = r.expr(param)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", s.location, err)
	}

	s.pool.Put(r)

	if withCache {
		_ = s.cache.Add(hash, expr)
	}

	result, err = s.exec(ctx, db, expr)
	if err != nil {
		return result, fmt.Errorf("statement at %s: %w", s.location, err)
	}

	return result, nil
}

// accessor provides reflection-based field access into a destination struct.
// It stores field index paths for nested access and handles pointer dereferencing.
type accessor[Dest any] struct {
	typ         reflect.Type
	pointerType reflect.Type
	pointer     bool
	indices     []int
}

// access returns a reflect.Value for the target field in the destination struct,
// creating intermediate pointers if necessary.
func (a accessor[Dest]) access(d *Dest) reflect.Value {
	v := reflect.ValueOf(d).Elem()

	for _, idx := range a.indices {
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

// Common reflect.Type instances for interface checks and built-in Go types
// used when mapping DB results to struct fields.
var (
	scannerType           = reflect.TypeFor[sql.Scanner]()
	timeType              = reflect.TypeFor[time.Time]()
	textUnmarshalerType   = reflect.TypeFor[encoding.TextUnmarshaler]()
	binaryUnmarshalerType = reflect.TypeFor[encoding.BinaryUnmarshaler]()
	byteSliceType         = reflect.TypeFor[[]byte]()
)

// newDestinator creates a new destination manager for a given result type,
// used to generate and cache field scanners.
func newDestinator[Dest any]() *destinator[Dest] {
	return &destinator[Dest]{
		store: map[string]Scanner[Dest]{},
		typ:   reflect.TypeFor[Dest](),
	}
}

// destinator generates and caches scanners for mapping query results into fields of Dest structs.
type destinator[Dest any] struct {
	mu    sync.RWMutex
	store map[string]Scanner[Dest]
	typ   reflect.Type
}

// makeAccessor analyzes the struct type and field path and builds an accessor.
func (d *destinator[Dest]) makeAccessor(t reflect.Type, field string) (accessor[Dest], error) {
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
					return accessor[Dest]{}, fmt.Errorf("field %s not found in struct %s", field, t.Name())
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

	l := len(indices)

	a := accessor[Dest]{
		typ:         t,
		pointerType: reflect.PointerTo(t),
		pointer:     l > 0 && indices[l-1] == -1,
		indices:     indices,
	}

	return a, nil
}

// cache checks for a cached scanner by key or builds and stores a new one using the provided function.
func (d *destinator[Dest]) cache(key string, field string, f func(a accessor[Dest]) (Scanner[Dest], error)) (Scanner[Dest], error) {
	d.mu.RLock()
	scanner, ok := d.store[key]
	d.mu.RUnlock()

	if ok {
		return scanner, nil
	}

	a, err := d.makeAccessor(d.typ, field)
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

// scan generates a scanner for common primitive types (string, int, time.Time, etc.)
// and assigns the scanned value to the target field.
func (d *destinator[Dest]) scan(field string) (Scanner[Dest], error) {
	return d.cache(field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.pointerType.Implements(scannerType) {
			return func() (any, func(dest *Dest) error) {
				var src any

				return &src, func(dest *Dest) error {
					return a.access(dest).Addr().Interface().(sql.Scanner).Scan(src)
				}
			}, nil
		}

		switch a.typ.Kind() {
		case reflect.String:
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *string

					return &src, func(dest *Dest) error {
						if src == nil {
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
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *int64

					return &src, func(dest *Dest) error {
						if src == nil {
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
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *uint64

					return &src, func(dest *Dest) error {
						if src == nil {
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
		case reflect.Float32, reflect.Float64:
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *float64

					return &src, func(dest *Dest) error {
						if src == nil {
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
		case reflect.Bool:
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *bool

					return &src, func(dest *Dest) error {
						if src == nil {
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

		if a.typ == timeType {
			if a.pointer {
				return func() (any, func(dest *Dest) error) {
					var src *time.Time

					return &src, func(dest *Dest) error {
						if src == nil {
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

		return nil, fmt.Errorf("scan: type %s is not supported", a.typ)
	})
}

// scanJSON generates a scanner that unmarshals JSON-encoded data into the destination field.
func (d *destinator[Dest]) scanJSON(field string) (Scanner[Dest], error) {
	return d.cache("JSON:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
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
	})
}

// scanBinary generates a scanner that uses encoding.BinaryUnmarshaler to decode the field value.
func (d *destinator[Dest]) scanBinary(field string) (Scanner[Dest], error) {
	return d.cache("Binary:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if !a.pointerType.Implements(binaryUnmarshalerType) {
			return nil, fmt.Errorf("scan binary: type %s doesn't implement encoding.BinaryUnmarshaler", a.typ)
		}

		return func() (any, func(dest *Dest) error) {
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				return a.access(dest).Addr().Interface().(encoding.BinaryUnmarshaler).UnmarshalBinary(src)
			}
		}, nil
	})
}

// scanText generates a scanner that uses encoding.TextUnmarshaler to parse text values from the DB.
func (d *destinator[Dest]) scanText(field string) (Scanner[Dest], error) {
	return d.cache("Text:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if !a.pointerType.Implements(textUnmarshalerType) {
			return nil, fmt.Errorf("scan text: type %s doesn't implement encoding.TextUnmarshaler", a.typ)
		}

		return func() (any, func(dest *Dest) error) {
			var src sql.Null[[]byte]

			return &src, func(dest *Dest) error {
				if !src.Valid {
					return nil
				}

				return a.access(dest).Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(src.V)
			}
		}, nil
	})
}

// scanStringSlice splits a string from the DB into a []string using the given separator,
// and assigns it to the destination field.
func (d *destinator[Dest]) scanStringSlice(field string, sep string) (Scanner[Dest], error) {
	return d.cache("StringSlice:"+field+":"+sep, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ.Kind() != reflect.Slice || a.typ.Elem().Kind() != reflect.String {
			return nil, fmt.Errorf("scan string slice: cannot set []string in type %s", a.typ)
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
	})
}

// layoutMap maps human-friendly layout aliases to standard Go time layouts.
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

// scanStringTime parses a time.Time value from a string field using a layout and time zone location.
// Supports both pointer and value destinations.
func (d *destinator[Dest]) scanStringTime(field string, layout string, location string) (Scanner[Dest], error) {
	return d.cache("StringTime:"+field+":"+layout+":"+location, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ != timeType {
			return nil, fmt.Errorf("scan string time: type %s is not time.Time", a.typ)
		}

		loc, err := time.LoadLocation(location)
		if err != nil {
			return nil, err
		}

		if l, ok := layoutMap[layout]; ok {
			layout = l
		}

		if layout == "" {
			return nil, errors.New("scan string time: no layout")
		}

		if a.pointer {
			return func() (any, func(dest *Dest) error) {
				var src *string

				return &src, func(dest *Dest) error {
					if src == nil {
						return nil
					}

					t, err := time.ParseInLocation(layout, *src, loc)
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
				t, err := time.ParseInLocation(layout, src, loc)
				if err != nil {
					return err
				}

				a.access(dest).Set(reflect.ValueOf(t))

				return nil
			}
		}, nil
	})
}

// escapeNode walks the parsed template tree and ensures each SQL-producing node ends with a call to the ident() function.
// This ensures correct placeholder binding in templates.
// Inspired by https://github.com/mhilton/sqltemplate/blob/main/escape.go.
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
			if len(cmd.Args) < 2 {
				continue
			}

			node, ok := cmd.Args[1].(*parse.StringNode)
			if !ok {
				continue
			}

			switch cmd.Args[0].String() {
			default:
				continue
			case "Scan":
				_, err := d.scan(node.Text)
				if err != nil {
					return err
				}
			case "ScanJSON":
				_, err := d.scanJSON(node.Text)
				if err != nil {
					return err
				}
			case "ScanBinary":
				_, err := d.scanJSON(node.Text)
				if err != nil {
					return err
				}
			case "ScanText":
				_, err := d.scanText(node.Text)
				if err != nil {
					return err
				}
			case "ScanStringSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanStringSlice(node.Text, sep.Text)
				if err != nil {
					return err
				}
			case "ScanStringTime":
				layout, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				location, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanStringTime(node.Text, layout.Text, location.Text)
				if err != nil {
					return err
				}
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

		return d.escapeNode(tpl, tpl.Root)
	}

	return nil
}

// hashPool is a sync.Pool of xxhash.Digest instances used to generate cache keys efficiently.
var hashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
}

// ident is the special template function name injected into expressions
// to bind template output to argument placeholders.
var ident = "__sqlt__"

// runner holds context, state, and buffers needed to execute a template with a specific Param.
// It collects SQL text, placeholders, and scanners.
type runner[Param any, Dest any] struct {
	ctx       context.Context
	tpl       *template.Template
	sqlWriter *sqlWriter
	args      []any
	scanners  []Scanner[Dest]
}

// expr renders the SQL template with the given Param,
// capturing the resulting SQL string, arguments, and scanners.
func (r *runner[Param, Dest]) expr(param Param) (Expression[Dest], error) {
	if err := r.tpl.Execute(r.sqlWriter, param); err != nil {
		return Expression[Dest]{}, err
	}

	expr := Expression[Dest]{
		SQL:      r.sqlWriter.String(),
		Args:     r.args,
		Scanners: r.scanners,
	}

	r.ctx = context.Background()
	r.sqlWriter.Reset()
	r.args = make([]any, 0, len(expr.Args))
	r.scanners = make([]Scanner[Dest], 0, len(expr.Scanners))

	return expr, nil
}

// sqlWriter writes template output into a normalized SQL string,
// collapsing whitespace and preserving consistent formatting.
type sqlWriter struct {
	data []byte
}

// Write implements io.Writer by normalizing and buffering SQL text,
// collapsing whitespace and preparing it for execution.
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

func (w *sqlWriter) Reset() {
	w.data = w.data[:0]
}

// String returns the accumulated SQL string and resets the writer buffer.
func (w *sqlWriter) String() string {
	n := len(w.data)

	if n == 0 {
		return ""
	} else if w.data[n-1] == ' ' {
		n--
	}

	return string(w.data[:n])
}
