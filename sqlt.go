package sqlt

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"runtime"
	"slices"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
	"unicode"

	"github.com/go-sqlt/structscan"
	"github.com/hashicorp/golang-lru/v2/expirable"
	"github.com/jba/templatecheck"
)

type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error)
}

type Logger interface {
	Log(ctx context.Context, info Info)
}

func Slog(logger *slog.Logger) Config {
	return Config{
		Logger: StructuredLogger{
			Logger: logger,
			Message: func(i Info) (string, slog.Level, []slog.Attr) {
				if i.Err != nil {
					return i.Err.Error(), slog.LevelError, []slog.Attr{
						slog.String("template", i.Template),
						slog.String("location", i.Location),
						slog.String("sql", i.SQL),
						slog.Any("args", i.Args),
						slog.Bool("cached", i.Cached),
					}
				}

				msg := i.Location

				if i.Template != "" {
					msg = fmt.Sprintf("%s at %s", i.Template, i.Location)
				}

				return msg, slog.LevelInfo, []slog.Attr{
					slog.Duration("duration", i.Duration),
					slog.String("sql", i.SQL),
					slog.Any("args", i.Args),
					slog.Bool("cached", i.Cached),
				}
			},
		},
	}
}

type StructuredLogger struct {
	Logger  *slog.Logger
	Message func(Info) (msg string, lvl slog.Level, attrs []slog.Attr)
}

func (l StructuredLogger) Log(ctx context.Context, info Info) {
	msg, lvl, attrs := l.Message(info)

	if msg == "" {
		return
	}

	l.Logger.LogAttrs(ctx, lvl, msg, attrs...)
}

func New(name string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.New(name), nil
			},
		},
	}
}

func Lookup(name string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				t := tpl.Lookup(name)
				if t == nil {
					return nil, fmt.Errorf("template not found: %s", name)
				}

				return t, nil
			},
		},
	}
}

func Parse(txt string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.Parse(txt)
			},
		},
	}
}

func ParseGlob(pattern string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.ParseGlob(pattern)
			},
		},
	}
}

func ParseFiles(filenames ...string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.ParseFiles(filenames...)
			},
		},
	}
}

func ParseFS(sys fs.FS, patterns ...string) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.ParseFS(sys, patterns...)
			},
		},
	}
}

func Funcs(fm template.FuncMap) Config {
	return Config{
		Parsers: []Parser{
			func(tpl *template.Template) (*template.Template, error) {
				return tpl.Funcs(fm), nil
			},
		},
	}
}

type Parser func(tpl *template.Template) (*template.Template, error)

type Config struct {
	Dialect     string
	Placeholder Placeholder
	Logger      Logger
	Cache       *Cache
	Parsers     []Parser
}

func (c Config) With(configs ...Config) Config {
	merged := Config{}

	for _, override := range append([]Config{c}, configs...) {
		if override.Dialect != "" {
			merged.Dialect = override.Dialect
		}

		if override.Placeholder != nil {
			merged.Placeholder = override.Placeholder
		}

		if override.Logger != nil {
			merged.Logger = override.Logger
		}

		if override.Cache != nil {
			merged.Cache = override.Cache
		}

		if len(override.Parsers) > 0 {
			merged.Parsers = append(merged.Parsers, override.Parsers...)
		}
	}

	return merged
}

func Sqlite() Config {
	return Dialect("Sqlite").With(Question())
}

func Postgres() Config {
	return Dialect("Postgres").With(Dollar())
}

func Dialect(name string) Config {
	return Config{
		Dialect: name,
	}
}

type Hasher interface {
	Hash(value any) (uint64, error)
}

type Cache struct {
	Expiration time.Duration
	Size       int
	Hasher     Hasher
}

func NoCache() Config {
	return Config{
		Cache: &Cache{},
	}
}

func LimitedCache(size int, hasher Hasher) Config {
	return Config{
		Cache: &Cache{
			Size:   size,
			Hasher: hasher,
		},
	}
}

func ExpiringCache(expiration time.Duration, hasher Hasher) Config {
	return Config{
		Cache: &Cache{
			Expiration: expiration,
			Hasher:     hasher,
		},
	}
}

type Placeholder interface {
	WritePlaceholder(pos int, writer io.Writer) error
}

type StaticPlaceholder string

func (p StaticPlaceholder) WritePlaceholder(_ int, writer io.Writer) error {
	_, err := writer.Write([]byte("?"))

	return err
}

type PositionalPlaceholder string

func (p PositionalPlaceholder) WritePlaceholder(pos int, writer io.Writer) error {
	_, err := writer.Write([]byte(string(p) + strconv.Itoa(pos)))

	return err
}

func Question() Config {
	return Config{
		Placeholder: StaticPlaceholder("?"),
	}
}

func Dollar() Config {
	return Config{
		Placeholder: PositionalPlaceholder("$"),
	}
}

func Colon() Config {
	return Config{
		Placeholder: PositionalPlaceholder(":"),
	}
}

func AtP() Config {
	return Config{
		Placeholder: PositionalPlaceholder("@p"),
	}
}

type Info struct {
	Duration time.Duration
	Template string
	Location string
	SQL      string
	Args     []any
	Err      error
	Cached   bool
}

type Expression[Dest any] struct {
	SQL    string
	Args   []any
	Mapper structscan.Mapper[Dest]
}

type Raw string

func Exec[Param any](configs ...Config) Statement[Param, sql.Result] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (sql.Result, error) {
		return db.ExecContext(ctx, expr.SQL, expr.Args...)
	}, configs...)
}

func QueryRow[Param any](configs ...Config) Statement[Param, *sql.Row] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (*sql.Row, error) {
		return db.QueryRowContext(ctx, expr.SQL, expr.Args...), nil
	}, configs...)
}

func Query[Param any](configs ...Config) Statement[Param, *sql.Rows] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[any]) (*sql.Rows, error) {
		return db.QueryContext(ctx, expr.SQL, expr.Args...)
	}, configs...)
}

func First[Param any, Dest any](configs ...Config) Statement[Param, Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		return expr.Mapper.Row(db.QueryRowContext(ctx, expr.SQL, expr.Args...))
	}, configs...)
}

func One[Param any, Dest any](configs ...Config) Statement[Param, Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) (Dest, error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return *new(Dest), err
		}

		return expr.Mapper.One(rows)
	}, configs...)
}

func All[Param any, Dest any](configs ...Config) Statement[Param, []Dest] {
	return newStmt[Param](func(ctx context.Context, db DB, expr Expression[Dest]) ([]Dest, error) {
		rows, err := db.QueryContext(ctx, expr.SQL, expr.Args...)
		if err != nil {
			return nil, err
		}

		return expr.Mapper.All(rows)
	}, configs...)
}

type Statement[Param, Result any] interface {
	Exec(ctx context.Context, db DB, param Param) (Result, error)
}

func Custom[Param any, Dest any, Result any](exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), configs ...Config) Statement[Param, Result] {
	return newStmt[Param](exec, configs...)
}

func newStmt[Param any, Dest any, Result any](exec func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error), configs ...Config) Statement[Param, Result] {
	_, file, line, _ := runtime.Caller(2)

	var (
		location = file + ":" + strconv.Itoa(line)
		config   = Sqlite().With(configs...)
		schema   = structscan.Describe[Dest]()

		t = template.New("").Option("missingkey=invalid").Funcs(template.FuncMap{
			"Dialect": func() string { return config.Dialect },
			"Raw":     func(sql string) Raw { return Raw(sql) },
			"Scan": func(path string, converters ...structscan.Converter) (structscan.Scanner[Dest], error) {
				field, err := schema.Field(path)
				if err != nil {
					return nil, err
				}

				switch len(converters) {
				case 0:
					return field, nil
				case 1:
					return field.Convert(converters[0])
				default:
					return field.Convert(structscan.Chain(converters...))
				}
			},

			"Nullable":            structscan.Nullable,
			"Default":             structscan.Default,
			"UnmarshalJSON":       structscan.UnmarshalJSON,
			"UnmarshalText":       structscan.UnmarshalText,
			"UnmarshalBinary":     structscan.UnmarshalBinary,
			"ParseTime":           structscan.ParseTime,
			"ParseTimeInLocation": structscan.ParseTimeInLocation,
			"Atoi":                structscan.Atoi,
			"ParseInt":            structscan.ParseInt,
			"ParseUint":           structscan.ParseUint,
			"ParseFloat":          structscan.ParseFloat,
			"ParseBool":           structscan.ParseBool,
			"ParseComplex":        structscan.ParseComplex,
			"Trim":                structscan.Trim,
			"TrimPrefix":          structscan.TrimPrefix,
			"TrimSuffix":          structscan.TrimSuffix,
			"Contains":            structscan.Contains,
			"ContainsAny":         structscan.ContainsAny,
			"HasPrefix":           structscan.HasPrefix,
			"HasSuffix":           structscan.HasSuffix,
			"EqualFold":           structscan.EqualFold,
			"Index":               structscan.Index,
			"ToLower":             structscan.ToLower,
			"ToUpper":             structscan.ToUpper,
			"Chain":               structscan.Chain,
			"OneOf":               structscan.OneOf,
			"Enum":                structscan.Enum,
			"Cut":                 structscan.Cut,
			"Split":               structscan.Split,
			"DateTime":            staticFunc(time.DateTime),
			"DateOnly":            staticFunc(time.DateOnly),
			"TimeOnly":            staticFunc(time.TimeOnly),
			"RFC3339":             staticFunc(time.RFC3339),
			"RFC3339Nano":         staticFunc(time.RFC3339Nano),
			"Layout":              staticFunc(time.Layout),
			"ANSIC":               staticFunc(time.ANSIC),
			"UnixDate":            staticFunc(time.UnixDate),
			"RubyDate":            staticFunc(time.RubyDate),
			"RFC822":              staticFunc(time.RFC822),
			"RFC822Z":             staticFunc(time.RFC822Z),
			"RFC850":              staticFunc(time.RFC850),
			"RFC1123":             staticFunc(time.RFC1123),
			"RFC1123Z":            staticFunc(time.RFC1123Z),
			"Kitchen":             staticFunc(time.Kitchen),
			"Stamp":               staticFunc(time.Stamp),
			"StampMilli":          staticFunc(time.StampMilli),
			"StampMicro":          staticFunc(time.StampMicro),
			"StampNano":           staticFunc(time.StampNano),
			"UTC":                 staticFunc(time.UTC),
			"Local":               staticFunc(time.Local), //nolint:gosmopolitan
			"LoadLocation":        time.LoadLocation,
		})
		err error
	)

	for _, p := range config.Parsers {
		t, err = p(t)
		if err != nil {
			panic(fmt.Errorf("statement at %s: parse template: %w", location, err))
		}
	}

	var zero Param
	if err = templatecheck.CheckText(t, zero); err != nil {
		panic(fmt.Errorf("statement at %s: check template: %w", location, err))
	}

	if err = escapeNode(t, t.Root); err != nil {
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

						return "", nil
					case structscan.Converter:
						if len(r.scanners) != 0 {
							return "", errors.New("use Scan function to access different fields of a struct")
						}

						field, err := schema.Field("")
						if err != nil {
							return "", err
						}

						scanner, err := field.Convert(a)
						if err != nil {
							return "", err
						}

						r.scanners = append(r.scanners, scanner)

						return "", nil
					case structscan.Scanner[Dest]:
						r.scanners = append(r.scanners, a)

						return "", nil
					default:
						r.args = append(r.args, arg)

						return "", config.Placeholder.WritePlaceholder(len(r.args), r.sqlWriter)
					}
				},
			})

			return r
		},
	}

	var cache *expirable.LRU[uint64, Expression[Dest]]

	if config.Cache != nil && config.Cache.Hasher != nil {
		cache = expirable.NewLRU[uint64, Expression[Dest]](config.Cache.Size, nil, config.Cache.Expiration)

		_, err = config.Cache.Hasher.Hash(zero)
		if err != nil {
			panic(fmt.Errorf("statement at %s: hashing param: %w", location, err))
		}
	}

	return &statement[Param, Dest, Result]{
		name:     t.Name(),
		location: location,
		cache:    cache,
		pool:     pool,
		logger:   config.Logger,
		exec:     exec,
		hasher:   config.Cache.Hasher,
	}
}

type statement[Param any, Dest any, Result any] struct {
	name     string
	location string
	cache    *expirable.LRU[uint64, Expression[Dest]]
	exec     func(ctx context.Context, db DB, expr Expression[Dest]) (Result, error)
	pool     *sync.Pool
	logger   Logger
	hasher   Hasher
}

func (s *statement[Param, Dest, Result]) Exec(ctx context.Context, db DB, param Param) (result Result, err error) {
	var (
		expr   Expression[Dest]
		hash   uint64
		cached bool
	)

	if s.logger != nil {
		now := time.Now()

		defer func() {
			s.logger.Log(ctx, Info{
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
		hash, err = s.hasher.Hash(param)
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

// Inspired by https://github.com/mhilton/sqltemplate/blob/main/escape.go.
func escapeNode(t *template.Template, n parse.Node) error {
	switch v := n.(type) {
	case *parse.ActionNode:
		return escapeNode(t, v.Pipe)
	case *parse.IfNode:
		return twoErrors(
			escapeNode(t, v.List),
			escapeNode(t, v.ElseList),
		)
	case *parse.ListNode:
		if v == nil {
			return nil
		}

		for _, n := range v.Nodes {
			if err := escapeNode(t, n); err != nil {
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
			escapeNode(t, v.List),
			escapeNode(t, v.ElseList),
		)
	case *parse.WithNode:
		return twoErrors(
			escapeNode(t, v.List),
			escapeNode(t, v.ElseList),
		)
	case *parse.TemplateNode:
		tpl := t.Lookup(v.Name)
		if tpl == nil {
			return fmt.Errorf("template %s not found", v.Name)
		}

		return escapeNode(tpl, tpl.Root)
	}

	return nil
}

const ident = "__sqlt__"

type runner[Param any, Dest any] struct {
	tpl       *template.Template
	sqlWriter *sqlWriter
	args      []any
	scanners  []structscan.Scanner[Dest]
}

func (r *runner[Param, Dest]) expr(param Param) (Expression[Dest], error) {
	if err := r.tpl.Execute(r.sqlWriter, param); err != nil {
		return Expression[Dest]{}, err
	}

	expr := Expression[Dest]{
		SQL:    r.sqlWriter.String(),
		Args:   slices.Clone(r.args),
		Mapper: structscan.Map(r.scanners...),
	}

	r.sqlWriter.Reset()
	r.args = r.args[:0]
	r.scanners = r.scanners[:0]

	return expr, nil
}

type sqlWriter struct {
	data []byte
}

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

func staticFunc[T any](t T) func() T {
	return func() T {
		return t
	}
}
