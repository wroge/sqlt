package sqlt

import (
	"context"
	"database/sql"
	"encoding"
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
	"unicode"

	"github.com/cespare/xxhash/v2"
	"github.com/goccy/go-json"
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

// Config holds settings for statement generation, caching, logging, and template behavior.
type Config struct {
	Dialect              string
	Placeholder          func(pos int, writer io.Writer) error
	Templates            []func(t *template.Template) (*template.Template, error)
	Log                  func(ctx context.Context, info Info)
	ExpressionSize       int
	ExpressionExpiration time.Duration
}

// With returns a new Config with all provided configs layered on top.
// Later configs override earlier ones.
func (c Config) With(configs ...Config) Config {
	var merged Config

	for _, override := range append([]Config{c}, configs...) {
		if override.Dialect != "" {
			merged.Dialect = override.Dialect
		}

		if override.Placeholder != nil {
			merged.Placeholder = override.Placeholder
		}

		if len(override.Templates) > 0 {
			merged.Templates = append(merged.Templates, override.Templates...)
		}

		if override.Log != nil {
			merged.Log = override.Log
		}

		if override.ExpressionExpiration != 0 {
			merged.ExpressionExpiration = override.ExpressionExpiration
		}

		if override.ExpressionSize != 0 {
			merged.ExpressionSize = override.ExpressionSize
		}
	}

	return merged
}

// Sqlite is the default configuration with a Question placeholder.
func Sqlite() Config {
	return Dialect("Sqlite").With(Question())
}

// Postgres sets the Dialect function to "Postgres" and uses a Dollar placeholder.
func Postgres() Config {
	return Dialect("Postgres").With(Dollar())
}

// Dialect sets the value of the Dialect function.
func Dialect(name string) Config {
	return Config{
		Dialect: name,
	}
}

func Cache(size int, exp time.Duration) Config {
	return Config{
		ExpressionSize:       size,
		ExpressionExpiration: exp,
	}
}

// NoCache disables expression caching by setting size and expiration to zero.
func NoCache() Config {
	return Cache(0, 0)
}

// NoExpirationCache creates a cache with a fixed size and no expiration.
func NoExpirationCache(size int) Config {
	return Cache(size, -1)
}

// UnlimitedSizeCache creates a cache without size constraints but with expiration.
func UnlimitedSizeCache(expiration time.Duration) Config {
	return Cache(-1, expiration)
}

// Placeholder configures how argument placeholders are rendered in SQL statements.
// For example, it can output ?, $1, :1, or @p1 depending on the SQL dialect.
func Placeholder(f func(pos int, writer io.Writer) error) Config {
	return Config{
		Placeholder: f,
	}
}

// Question is a placeholder format used by sqlite.
func Question() Config {
	return Placeholder(func(pos int, writer io.Writer) error {
		_, err := writer.Write([]byte("?"))

		return err
	})
}

// Dollar is a placeholder format used by postgres.
func Dollar() Config {
	return Placeholder(func(pos int, writer io.Writer) error {
		_, err := writer.Write([]byte("$" + strconv.Itoa(pos)))

		return err
	})
}

// Colon is a placeholder format used by oracle.
func Colon() Config {
	return Placeholder(func(pos int, writer io.Writer) error {
		_, err := writer.Write([]byte(":" + strconv.Itoa(pos)))

		return err
	})
}

// AtP is a placeholder format used by sql server.
func AtP() Config {
	return Placeholder(func(pos int, writer io.Writer) error {
		_, err := writer.Write([]byte("@p" + strconv.Itoa(pos)))

		return err
	})
}

// Template defines a function that modifies a Go text/template before execution.
func Template(f func(t *template.Template) (*template.Template, error)) Config {
	return Config{
		Templates: []func(t *template.Template) (*template.Template, error){
			f,
		},
	}
}

// Name creates a Template config that defines a new named template.
func Name(name string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.New(name), nil
	})
}

// Parse returns a Template config that parses and adds the provided template text.
func Parse(text string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.Parse(text)
	})
}

// ParseFS returns a Template config that loads and parses templates from the given fs.FS using patterns.
func ParseFS(fs fs.FS, patterns ...string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.ParseFS(fs, patterns...)
	})
}

// ParseFiles returns a Template config that loads and parses templates from the specified files.
func ParseFiles(filenames ...string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.ParseFiles(filenames...)
	})
}

// ParseGlob returns a Template config that loads templates matching a glob pattern.
func ParseGlob(pattern string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.ParseGlob(pattern)
	})
}

// Funcs returns a Template config that registers the provided functions to the template.
func Funcs(fm template.FuncMap) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		return t.Funcs(fm), nil
	})
}

// Lookup returns a Template config that retrieves a named template from the template set.
func Lookup(name string) Config {
	return Template(func(t *template.Template) (*template.Template, error) {
		t = t.Lookup(name)
		if t == nil {
			return nil, fmt.Errorf("template %s not found", name)
		}

		return t, nil
	})
}

// Log defines a function used to log execution metadata for each SQL operation.
func Log(l func(ctx context.Context, info Info)) Config {
	return Config{
		Log: l,
	}
}

// Info holds metadata about an executed SQL statement, including duration, parameters, and errors.
type Info struct {
	Duration time.Duration
	Template string
	Location string
	SQL      string
	Args     []any
	Err      error
	Cached   bool
}

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

// First executes the SQL expression and returns at most one result row.
// If no row is found, returns sql.ErrNoRows.
func (e Expression[Dest]) First(ctx context.Context, db DB) (Dest, error) {
	return e.fetchOne(ctx, db, false)
}

// ErrTooManyRows is returned when more than one row is found where only one was expected.
var ErrTooManyRows = errors.New("too many rows")

// One executes the expression and expects exactly one row.
// If no row is found, returns sql.ErrNoRows.
// If more than one row is found, returns ErrTooManyRows.
func (e Expression[Dest]) One(ctx context.Context, db DB) (Dest, error) {
	return e.fetchOne(ctx, db, true)
}

// fetchOne is a shared internal helper to retrieve one result from the DB,
// with an optional enforcement of exactly one result.
func (e Expression[Dest]) fetchOne(ctx context.Context, db DB, enforceOne bool) (Dest, error) {
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

	if enforceOne && rows.Next() {
		return one, errors.Join(ErrTooManyRows, rows.Close())
	}

	return one, errors.Join(rows.Err(), rows.Close())
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

	return all, errors.Join(rows.Err(), rows.Close())
}

// Scanner defines a two-part function that produces a scan target and a function
// to assign the scanned value into a destination structure.
type Scanner[Dest any] func() (any, func(dest *Dest) error)

// Raw marks SQL input to be used verbatim without modification by the template engine.
type Raw string

// Exec renders the statement using the provided parameter, applies optional caching, and runs it on the given DB handle.
func Exec[Param any](configs ...Config) Statement[Param, sql.Result] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (sql.Result, error) {
		return db.ExecContext(ctx, expr.SQL, expr.Args...)
	}, configs...)
}

// QueryRow creates a Statement that returns a single *sql.Row from the query.
func QueryRow[Param any](configs ...Config) Statement[Param, *sql.Row] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (*sql.Row, error) {
		return db.QueryRowContext(ctx, expr.SQL, expr.Args...), nil
	}, configs...)
}

// Query creates a Statement that returns *sql.Rows for result iteration.
func Query[Param any](configs ...Config) Statement[Param, *sql.Rows] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (*sql.Rows, error) {
		return db.QueryContext(ctx, expr.SQL, expr.Args...)
	}, configs...)
}

// First creates a Statement that retrieves the first matching row mapped to Dest.
func First[Param any, Dest any](configs ...Config) Statement[Param, Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.First(ctx, db)
	}, configs...)
}

// One returns exactly one row mapped into Dest. If no row is found, it returns sql.ErrNoRows.
// If more than one row is found, it returns ErrTooManyRows.
func One[Param any, Dest any](configs ...Config) Statement[Param, Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.One(ctx, db)
	}, configs...)
}

// All creates a Statement that retrieves all matching rows mapped into a slice of Dest.
func All[Param any, Dest any](configs ...Config) Statement[Param, []Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) ([]Dest, error) {
		return expr.All(ctx, db)
	}, configs...)
}

// Statement represents a compiled, executable SQL statement.
type Statement[Param, Result any] interface {
	Exec(ctx context.Context, db DB, param Param) (Result, error)
}

// Custom constructs a customizable Statement with executor and config options.
func Custom[Param any, Dest any, Result any](exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), configs ...Config) Statement[Param, Result] {
	return newStmt[Param](exec, configs...)
}

// newStmt creates a new statement with template parsing, validation, and caching.
// It returns a reusable, thread-safe Statement.
func newStmt[Param any, Dest any, Result any](exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), configs ...Config) Statement[Param, Result] {
	_, file, line, _ := runtime.Caller(2)

	location := file + ":" + strconv.Itoa(line)

	config := Sqlite().With(configs...)

	var (
		d = newDestinator[Dest]()

		t = template.New("").Funcs(template.FuncMap{
			"Dialect":         func() string { return config.Dialect },
			"Raw":             func(sql string) Raw { return Raw(sql) },
			"Scan":            d.scan,
			"ScanBytes":       d.scanBytes,
			"ScanTime":        d.scanTime,
			"ScanString":      d.scanString,
			"ScanInt":         d.scanInt,
			"ScanUint":        d.scanUint,
			"ScanFloat":       d.scanFloat,
			"ScanBool":        d.scanBool,
			"ScanJSON":        d.scanJSON,
			"ScanBinary":      d.scanBinary,
			"ScanText":        d.scanText,
			"ScanStringSlice": d.scanStringSlice,
			"ScanIntSlice":    d.scanIntSlice,
			"ScanUintSlice":   d.scanUintSlice,
			"ScanFloatSlice":  d.scanFloatSlice,
			"ScanBoolSlice":   d.scanBoolSlice,
			"ScanStringTime":  d.scanStringTime,
		})
		err error
	)

	for _, tpl := range config.Templates {
		t, err = tpl(t)
		if err != nil {
			panic(fmt.Errorf("statement at %s: parse template: %w", location, err))
		}
	}

	if err = templatecheck.CheckText(t, *new(Param)); err != nil {
		panic(fmt.Errorf("statement at %s: check template: %w", location, err))
	}

	if err = d.escapeNode(t, t.Root); err != nil {
		panic(fmt.Errorf("statement at %s: escape template: %w", location, err))
	}

	t, err = t.Clone()
	if err != nil {
		panic(fmt.Errorf("statement at %s: clone template: %w", location, err))
	}

	pool := &sync.Pool{
		New: func() any {
			tc, _ := t.Clone()

			r := &runner[Param, Dest]{
				tpl:       tc,
				sqlWriter: &sqlWriter{},
			}

			r.tpl.Funcs(template.FuncMap{
				ident: func(arg any) (Raw, error) {
					switch a := arg.(type) {
					case Raw:
						r.sqlWriter.data = append(r.sqlWriter.data, []byte(a)...)

						return Raw(""), nil
					case Scanner[Dest]:
						r.scanners = append(r.scanners, a)

						return Raw(""), nil
					default:
						r.args = append(r.args, arg)

						return Raw(""), config.Placeholder(len(r.args), r.sqlWriter)
					}
				},
			})

			return r
		},
	}

	var cache *expirable.LRU[uint64, Expression[Dest]]

	if config.ExpressionSize != 0 || config.ExpressionExpiration != 0 {
		cache = expirable.NewLRU[uint64, Expression[Dest]](config.ExpressionSize, nil, config.ExpressionExpiration)
	}

	return &statement[Param, Dest, Result]{
		name:     t.Name(),
		location: location,
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
	cache    *expirable.LRU[uint64, Expression[Dest]]
	exec     func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error)
	pool     *sync.Pool
	log      func(ctx context.Context, info Info)
}

// Exec renders the template with the given param, applies caching (if enabled),
// and executes the resulting SQL expression using the provided DB.
func (s *statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (result Result, err error) {
	var (
		expr   Expression[Dest]
		hash   uint64
		cached bool
	)

	if s.log != nil {
		now := time.Now()

		defer func() {
			s.log(ctx, Info{
				Template: s.name,
				Location: s.location,
				Duration: time.Since(now),
				SQL:      expr.SQL,
				Args:     expr.Args,
				Err:      err,
				Cached:   cached,
			})
		}()
	}

	if s.cache != nil {
		b, err := json.Marshal(param)
		if err != nil {
			return result, fmt.Errorf("statement at %s: marshal json: %w", s.location, err)
		}

		hash = xxhash.Sum64(b)

		expr, cached = s.cache.Get(hash)
		if cached {
			result, err = s.exec(ctx, db, expr)
			if err != nil {
				return result, fmt.Errorf("statement at %s: cached execution: %w", s.location, err)
			}

			return result, nil
		}
	}

	r := s.pool.Get().(*runner[Param, Dest])

	expr, err = r.expr(param)
	if err != nil {
		return result, fmt.Errorf("statement at %s: expression: %w", s.location, err)
	}

	s.pool.Put(r)

	if s.cache != nil {
		_ = s.cache.Add(hash, expr)
	}

	result, err = s.exec(ctx, db, expr)
	if err != nil {
		return result, fmt.Errorf("statement at %s: execution: %w", s.location, err)
	}

	return result, nil
}

// accessor provides field-level reflective access to nested struct fields.
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
	jsonMessageType       = reflect.TypeFor[json.RawMessage]()
)

// newDestinator creates a new destination manager for a given result type,
// used to generate and cache field scanners.
func newDestinator[Dest any]() *destinator[Dest] {
	return &destinator[Dest]{
		store: map[string]Scanner[Dest]{},
		typ:   reflect.TypeFor[Dest](),
	}
}

// destinator manages scan function registration and caching for result mappings.
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

// scan generates a scanner for values implementing sql.Scanner.
func (d *destinator[Dest]) scan(field string) (Scanner[Dest], error) {
	return d.cache(field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if !a.pointerType.Implements(scannerType) {
			return nil, fmt.Errorf("scan: type %s doesn't implement sql.Scanner", a.typ)
		}

		return func() (any, func(dest *Dest) error) {
			var src any

			return &src, func(dest *Dest) error {
				return a.access(dest).Addr().Interface().(sql.Scanner).Scan(src)
			}
		}, nil
	})
}

func (d *destinator[Dest]) scanBytes(field string) (Scanner[Dest], error) {
	return d.cache("Bytes:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ != byteSliceType {
			return nil, fmt.Errorf("scan bytes: type %s is not of type []byte", a.typ)
		}

		return func() (any, func(dest *Dest) error) {
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				a.access(dest).Set(reflect.ValueOf(src))

				return nil
			}
		}, nil
	})
}

// scanTime generates a scanner for time.Time values.
func (d *destinator[Dest]) scanTime(field string) (Scanner[Dest], error) {
	return d.cache("Time:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ != timeType {
			return nil, fmt.Errorf("scan time: type %s is not of type time.Time", a.typ)
		}

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
	})
}

// scanAny is a helper function.
func scanAny[T any, Dest any](pointer bool, set func(dest *Dest, src T)) Scanner[Dest] {
	if pointer {
		return func() (any, func(dest *Dest) error) {
			var src *T

			return &src, func(dest *Dest) error {
				if src == nil {
					return nil
				}

				set(dest, *src)

				return nil
			}
		}
	}

	return func() (any, func(dest *Dest) error) {
		var src T

		return &src, func(dest *Dest) error {
			set(dest, src)

			return nil
		}
	}
}

// scanString generates a scanner for string values.
func (d *destinator[Dest]) scanString(field string) (Scanner[Dest], error) {
	return d.cache("String:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ.Kind() != reflect.String {
			return nil, fmt.Errorf("scan string: type %s is not of kind string", a.typ)
		}

		return scanAny(a.pointer, func(dest *Dest, src string) {
			a.access(dest).SetString(src)
		}), nil
	})
}

// scanBool generates a scanner for bool values.
func (d *destinator[Dest]) scanBool(field string) (Scanner[Dest], error) {
	return d.cache("Bool:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ.Kind() != reflect.Bool {
			return nil, fmt.Errorf("scan bool: type %s is not of kind bool", a.typ)
		}

		return scanAny(a.pointer, func(dest *Dest, src bool) {
			a.access(dest).SetBool(src)
		}), nil
	})
}

// scanInt generates a scanner for int values.
func (d *destinator[Dest]) scanInt(field string) (Scanner[Dest], error) {
	return d.cache("Int:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		switch a.typ.Kind() {
		default:
			return nil, fmt.Errorf("scan int: type %s is not of kind int", a.typ)
		case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
			return scanAny(a.pointer, func(dest *Dest, src int64) {
				a.access(dest).SetInt(src)
			}), nil
		}
	})
}

// scanUint generates a scanner for uint values.
func (d *destinator[Dest]) scanUint(field string) (Scanner[Dest], error) {
	return d.cache("Uint:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		switch a.typ.Kind() {
		default:
			return nil, fmt.Errorf("scan uint: type %s is not of kind uint", a.typ)
		case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
			return scanAny(a.pointer, func(dest *Dest, src uint64) {
				a.access(dest).SetUint(src)
			}), nil
		}
	})
}

// scanFloat generates a scanner for float values.
func (d *destinator[Dest]) scanFloat(field string) (Scanner[Dest], error) {
	return d.cache("Float:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		switch a.typ.Kind() {
		default:
			return nil, fmt.Errorf("scan float: type %s is not of kind float", a.typ)
		case reflect.Float64, reflect.Float32:
			return scanAny(a.pointer, func(dest *Dest, src float64) {
				a.access(dest).SetFloat(src)
			}), nil
		}
	})
}

// scanJSON generates a scanner that unmarshals JSON-encoded data into the destination field.
func (d *destinator[Dest]) scanJSON(field string) (Scanner[Dest], error) {
	return d.cache("JSON:"+field, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ == jsonMessageType {
			return func() (any, func(dest *Dest) error) {
				var src []byte

				return &src, func(dest *Dest) error {
					if len(src) == 0 {
						return nil
					}

					a.access(dest).Set(reflect.ValueOf(src))

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
			var src []byte

			return &src, func(dest *Dest) error {
				if len(src) == 0 {
					return nil
				}

				return a.access(dest).Addr().Interface().(encoding.TextUnmarshaler).UnmarshalText(src)
			}
		}, nil
	})
}

func (d *destinator[Dest]) scanStringSliceSetter(t string, kinds []reflect.Kind, field string, sep string, set func(value reflect.Value, str string) error) (Scanner[Dest], error) {
	lt := strings.ToLower(t)

	return d.cache(t+"Slice:"+field+":"+sep, field, func(a accessor[Dest]) (Scanner[Dest], error) {
		if a.typ.Kind() != reflect.Slice {
			return nil, fmt.Errorf("scan %s slice: cannot set []%s in type %s", lt, lt, a.typ)
		}

		var indirections int

		elem := a.typ.Elem()
		for elem.Kind() == reflect.Pointer {
			elem = elem.Elem()

			indirections++

			continue
		}

		if !slices.Contains(kinds, elem.Kind()) {
			return nil, fmt.Errorf("scan %s slice: cannot set []%s in type %s", lt, lt, a.typ)
		}

		return func() (any, func(dest *Dest) error) {
			var src *string

			return &src, func(dest *Dest) error {
				if src == nil || *src == "" {
					return nil
				}

				split := slices.DeleteFunc(strings.Split(*src, sep), func(d string) bool {
					return d == ""
				})

				value := a.access(dest)

				value.Set(reflect.MakeSlice(a.typ, len(split), len(split)))

				for i, v := range split {
					index := value.Index(i)

					for range indirections {
						if index.IsNil() {
							index.Set(reflect.New(index.Type().Elem()))
						}

						index = index.Elem()
					}

					if err := set(index, v); err != nil {
						return err
					}
				}

				return nil
			}
		}, nil
	})
}

// scanStringSlice splits a string from the DB into a []string using the given separator,
// and assigns it to the destination field.
func (d *destinator[Dest]) scanStringSlice(field string, sep string) (Scanner[Dest], error) {
	return d.scanStringSliceSetter("String",
		[]reflect.Kind{reflect.String},
		field, sep, func(value reflect.Value, str string) error {
			value.SetString(str)

			return nil
		})
}

// scanIntSlice splits a string from the DB and converts each part to int.
func (d *destinator[Dest]) scanIntSlice(field string, sep string) (Scanner[Dest], error) {
	return d.scanStringSliceSetter("Int",
		[]reflect.Kind{reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64},
		field, sep, func(value reflect.Value, str string) error {
			v, err := strconv.ParseInt(str, 10, 64)
			if err != nil {
				return err
			}

			value.SetInt(v)

			return nil
		})
}

// scanUintSlice splits a string from the DB and converts each part to uint.
func (d *destinator[Dest]) scanUintSlice(field string, sep string) (Scanner[Dest], error) {
	return d.scanStringSliceSetter("Uint",
		[]reflect.Kind{reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64},
		field, sep, func(value reflect.Value, str string) error {
			v, err := strconv.ParseUint(str, 10, 64)
			if err != nil {
				return err
			}

			value.SetUint(v)

			return nil
		})
}

// scanFloatSlice splits a string from the DB and converts each part to float.
func (d *destinator[Dest]) scanFloatSlice(field string, sep string) (Scanner[Dest], error) {
	return d.scanStringSliceSetter("Float", []reflect.Kind{reflect.Float32, reflect.Float64}, field, sep, func(value reflect.Value, str string) error {
		v, err := strconv.ParseFloat(str, 64)
		if err != nil {
			return err
		}

		value.SetFloat(v)

		return nil
	})
}

// scanBoolSlice splits a string from the DB and converts each part to bool.
func (d *destinator[Dest]) scanBoolSlice(field string, sep string) (Scanner[Dest], error) {
	return d.scanStringSliceSetter("Bool", []reflect.Kind{reflect.Bool}, field, sep, func(value reflect.Value, str string) error {
		v, err := strconv.ParseBool(str)
		if err != nil {
			return err
		}

		value.SetBool(v)

		return nil
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
			case "ScanBytes":
				_, err := d.scanBytes(node.Text)
				if err != nil {
					return err
				}
			case "ScanTime":
				_, err := d.scanTime(node.Text)
				if err != nil {
					return err
				}
			case "ScanString":
				_, err := d.scanString(node.Text)
				if err != nil {
					return err
				}
			case "ScanInt":
				_, err := d.scanInt(node.Text)
				if err != nil {
					return err
				}
			case "ScanUint":
				_, err := d.scanUint(node.Text)
				if err != nil {
					return err
				}
			case "ScanFloat":
				_, err := d.scanFloat(node.Text)
				if err != nil {
					return err
				}
			case "ScanBool":
				_, err := d.scanBool(node.Text)
				if err != nil {
					return err
				}
			case "ScanJSON":
				_, err := d.scanJSON(node.Text)
				if err != nil {
					return err
				}
			case "ScanBinary":
				_, err := d.scanBinary(node.Text)
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
			case "ScanIntSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanIntSlice(node.Text, sep.Text)
				if err != nil {
					return err
				}
			case "ScanUintSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanUintSlice(node.Text, sep.Text)
				if err != nil {
					return err
				}
			case "ScanFloatSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanFloatSlice(node.Text, sep.Text)
				if err != nil {
					return err
				}
			case "ScanBoolSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := d.scanBoolSlice(node.Text, sep.Text)
				if err != nil {
					return err
				}
			case "ScanStringTime":
				layout, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				location, ok := cmd.Args[3].(*parse.StringNode)
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
		if tpl == nil {
			return fmt.Errorf("template %s not found", v.Name)
		}

		return d.escapeNode(tpl, tpl.Root)
	}

	return nil
}

// ident is the internal name for the binding function injected into SQL templates.
// It ensures that expression values are correctly converted to placeholders (e.g., ?, $1, @p1).
const ident = "__sqlt__"

// runner stores the intermediate state during template rendering and SQL generation.
type runner[Param any, Dest any] struct {
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
		if unicode.IsSpace(rune(b)) {
			if len(w.data) > 0 && w.data[len(w.data)-1] != ' ' {
				w.data = append(w.data, ' ')
			}
		} else {
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
