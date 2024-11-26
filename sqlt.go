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
	"runtime"
	"strings"
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

type Option interface {
	Configure(config *Config)
}

type Config struct {
	Start           Start
	End             End
	Placeholder     Placeholder
	TemplateOptions []TemplateOption
}

func (c Config) Configure(config *Config) {
	if c.Start != nil {
		config.Start = c.Start
	}

	if c.End != nil {
		config.End = c.End
	}

	if c.Placeholder != "" {
		config.Placeholder = c.Placeholder
	}

	if len(c.TemplateOptions) > 0 {
		config.TemplateOptions = append(config.TemplateOptions, c.TemplateOptions...)
	}
}

type Start func(runner *Runner)

func (s Start) Configure(config *Config) {
	config.Start = s
}

type End func(err error, runner *Runner)

func (e End) Configure(config *Config) {
	config.End = e
}

type Placeholder string

func (p Placeholder) Configure(config *Config) {
	config.Placeholder = p
}

func Dollar() Placeholder {
	return "$%d"
}

func Colon() Placeholder {
	return ":%d"
}

func AtP() Placeholder {
	return "@p%d"
}

func Question() Placeholder {
	return "?"
}

type TemplateOption func(tpl *template.Template) (*template.Template, error)

func (to TemplateOption) Configure(config *Config) {
	config.TemplateOptions = append(config.TemplateOptions, to)
}

func New(name string) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.New(name), nil
	}
}

func Parse(text string) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Parse(text)
	}
}

func ParseFS(fs fs.FS, patterns ...string) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFS(fs, patterns...)
	}
}

func ParseFiles(filenames ...string) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseFiles(filenames...)
	}
}

func ParseGlob(pattern string) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.ParseGlob(pattern)
	}
}

func Funcs(fm template.FuncMap) TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Funcs(fm), nil
	}
}

func MissingKeyInvalid() TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=invalid"), nil
	}
}

func MissingKeyZero() TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=zero"), nil
	}
}

func MissingKeyError() TemplateOption {
	return func(tpl *template.Template) (*template.Template, error) {
		return tpl.Option("missingkey=error"), nil
	}
}

func Lookup(name string) TemplateOption {
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

type Runner struct {
	Context  context.Context
	Template *template.Template
	SQL      *SQL
	Args     []any
	File     string
	Line     int
}

func (r *Runner) Reset() {
	r.Context = nil
	r.SQL.Reset()
	r.Args = r.Args[:0]
}

func (r *Runner) Exec(db DB, param any) (sql.Result, error) {
	if err := r.Template.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	return db.ExecContext(r.Context, r.SQL.String(), r.Args...)
}

func (r *Runner) Query(db DB, param any) (*sql.Rows, error) {
	if err := r.Template.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	return db.QueryContext(r.Context, r.SQL.String(), r.Args...)
}

func (r *Runner) QueryRow(db DB, param any) (*sql.Row, error) {
	if err := r.Template.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	return db.QueryRowContext(r.Context, r.SQL.String(), r.Args...), nil
}

func Stmt[Param any](opts ...Option) *Statement[Param] {
	_, file, line, _ := runtime.Caller(1)

	config := &Config{
		Placeholder: "?",
	}

	for _, opt := range opts {
		opt.Configure(config)
	}

	var (
		tpl = defaultTemplate().Funcs(template.FuncMap{
			"Dest": func() any {
				return nil
			},
		})
		err error
	)

	for _, to := range config.TemplateOptions {
		tpl, err = to(tpl)
		if err != nil {
			panic(err)
		}
	}

	if err = templatecheck.CheckText(tpl, *new(Param)); err != nil {
		panic(err)
	}

	escape(tpl)

	positional := strings.Contains(string(config.Placeholder), "%d")
	placeholder := string(config.Placeholder)

	return &Statement[Param]{
		start: config.Start,
		end:   config.End,
		pool: &sync.Pool{
			New: func() any {
				t, err := tpl.Clone()
				if err != nil {
					panic(err)
				}

				runner := &Runner{
					Template: t,
					SQL:      &SQL{},
					File:     file,
					Line:     line,
				}

				t.Funcs(template.FuncMap{
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						default:
							runner.Args = append(runner.Args, arg)

							if positional {
								return Raw(fmt.Sprintf(placeholder, len(runner.Args)))
							}

							return Raw(placeholder)
						}
					},
				})

				return runner
			},
		},
	}
}

type Statement[Param any] struct {
	start func(runner *Runner)
	end   func(err error, runner *Runner)
	pool  *sync.Pool
}

func (s *Statement[Param]) Get(ctx context.Context) *Runner {
	runner := s.pool.Get().(*Runner)

	runner.Context = ctx

	if s.start != nil {
		s.start(runner)
	}

	return runner
}

func (s *Statement[Param]) Put(err error, runner *Runner) {
	if s.end != nil {
		s.end(err, runner)
	}

	runner.Reset()

	s.pool.Put(runner)
}

func (s *Statement[Param]) Exec(ctx context.Context, db DB, param Param) (result sql.Result, err error) {
	runner := s.Get(ctx)

	defer s.Put(err, runner)

	return runner.Exec(db, param)
}

func (s *Statement[Param]) QueryRow(ctx context.Context, db DB, param Param) (row *sql.Row, err error) {
	runner := s.Get(ctx)

	defer s.Put(err, runner)

	return runner.QueryRow(db, param)
}

func (s *Statement[Param]) Query(ctx context.Context, db DB, param Param) (rows *sql.Rows, err error) {
	runner := s.Get(ctx)

	defer s.Put(err, runner)

	return runner.Query(db, param)
}

type QueryRunner[Dest any] struct {
	Runner  *Runner
	Dest    *Dest
	Values  []any
	Mappers []func() error
}

func (qr *QueryRunner[Dest]) Reset() {
	qr.Runner.Reset()
	qr.Values = qr.Values[:0]
	qr.Mappers = qr.Mappers[:0]
}

func QueryStmt[Param, Dest any](opts ...Option) *QueryStatement[Param, Dest] {
	_, file, line, _ := runtime.Caller(1)

	config := &Config{
		Placeholder: "?",
	}

	for _, opt := range opts {
		opt.Configure(config)
	}

	var (
		tpl = defaultTemplate().Funcs(template.FuncMap{
			"Dest": func() *Dest {
				return new(Dest)
			},
		})
		err error
	)

	destType := reflect.TypeFor[Dest]().Name()

	if goodName(destType) {
		tpl.Funcs(template.FuncMap{
			destType: func() *Dest {
				return new(Dest)
			},
		})
	}

	for _, to := range config.TemplateOptions {
		tpl, err = to(tpl)
		if err != nil {
			panic(err)
		}
	}

	if err = templatecheck.CheckText(tpl, *new(Param)); err != nil {
		panic(err)
	}

	escape(tpl)

	positional := strings.Contains(string(config.Placeholder), "%d")
	placeholder := string(config.Placeholder)

	return &QueryStatement[Param, Dest]{
		start: config.Start,
		end:   config.End,
		pool: &sync.Pool{
			New: func() any {
				t, err := tpl.Clone()
				if err != nil {
					panic(err)
				}

				runner := &QueryRunner[Dest]{
					Runner: &Runner{
						Template: t,
						SQL:      &SQL{},
						File:     file,
						Line:     line,
					},
					Dest: new(Dest),
				}

				if goodName(destType) {
					t.Funcs(template.FuncMap{
						destType: func() *Dest {
							return runner.Dest
						},
					})
				}

				t.Funcs(template.FuncMap{
					"Dest": func() *Dest {
						return runner.Dest
					},
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						case Scanner:
							runner.Values = append(runner.Values, a.Value)
							runner.Mappers = append(runner.Mappers, a.Map)

							return Raw(a.SQL)
						default:
							runner.Runner.Args = append(runner.Runner.Args, arg)

							if positional {
								return Raw(fmt.Sprintf(placeholder, len(runner.Runner.Args)))
							}

							return Raw(placeholder)
						}
					},
				})

				return runner
			},
		},
	}
}

type QueryStatement[Param, Dest any] struct {
	start func(runner *Runner)
	end   func(err error, runner *Runner)
	pool  *sync.Pool
}

func (qs *QueryStatement[Param, Dest]) Start(ctx context.Context) *QueryRunner[Dest] {
	runner := qs.pool.Get().(*QueryRunner[Dest])

	runner.Runner.Context = ctx

	if qs.start != nil {
		qs.start(runner.Runner)
	}

	return runner
}

func (qs *QueryStatement[Param, Dest]) End(err error, runner *QueryRunner[Dest]) {
	if qs.end != nil {
		qs.end(err, runner.Runner)
	}

	runner.Reset()

	qs.pool.Put(runner)
}

func (qs *QueryStatement[Param, Dest]) All(ctx context.Context, db DB, param Param) (result []Dest, err error) {
	runner := qs.Start(ctx)

	defer qs.End(err, runner)

	var rows *sql.Rows

	rows, err = runner.Runner.Query(db, param)
	if err != nil {
		return nil, err
	}

	if len(runner.Values) == 0 {
		runner.Values = []any{runner.Dest}
	}

	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	for rows.Next() {
		if err = rows.Scan(runner.Values...); err != nil {
			return nil, err
		}

		for _, m := range runner.Mappers {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return nil, err
			}
		}

		result = append(result, *runner.Dest)
	}

	return result, err
}

var ErrTooManyRows = errors.New("too many rows")

func (qs *QueryStatement[Param, Dest]) One(ctx context.Context, db DB, param Param) (result Dest, err error) {
	runner := qs.Start(ctx)

	defer qs.End(err, runner)

	var rows *sql.Rows

	rows, err = runner.Runner.Query(db, param)
	if err != nil {
		return *runner.Dest, err
	}

	if len(runner.Values) == 0 {
		runner.Values = []any{runner.Dest}
	}

	defer func() {
		err = errors.Join(err, rows.Close())
	}()

	if !rows.Next() {
		return *runner.Dest, sql.ErrNoRows
	}

	err = rows.Scan(runner.Values...)
	if err != nil {
		return *runner.Dest, err
	}

	for _, m := range runner.Mappers {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return *runner.Dest, err
		}
	}

	if rows.Next() {
		return *runner.Dest, ErrTooManyRows
	}

	return *runner.Dest, err
}

func (qs *QueryStatement[Param, Dest]) First(ctx context.Context, db DB, param Param) (result Dest, err error) {
	runner := qs.Start(ctx)

	defer qs.End(err, runner)

	var row *sql.Row

	row, err = runner.Runner.QueryRow(db, param)
	if err != nil {
		return *runner.Dest, err
	}

	if len(runner.Values) == 0 {
		runner.Values = []any{runner.Dest}
	}

	if err = row.Scan(runner.Values...); err != nil {
		return *runner.Dest, err
	}

	for _, m := range runner.Mappers {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return *runner.Dest, err
		}
	}

	return *runner.Dest, nil
}

type SQL struct {
	data []byte
}

func (w *SQL) Reset() {
	w.data = w.data[:0]
}

func (w *SQL) Write(data []byte) (int, error) {
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

func (w *SQL) String() string {
	if len(w.data) == 0 {
		return ""
	}

	if w.data[len(w.data)-1] == ' ' {
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
