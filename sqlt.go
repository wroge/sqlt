package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
	"unicode"

	"github.com/jba/templatecheck"
)

type DB interface {
	QueryContext(ctx context.Context, str string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, str string, args ...any) *sql.Row
	ExecContext(ctx context.Context, str string, args ...any) (sql.Result, error)
}

func InTx(ctx context.Context, opts *sql.TxOptions, db *sql.DB, do func(db DB) error) (err error) {
	var tx *sql.Tx

	tx, err = db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	err = do(tx)

	return err
}

type Config struct {
	Context     func(ctx context.Context, runner Runner) context.Context
	Log         func(ctx context.Context, err error, runner Runner)
	Placeholder string
	Positional  bool
	Options     []Option
}

type Option func(tpl *template.Template) (*template.Template, error)

func New(name string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.New(name), nil
	}
}

func Parse(text string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Parse(text)
	}
}

func ParseFS(fs fs.FS, patterns ...string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFS(fs, patterns...)
	}
}

func ParseFiles(filenames ...string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFiles(filenames...)
	}
}

func ParseGlob(pattern string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseGlob(pattern)
	}
}

func Funcs(fm template.FuncMap) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Funcs(fm), nil
	}
}

func MissingKeyInvalid() Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=invalid"), nil
	}
}

func MissingKeyZero() Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=zero"), nil
	}
}

func MissingKeyError() Option {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=error"), nil
	}
}

func Lookup(name string) Option {
	return func(tpl *template.Template) (*template.Template, error) {
		tpl = tpl.Lookup(name)
		if tpl == nil {
			return nil, fmt.Errorf("template '%s' not found", name)
		}

		return tpl, nil
	}
}

type Raw string

type Scanner struct {
	Value any
	Map   func() error
	SQL   string
}

func Scan[T any](dest *T, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, errors.New("invalid nil pointer")
	}

	return Scanner{
		SQL:   str,
		Value: dest,
	}, nil
}

var null = []byte("null")

func ScanJSON[T any](dest *T, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, errors.New("invalid nil pointer")
	}

	var data []byte

	return Scanner{
		SQL:   str,
		Value: &data,
		Map: func() error {
			if len(data) == 0 || bytes.Equal(data, null) {
				*dest = *new(T)

				return nil
			}

			return json.Unmarshal(data, dest)
		},
	}, nil
}

func defaultTemplate() *template.Template {
	return template.New("").Funcs(template.FuncMap{
		// ident is a stub function
		ident: func(arg any) Raw {
			return ""
		},
		"Dest": func() any {
			return nil
		},
		"Raw": func(str string) Raw {
			return Raw(str)
		},
		"Scan": func(value sql.Scanner, str string) (Scanner, error) {
			if value == nil {
				return Scanner{}, errors.New("invalid nil pointer")
			}

			return Scanner{
				SQL:   str,
				Value: value,
			}, nil
		},
		"ScanJSON": func(value json.Unmarshaler, str string) (Scanner, error) {
			if value == nil {
				return Scanner{}, errors.New("invalid nil pointer")
			}

			var data []byte

			return Scanner{
				SQL:   str,
				Value: &data,
				Map: func() error {
					if err := value.UnmarshalJSON(data); err != nil {
						return err
					}

					return nil
				},
			}, nil
		},
		"ScanString":    Scan[string],
		"ScanBytes":     Scan[[]byte],
		"ScanInt":       Scan[int],
		"ScanInt8":      Scan[int8],
		"ScanInt16":     Scan[int16],
		"ScanInt32":     Scan[int32],
		"ScanInt64":     Scan[int64],
		"ScanUint":      Scan[uint],
		"ScanUint8":     Scan[uint8],
		"ScanUint16":    Scan[uint16],
		"ScanUint32":    Scan[uint32],
		"ScanUint64":    Scan[uint64],
		"ScanBool":      Scan[bool],
		"ScanFloat32":   Scan[float32],
		"ScanFloat64":   Scan[float64],
		"ScanTime":      Scan[time.Time],
		"ScanDuration":  Scan[time.Duration],
		"ScanStringP":   Scan[*string],
		"ScanBytesP":    Scan[*[]byte],
		"ScanIntP":      Scan[*int],
		"ScanInt8P":     Scan[*int8],
		"ScanInt16P":    Scan[*int16],
		"ScanInt32P":    Scan[*int32],
		"ScanInt64P":    Scan[*int64],
		"ScanUintP":     Scan[*uint],
		"ScanUint8P":    Scan[*uint8],
		"ScanUint16P":   Scan[*uint16],
		"ScanUint32P":   Scan[*uint32],
		"ScanUint64P":   Scan[*uint64],
		"ScanBoolP":     Scan[*bool],
		"ScanFloat32P":  Scan[*float32],
		"ScanFloat64P":  Scan[*float64],
		"ScanTimeP":     Scan[*time.Time],
		"ScanDurationP": Scan[*time.Duration],
	})
}

func Stmt[Param any](config *Config, opts ...Option) *Statement[Param] {
	tpl := defaultTemplate()

	var err error

	for _, opt := range append(config.Options, opts...) {
		tpl, err = opt(tpl)
		if err != nil {
			panic(err)
		}
	}

	if err = templatecheck.CheckText(tpl, *new(Param)); err != nil {
		panic(err)
	}

	escape(tpl)

	return &Statement[Param]{
		Context: config.Context,
		Log:     config.Log,
		Pool: &sync.Pool{
			New: func() any {
				t, err := tpl.Clone()
				if err != nil {
					return err
				}

				runner := &execRunner{
					tpl:    t,
					writer: &writer{},
				}

				t.Funcs(template.FuncMap{
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						default:
							runner.args = append(runner.args, arg)

							if config.Positional {
								return Raw(config.Placeholder + strconv.Itoa(len(runner.args)))
							}

							return Raw(config.Placeholder)
						}
					},
				})

				return runner
			},
		},
	}
}

type Statement[Param any] struct {
	Context func(ctx context.Context, runner Runner) context.Context
	Log     func(ctx context.Context, err error, runner Runner)
	Pool    *sync.Pool
}

type Runner interface {
	Template() *template.Template
	SQL() fmt.Stringer
	Args() []any
}

type execRunner struct {
	tpl    *template.Template
	writer *writer
	args   []any
}

func (r *execRunner) Template() *template.Template {
	return r.tpl
}

func (r *execRunner) SQL() fmt.Stringer {
	return r.writer
}

func (r *execRunner) Args() []any {
	return r.args
}

func runExec[Param, Result any](s *Statement[Param], ctx context.Context, param Param, exec func(ctx context.Context, runner *execRunner) (Result, error)) (Result, error) {
	var (
		result Result
		err    error
	)

	item := s.Pool.Get()
	if err, ok := item.(error); ok {
		if s.Log != nil {
			s.Log(ctx, err, nil)
		}

		return result, err
	}

	runner := item.(*execRunner)

	if s.Context != nil {
		ctx = s.Context(ctx, runner)
	}

	defer func() {
		if s.Log != nil {
			s.Log(ctx, err, runner)
		}

		runner.writer.Reset()
		runner.args = runner.args[:0]

		s.Pool.Put(runner)
	}()

	if err = runner.tpl.Execute(runner.writer, param); err != nil {
		return result, err
	}

	result, err = exec(ctx, runner)
	if err != nil {
		return result, err
	}

	return result, err
}

func (s *Statement[Param]) Exec(ctx context.Context, db DB, param Param) (sql.Result, error) {
	return runExec(s, ctx, param, func(ctx context.Context, runner *execRunner) (sql.Result, error) {
		return db.ExecContext(ctx, runner.writer.String(), runner.args...)
	})
}

func (s *Statement[Param]) QueryRow(ctx context.Context, db DB, param Param) (*sql.Row, error) {
	return runExec(s, ctx, param, func(ctx context.Context, runner *execRunner) (*sql.Row, error) {
		return db.QueryRowContext(ctx, runner.writer.String(), runner.args...), nil
	})
}

func (s *Statement[Param]) Query(ctx context.Context, db DB, param Param) (*sql.Rows, error) {
	return runExec(s, ctx, param, func(ctx context.Context, runner *execRunner) (*sql.Rows, error) {
		return db.QueryContext(ctx, runner.writer.String(), runner.args...)
	})
}

func QueryStmt[Param, Dest any](config *Config, opts ...Option) *QueryStatement[Param, Dest] {
	tpl := defaultTemplate()

	destType := reflect.TypeFor[Dest]().Name()
	if goodName(destType) {
		tpl = tpl.Funcs(template.FuncMap{
			destType: func() *Dest {
				return new(Dest)
			},
		})
	}

	var err error

	for _, opt := range append(config.Options, opts...) {
		tpl, err = opt(tpl)
		if err != nil {
			panic(err)
		}
	}

	if err = templatecheck.CheckText(tpl, *new(Param)); err != nil {
		panic(err)
	}

	escape(tpl)

	return &QueryStatement[Param, Dest]{
		ctx: config.Context,
		log: config.Log,
		pool: &sync.Pool{
			New: func() any {
				t, err := tpl.Clone()
				if err != nil {
					return err
				}

				runner := &queryRunner[Dest]{
					tpl:    t,
					writer: &writer{},
				}

				if goodName(destType) {
					t.Funcs(template.FuncMap{
						destType: func() *Dest {
							return runner.dest
						},
					})
				}

				t.Funcs(template.FuncMap{
					"Dest": func() *Dest {
						return runner.dest
					},
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						case Scanner:
							runner.scanners = append(runner.scanners, a.Value)
							runner.mappers = append(runner.mappers, a.Map)

							return Raw(a.SQL)
						default:
							runner.args = append(runner.args, arg)

							if config.Positional {
								return Raw(config.Placeholder + strconv.Itoa(len(runner.args)))
							}

							return Raw(config.Placeholder)
						}
					},
				})

				return runner
			},
		},
	}
}

type queryRunner[Dest any] struct {
	tpl      *template.Template
	writer   *writer
	args     []any
	dest     *Dest
	scanners []any
	mappers  []func() error
}

func (r *queryRunner[Dest]) Template() *template.Template {
	return r.tpl
}

func (r *queryRunner[Dest]) SQL() fmt.Stringer {
	return r.writer
}

func (r *queryRunner[Dest]) Args() []any {
	return r.args
}

type QueryStatement[Param, Dest any] struct {
	ctx  func(ctx context.Context, runner Runner) context.Context
	log  func(ctx context.Context, err error, runner Runner)
	pool *sync.Pool
}

func runQuery[Param, Dest, Result any](s *QueryStatement[Param, Dest], ctx context.Context, param Param, exec func(ctx context.Context, runner *queryRunner[Dest]) (Result, error)) (Result, error) {
	var (
		result Result
		err    error
	)

	item := s.pool.Get()
	if err, ok := item.(error); ok {
		if s.log != nil {
			s.log(ctx, err, nil)
		}

		return result, err
	}

	runner := item.(*queryRunner[Dest])

	if s.ctx != nil {
		ctx = s.ctx(ctx, runner)
	}

	runner.dest = new(Dest)

	defer func() {
		if s.log != nil {
			s.log(ctx, err, runner)
		}

		runner.writer.Reset()
		runner.args = runner.args[:0]
		runner.scanners = runner.scanners[:0]
		runner.mappers = runner.mappers[:0]

		s.pool.Put(runner)
	}()

	if err = runner.tpl.Execute(runner.writer, param); err != nil {
		return result, err
	}

	if len(runner.scanners) == 0 {
		runner.scanners = []any{runner.dest}
	}

	result, err = exec(ctx, runner)
	if err != nil {
		return result, err
	}

	return result, err
}

func (qs *QueryStatement[Param, Dest]) All(ctx context.Context, db DB, param Param) ([]Dest, error) {
	return runQuery(qs, ctx, param, func(ctx context.Context, runner *queryRunner[Dest]) ([]Dest, error) {
		rows, err := db.QueryContext(ctx, runner.writer.String(), runner.args...)
		if err != nil {
			return nil, err
		}

		defer func() {
			err = errors.Join(err, rows.Close())
		}()

		var result []Dest

		for rows.Next() {
			if err = rows.Scan(runner.scanners...); err != nil {
				return nil, err
			}

			for _, m := range runner.mappers {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					return nil, err
				}
			}

			result = append(result, *runner.dest)
		}

		return result, err
	})
}

func (qs *QueryStatement[Param, Dest]) Limit(ctx context.Context, db DB, param Param, limit int) ([]Dest, error) {
	return runQuery(qs, ctx, param, func(ctx context.Context, runner *queryRunner[Dest]) ([]Dest, error) {
		rows, err := db.QueryContext(ctx, runner.writer.String(), runner.args...)
		if err != nil {
			return nil, err
		}

		defer func() {
			err = errors.Join(err, rows.Close())
		}()

		var result []Dest

		for rows.Next() {
			if err = rows.Scan(runner.scanners...); err != nil {
				return nil, err
			}

			for _, m := range runner.mappers {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					return nil, err
				}
			}

			result = append(result, *runner.dest)
		}

		return result, err
	})
}

var ErrTooManyRows = errors.New("too many rows")

func (qs *QueryStatement[Param, Dest]) One(ctx context.Context, db DB, param Param) (Dest, error) {
	return runQuery(qs, ctx, param, func(ctx context.Context, runner *queryRunner[Dest]) (Dest, error) {
		rows, err := db.QueryContext(ctx, runner.writer.String(), runner.args...)
		if err != nil {
			return *runner.dest, err
		}

		defer func() {
			err = errors.Join(err, rows.Close())
		}()

		if !rows.Next() {
			return *runner.dest, sql.ErrNoRows
		}

		err = rows.Scan(runner.scanners...)
		if err != nil {
			return *runner.dest, err
		}

		for _, m := range runner.mappers {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return *runner.dest, err
			}
		}

		if rows.Next() {
			return *runner.dest, ErrTooManyRows
		}

		return *runner.dest, err
	})
}

func (qs *QueryStatement[Param, Dest]) First(ctx context.Context, db DB, param Param) (Dest, error) {
	return runQuery(qs, ctx, param, func(ctx context.Context, runner *queryRunner[Dest]) (Dest, error) {
		err := db.QueryRowContext(ctx, runner.writer.String(), runner.args...).Scan(runner.scanners...)
		if err != nil {
			return *runner.dest, err
		}

		for _, m := range runner.mappers {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return *runner.dest, err
			}
		}

		return *runner.dest, nil
	})
}

type writer struct {
	data []byte
}

func (w *writer) Reset() {
	w.data = w.data[:0]
}

func (w *writer) Write(data []byte) (int, error) {
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

func (w *writer) String() string {
	if len(w.data) > 0 && w.data[len(w.data)-1] == ' ' {
		return string(w.data[:len(w.data)-1])
	}

	return string(w.data)
}

var ident = "___sqlt___"

// stolen from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func escape(text *template.Template) {
	for _, tpl := range text.Templates() {
		if tpl.Tree.Root == nil {
			continue
		}

		escapeNode(tpl.Tree, tpl.Tree.Root)
	}
}

func escapeNode(s *parse.Tree, n parse.Node) {
	switch v := n.(type) {
	case *parse.ActionNode:
		escapeNode(s, v.Pipe)
	case *parse.IfNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	case *parse.ListNode:
		if v == nil {
			return
		}

		for _, n := range v.Nodes {
			escapeNode(s, n)
		}
	case *parse.PipeNode:
		if len(v.Decl) > 0 {
			return
		}

		if len(v.Cmds) < 1 {
			return
		}

		cmd := v.Cmds[len(v.Cmds)-1]
		if len(cmd.Args) == 1 && cmd.Args[0].Type() == parse.NodeIdentifier && cmd.Args[0].(*parse.IdentifierNode).Ident == ident {
			return
		}

		v.Cmds = append(v.Cmds, &parse.CommandNode{
			NodeType: parse.NodeCommand,
			Args:     []parse.Node{parse.NewIdentifier(ident).SetTree(s).SetPos(cmd.Pos)},
		})
	case *parse.RangeNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	case *parse.WithNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	}
}

func goodName(name string) bool {
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r == '_':
		case i == 0 && !unicode.IsLetter(r):
			return false
		case !unicode.IsLetter(r) && !unicode.IsDigit(r):
			return false
		}
	}
	return true
}
