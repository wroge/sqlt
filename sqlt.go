package sqlt

import (
	"context"
	"database/sql"
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

// DB is the interface of *sql.DB and *sql.Tx.
type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error)
}

// Raw lets template functions insert SQL directly.
// Use with caution to prevent SQL injection.
type Raw string

// Scanner maps SQL columns to Dest.
// If reusable, Dest can be reused for each sql.Rows.Next().
type Scanner[Dest any] struct {
	Mapper   Mapper[Dest]
	Reusable bool
	SQL      string
}

// Mapper maps a destination pointer with an optional transformation.
type Mapper[Dest any] func(dest *Dest) (any, func() error)

// Option is implemented by Config, Placeholders, Templates, etc.
type Option interface {
	Configure(config *Config)
}

// Config holds all configuration options.
type Config struct {
	Placeholder Placeholder
	Templates   []Template
	Log         Log
	Cache       *Cache
	Hasher      Hasher
}

// Configure applies Config settings.
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
	Duration time.Duration
	Mode     Mode
	Template string
	Location string
	SQL      string
	Args     []any
	Err      error
	Reusable bool
	Cached   bool
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

// Expression represents a SQL statement with arguments and mappers.
type Expression[Dest any] struct {
	SQL      string
	Args     []any
	Mappers  []Mapper[Dest]
	Reusable bool
}

func destMappers[Dest any](dest *Dest, mappings []Mapper[Dest]) ([]any, []func() error) {
	if len(mappings) == 0 {
		return []any{dest}, nil
	}

	var (
		values  = make([]any, len(mappings))
		mappers = make([]func() error, len(mappings))
	)

	for i, m := range mappings {
		values[i], mappers[i] = m(dest)
	}

	return values, mappers
}

// Exec runs an expression using ExecContext.
func Exec[Param any](opts ...Option) *Statement[Param, any, sql.Result] {
	return Stmt[Param](getLocation(), ExecMode, func(ctx context.Context, db DB, expr Expression[any]) (sql.Result, error) {
		return db.ExecContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

// Query runs an expression using QueryContext.
func Query[Param any](opts ...Option) *Statement[Param, any, *sql.Rows] {
	return Stmt[Param](getLocation(), QueryMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Rows, error) {
		return db.QueryContext(ctx, expr.SQL, expr.Args...)
	}, opts...)
}

// QueryRow runs an expression using QueryRowContext.
func QueryRow[Param any](opts ...Option) *Statement[Param, any, *sql.Row] {
	return Stmt[Param](getLocation(), QueryRowMode, func(ctx context.Context, db DB, expr Expression[any]) (*sql.Row, error) {
		return db.QueryRowContext(ctx, expr.SQL, expr.Args...), nil
	}, opts...)
}

// First runs QueryContext and maps the first result.
func First[Param any, Dest any](opts ...Option) *Statement[Param, Dest, Dest] {
	return Stmt[Param](getLocation(), FirstMode, func(ctx context.Context, db DB, expr Expression[Dest]) (first Dest, err error) {
		row := db.QueryRowContext(ctx, expr.SQL, expr.Args...)

		values, mappers := destMappers(&first, expr.Mappers)

		if err = row.Scan(values...); err != nil {
			return first, err
		}

		for _, m := range mappers {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return first, err
			}
		}

		return first, nil
	}, opts...)
}

// ErrTooManyRows is returned from One statements if more than one row exists.
// This error can be wrapped and should be identified using errors.Is.
var ErrTooManyRows = errors.New("sqlt: too many rows")

// One runs QueryContext and maps one result.
// Returns ErrTooManyRows if multiple rows exist.
func One[Param any, Dest any](opts ...Option) *Statement[Param, Dest, Dest] {
	return Stmt[Param](getLocation(), OneMode, func(ctx context.Context, db DB, expr Expression[Dest]) (one Dest, err error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return one, err
		}

		defer func() {
			err = errors.Join(err, rows.Close(), rows.Err())
		}()

		if !rows.Next() {
			return one, sql.ErrNoRows
		}

		values, mappers := destMappers(&one, expr.Mappers)

		if err = rows.Scan(values...); err != nil {
			return one, err
		}

		for _, m := range mappers {
			if m != nil {
				if err = m(); err != nil {
					return one, err
				}
			}
		}

		if rows.Next() {
			return one, ErrTooManyRows
		}

		return one, nil
	}, opts...)
}

func getLocation() string {
	_, file, line, _ := runtime.Caller(2)

	return fmt.Sprintf("%s:%d", file, line)
}

// All runs QueryContext and maps the results.
func All[Param any, Dest any](opts ...Option) *Statement[Param, Dest, []Dest] {
	return Stmt[Param](getLocation(), AllMode, func(ctx context.Context, db DB, expr Expression[Dest]) (all []Dest, err error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return nil, err
		}

		defer func() {
			err = errors.Join(err, rows.Close(), rows.Err())
		}()

		var (
			dest    *Dest
			values  []any
			mappers []func() error
		)

		if expr.Reusable {
			dest = new(Dest)

			values, mappers = destMappers(dest, expr.Mappers)
		}

		for rows.Next() {
			if !expr.Reusable {
				dest = new(Dest)

				values, mappers = destMappers(dest, expr.Mappers)
			}

			if err = rows.Scan(values...); err != nil {
				return nil, err
			}

			for _, m := range mappers {
				if m != nil {
					if err = m(); err != nil {
						return nil, err
					}
				}
			}

			all = append(all, *dest)
		}

		return all, nil
	}, opts...)
}

// Stmt can be used to define a statement using your own exec function.
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

	sb := &statementBuilder[Param, Dest, Result]{
		mode:      mode,
		location:  location,
		config:    config,
		destType:  reflect.TypeFor[Dest](),
		accessors: map[string]accessor{},
	}

	return sb.toStmt(exec)
}

// A Statement can execute the predefined template and optionally scan rows into a Result.
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

var hashPool = sync.Pool{
	New: func() any {
		return xxhash.New()
	},
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

		defer func() {
			s.log(ctx, Info{
				Template: s.name,
				Location: s.location,
				Duration: time.Since(now),
				Mode:     s.mode,
				SQL:      expr.SQL,
				Args:     expr.Args,
				Err:      err,
				Reusable: expr.Reusable,
				Cached:   cached,
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
			return result, err
		}

		hash = hasher.Sum64()

		expr, cached = s.cache.Get(hash)
		if cached {
			return s.exec(ctx, db, expr)
		}
	}

	r := s.pool.Get().(*runner[Param, Dest])
	defer func() {
		r.Reset()
		s.pool.Put(r)
	}()

	expr, err = r.Expr(param)
	if err != nil {
		return result, err
	}

	if s.cache != nil {
		_ = s.cache.Add(hash, expr)
	}

	return s.exec(ctx, db, expr)
}

type runner[Param any, Dest any] struct {
	tpl       *template.Template
	sqlWriter *sqlWriter
	args      []any
	reusable  bool
	mappers   []Mapper[Dest]
}

func (r *runner[Param, Dest]) Reset() {
	r.sqlWriter.Reset()
	r.args = r.args[:0]
	r.mappers = r.mappers[:0]
}

func (r *runner[Param, Dest]) Expr(param Param) (Expression[Dest], error) {
	if err := r.tpl.Execute(r.sqlWriter, param); err != nil {
		return Expression[Dest]{}, err
	}

	return Expression[Dest]{
		SQL:      r.sqlWriter.String(),
		Args:     slices.Clone(r.args),
		Reusable: r.reusable,
		Mappers:  slices.Clone(r.mappers),
	}, nil
}

type sqlWriter struct {
	data []byte
}

// Reset the writer.
func (s *sqlWriter) Reset() {
	s.data = s.data[:0]
}

// Write implements io.Writer.
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

// String returns the sql string from the writer.
func (s *sqlWriter) String() string {
	if len(s.data) == 0 {
		return ""
	}

	if s.data[len(s.data)-1] == ' ' {
		s.data = s.data[:len(s.data)-1]
	}

	return string(s.data)
}

var ident = "__sqlt__"

func checkType(t reflect.Type, expect string) bool {
	if expect == "" {
		return true
	}

	fullPath := strings.ContainsRune(expect, '/')
	index := strings.IndexByte(expect, '.')
	if index >= 0 {
		if fullPath {
			return expect[:index] == t.PkgPath()
		}

		return expect == t.String()
	}

	return expect == t.Name()
}

type accessor struct {
	indices []int
	typ     reflect.Type
}

func (a accessor) get(v reflect.Value) reflect.Value {
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

type statementBuilder[Param any, Dest any, Result any] struct {
	mode           Mode
	location       string
	config         *Config
	destType       reflect.Type
	accessors      map[string]accessor
	accessorsMutex sync.RWMutex
}

func (sb *statementBuilder[Param, Dest, Result]) getAccessor(field string) (accessor, error) {
	sb.accessorsMutex.RLock()
	if acc, ok := sb.accessors[field]; ok {
		sb.accessorsMutex.RUnlock()
		return acc, nil
	}
	sb.accessorsMutex.RUnlock()

	var t = sb.destType

	if field == "" {
		return accessor{typ: t}, nil
	}

	indices := []int{}

	for t.Kind() == reflect.Pointer {
		t = t.Elem()

		indices = append(indices, -1)

		continue
	}

	if t.Kind() != reflect.Struct {
		return accessor{}, fmt.Errorf("dest type '%s' is not of kind 'struct'", t.Name())
	}

	for _, part := range strings.Split(field, ".") {
		sf, found := t.FieldByName(part)
		if !found {
			return accessor{}, fmt.Errorf("field %s not found in struct %s", field, t.Name())
		}

		if !sf.IsExported() {
			return accessor{}, fmt.Errorf("field %s in struct %s is not exported", field, t.Name())
		}

		indices = append(indices, sf.Index[0])
		t = sf.Type

		for t.Kind() == reflect.Pointer {
			t = t.Elem()

			indices = append(indices, -1)

			continue
		}
	}

	acc := accessor{typ: t, indices: indices}

	sb.accessorsMutex.Lock()
	sb.accessors[field] = acc
	sb.accessorsMutex.Unlock()

	return acc, nil
}

func (sb *statementBuilder[Param, Dest, Result]) toStmt(exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error)) *Statement[Param, Dest, Result] {
	var (
		t = template.New("").Funcs(template.FuncMap{
			"Raw": func(sql string) Raw { return Raw(sql) },
			"Scan": func(field string, sql string) (Scanner[Dest], error) {
				return sb.scan("", field, sql, false)
			},
			"ScanType": func(expectedType string, field string, sql string) (Scanner[Dest], error) {
				return sb.scan(expectedType, field, sql, false)
			},
			"ScanJSON": func(field string, sql string) (Scanner[Dest], error) {
				return sb.scan("", field, sql, true)
			},
			"ScanTypeJSON": func(expectedType string, field string, sql string) (Scanner[Dest], error) {
				return sb.scan(expectedType, field, sql, true)
			},
		})
		err error
	)

	for _, to := range sb.config.Templates {
		t, err = to(t)
		if err != nil {
			panic(err)
		}
	}

	if err = templatecheck.CheckText(t, *new(Param)); err != nil {
		panic(err)
	}

	if err = sb.escape(t); err != nil {
		panic(err)
	}

	t, err = t.Clone()
	if err != nil {
		panic(err)
	}

	var (
		placeholder = string(sb.config.Placeholder)
		positional  = strings.Contains(placeholder, "%d")
	)

	pool := &sync.Pool{
		New: func() any {
			tc, _ := t.Clone()

			r := &runner[Param, Dest]{
				tpl:       tc,
				sqlWriter: &sqlWriter{},
				reusable:  true,
			}

			r.tpl.Funcs(template.FuncMap{
				ident: func(arg any) Raw {
					switch a := arg.(type) {
					case Raw:
						return a
					case Scanner[Dest]:
						if !a.Reusable {
							r.reusable = false
						}

						r.mappers = append(r.mappers, a.Mapper)

						return Raw(a.SQL)
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

	if sb.config.Cache != nil {
		cache = expirable.NewLRU[uint64, Expression[Dest]](sb.config.Cache.Size, nil, sb.config.Cache.Expiration)
	}

	return &Statement[Param, Dest, Result]{
		name:     t.Name(),
		location: sb.location,
		mode:     sb.mode,
		hasher:   sb.config.Hasher,
		cache:    cache,
		pool:     pool,
		log:      sb.config.Log,
		exec:     exec,
	}
}

func (sb *statementBuilder[Param, Dest, Result]) scan(expectType string, field string, sql string, jsonMode bool) (Scanner[Dest], error) {
	accessor, err := sb.getAccessor(field)
	if err != nil {
		return Scanner[Dest]{}, err
	}

	if !checkType(accessor.typ, expectType) {
		return Scanner[Dest]{}, fmt.Errorf("field %s expects type '%s' but got '%s'", field, expectType, accessor.typ.Name())
	}

	if jsonMode {
		if accessor.typ.Kind() == reflect.Slice && accessor.typ.Elem().Kind() == reflect.Uint8 {
			return Scanner[Dest]{
				SQL:      sql,
				Reusable: !slices.Contains(accessor.indices, -1),
				Mapper: func(dest *Dest) (any, func() error) {
					v := reflect.ValueOf(dest).Elem()

					fieldValue := accessor.get(v)

					var data []byte

					return &data, func() error {
						fieldValue.SetBytes(data)

						return nil
					}
				},
			}, nil
		}

		return Scanner[Dest]{
			SQL:      sql,
			Reusable: !slices.Contains(accessor.indices, -1),
			Mapper: func(dest *Dest) (any, func() error) {
				v := reflect.ValueOf(dest).Elem()

				fieldValue := accessor.get(v)

				var data []byte

				return &data, func() error {
					return json.Unmarshal(data, fieldValue.Addr().Interface())
				}
			},
		}, nil
	}

	return Scanner[Dest]{
		SQL:      sql,
		Reusable: !slices.Contains(accessor.indices, -1),
		Mapper: func(dest *Dest) (any, func() error) {
			v := reflect.ValueOf(dest).Elem()

			fieldValue := accessor.get(v)

			return fieldValue.Addr().Interface(), nil
		},
	}, nil
}

// copied from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func (sb *statementBuilder[Param, Dest, Result]) escape(text *template.Template) error {
	return sb.escapeNode(text, text.Tree, text.Tree.Root)
}

func (sb *statementBuilder[Param, Dest, Result]) escapeNode(t *template.Template, s *parse.Tree, n parse.Node) error {
	switch v := n.(type) {
	case *parse.ActionNode:
		return sb.escapeNode(t, s, v.Pipe)
	case *parse.IfNode:
		return errors.Join(
			sb.escapeNode(t, s, v.List),
			sb.escapeNode(t, s, v.ElseList),
		)
	case *parse.ListNode:
		if v == nil {
			return nil
		}

		for _, n := range v.Nodes {
			if err := sb.escapeNode(t, s, n); err != nil {
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

		if v.Cmds[0].Args[0].String() == "Scan" || v.Cmds[0].Args[0].String() == "ScanJSON" {
			if len(v.Cmds[0].Args) != 3 {
				return fmt.Errorf("function '%s' has an invalid number of args", v.Cmds[0].Args[0])
			}

			node, ok := v.Cmds[0].Args[1].(*parse.StringNode)
			if !ok {
				return fmt.Errorf("do not set field of function '%s' dynamically", v.Cmds[0].Args[0])
			}

			_, err := sb.getAccessor(node.Text)
			if err != nil {
				return err
			}
		}

		if v.Cmds[0].Args[0].String() == "ScanType" || v.Cmds[0].Args[0].String() == "ScanTypeJSON" {
			if len(v.Cmds[0].Args) != 4 {
				return fmt.Errorf("function '%s' has an invalid number of args", v.Cmds[0].Args[0])
			}

			node, ok := v.Cmds[0].Args[2].(*parse.StringNode)
			if !ok {
				return fmt.Errorf("do not set field of function '%s' dynamically", v.Cmds[0].Args[0])
			}

			acc, err := sb.getAccessor(node.Text)
			if err != nil {
				return err
			}

			typeNode, ok := v.Cmds[0].Args[1].(*parse.StringNode)
			if !ok {
				return fmt.Errorf("do not set type of function '%s' dynamically", v.Cmds[0].Args[0])
			}

			if !checkType(acc.typ, typeNode.Text) {
				return fmt.Errorf("function '%s %s' expects type '%s' but got '%s'", v.Cmds[0].Args[0], node.Text, typeNode.Text, acc.typ.Name())
			}
		}

		cmd := v.Cmds[len(v.Cmds)-1]
		if len(cmd.Args) == 1 && cmd.Args[0].Type() == parse.NodeIdentifier && cmd.Args[0].(*parse.IdentifierNode).Ident == ident {
			return nil
		}

		v.Cmds = append(v.Cmds, &parse.CommandNode{
			NodeType: parse.NodeCommand,
			Args:     []parse.Node{parse.NewIdentifier(ident).SetTree(s).SetPos(cmd.Pos)},
		})
	case *parse.RangeNode:
		return errors.Join(
			sb.escapeNode(t, s, v.List),
			sb.escapeNode(t, s, v.ElseList),
		)
	case *parse.WithNode:
		return errors.Join(
			sb.escapeNode(t, s, v.List),
			sb.escapeNode(t, s, v.ElseList),
		)
	case *parse.TemplateNode:
		tpl := t.Lookup(v.Name)

		return sb.escapeNode(tpl, tpl.Tree, tpl.Tree.Root)
	}

	return nil
}
