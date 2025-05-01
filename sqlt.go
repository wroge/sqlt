// Package sqlt is a go template based sql builder and struct mapper.
package sqlt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"runtime"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
	"unicode"

	"github.com/cespare/xxhash/v2"
	"github.com/go-sqlt/datahash"
	"github.com/go-sqlt/structscan"
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
	Hasher               func(any) (uint64, error)
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

		if override.Hasher != nil {
			merged.Hasher = override.Hasher
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

// Hasher injects a custom hash function for cache keys.
func Hasher(fn func(param any) (uint64, error)) Config {
	return Config{
		Hasher: fn,
	}
}

// Cache is enables if size or expiration is not 0.
// Negative size or expiration means unlimited or non-expirable cache.
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
	return Placeholder(func(_ int, writer io.Writer) error {
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
	Scanners []structscan.Scanner[Dest]
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
		return structscan.First(db.QueryRowContext(ctx, expr.SQL, expr.Args...), expr.Scanners...)
	}, configs...)
}

// One returns exactly one row mapped into Dest. If no row is found, it returns sql.ErrNoRows.
// If more than one row is found, it returns ErrTooManyRows.
func One[Param any, Dest any](configs ...Config) Statement[Param, Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return *new(Dest), err
		}

		return structscan.One(rows, expr.Scanners...)
	}, configs...)
}

// All creates a Statement that retrieves all matching rows mapped into a slice of Dest.
func All[Param any, Dest any](configs ...Config) Statement[Param, []Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) ([]Dest, error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return nil, err
		}

		return structscan.All(rows, expr.Scanners...)
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
		s = structscan.New[Dest]()

		t = template.New("").Funcs(template.FuncMap{
			"Dialect":         func() string { return config.Dialect },
			"Raw":             func(sql string) Raw { return Raw(sql) },
			"Scan":            s.Scan,
			"ScanBytes":       s.ScanBytes,
			"ScanTime":        s.ScanTime,
			"ScanString":      s.ScanString,
			"ScanInt":         s.ScanInt,
			"ScanUint":        s.ScanUint,
			"ScanFloat":       s.ScanFloat,
			"ScanBool":        s.ScanBool,
			"ScanJSON":        s.ScanJSON,
			"ScanBinary":      s.ScanBinary,
			"ScanText":        s.ScanText,
			"ScanStringSlice": s.ScanStringSlice,
			"ScanStringTime":  s.ScanStringTime,
		})
		err error
	)

	for _, tpl := range config.Templates {
		t, err = tpl(t)
		if err != nil {
			panic(fmt.Errorf("statement at %s: parse template: %w", location, err))
		}
	}

	var zero Param
	if err = templatecheck.CheckText(t, zero); err != nil {
		panic(fmt.Errorf("statement at %s: check template: %w", location, err))
	}

	if err = escapeNode(s, t, t.Root); err != nil {
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
					case structscan.Scanner[Dest]:
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

		if config.Hasher == nil {
			hasher := datahash.New(xxhash.New, datahash.Options{})

			config.Hasher = func(a any) (uint64, error) {
				return hasher.Hash(a)
			}
		}

		_, err = config.Hasher(zero)
		if err != nil {
			panic(fmt.Errorf("statement at %s: hashing param: %w", location, err))
		}
	}

	return &statement[Param, Dest, Result]{
		name:     t.Name(),
		location: location,
		cache:    cache,
		pool:     pool,
		log:      config.Log,
		exec:     exec,
		hasher:   config.Hasher,
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
	hasher   func(any) (uint64, error)
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
		hash, err = s.hasher(param)
		if err != nil {
			return result, fmt.Errorf("statement at %s: hashing param: %w", s.location, err)
		}

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

// escapeNode walks the parsed template tree and ensures each SQL-producing node ends with a call to the ident() function.
// This ensures correct placeholder binding in templates.
// Inspired by https://github.com/mhilton/sqltemplate/blob/main/escape.go.
func escapeNode[T any](s *structscan.Struct[T], t *template.Template, n parse.Node) error {
	switch v := n.(type) {
	case *parse.ActionNode:
		return escapeNode(s, t, v.Pipe)
	case *parse.IfNode:
		return twoErrors(
			escapeNode(s, t, v.List),
			escapeNode(s, t, v.ElseList),
		)
	case *parse.ListNode:
		if v == nil {
			return nil
		}

		for _, n := range v.Nodes {
			if err := escapeNode(s, t, n); err != nil {
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
				_, err := s.Scan(node.Text)
				if err != nil {
					return err
				}
			case "ScanBytes":
				_, err := s.ScanBytes(node.Text)
				if err != nil {
					return err
				}
			case "ScanTime":
				_, err := s.ScanTime(node.Text)
				if err != nil {
					return err
				}
			case "ScanString":
				_, err := s.ScanString(node.Text)
				if err != nil {
					return err
				}
			case "ScanInt":
				_, err := s.ScanInt(node.Text)
				if err != nil {
					return err
				}
			case "ScanUint":
				_, err := s.ScanUint(node.Text)
				if err != nil {
					return err
				}
			case "ScanFloat":
				_, err := s.ScanFloat(node.Text)
				if err != nil {
					return err
				}
			case "ScanBool":
				_, err := s.ScanBool(node.Text)
				if err != nil {
					return err
				}
			case "ScanJSON":
				_, err := s.ScanJSON(node.Text)
				if err != nil {
					return err
				}
			case "ScanBinary":
				_, err := s.ScanBinary(node.Text)
				if err != nil {
					return err
				}
			case "ScanText":
				_, err := s.ScanText(node.Text)
				if err != nil {
					return err
				}
			case "ScanStringSlice":
				sep, ok := cmd.Args[2].(*parse.StringNode)
				if !ok {
					continue
				}

				_, err := s.ScanStringSlice(node.Text, sep.Text)
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

				_, err := s.ScanStringTime(node.Text, layout.Text, location.Text)
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
		return twoErrors(
			escapeNode(s, t, v.List),
			escapeNode(s, t, v.ElseList),
		)
	case *parse.WithNode:
		return twoErrors(
			escapeNode(s, t, v.List),
			escapeNode(s, t, v.ElseList),
		)
	case *parse.TemplateNode:
		tpl := t.Lookup(v.Name)
		if tpl == nil {
			return fmt.Errorf("template %s not found", v.Name)
		}

		return escapeNode(s, tpl, tpl.Root)
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
	scanners  []structscan.Scanner[Dest]
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
	r.scanners = make([]structscan.Scanner[Dest], 0, len(expr.Scanners))

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

func twoErrors(err1, err2 error) error {
	if err1 == nil {
		return err2
	}

	if err2 == nil {
		return err1
	}

	return errors.Join(err1, err2)
}
