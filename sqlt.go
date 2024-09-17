package sqlt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"reflect"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"

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

type Raw string

type Scanner struct {
	Dest any
	Map  func() error
	SQL  string
}

type Slice[T any] []T

func (s Slice[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal([]T(s))
}

func (s *Slice[T]) UnmarshalJSON(data []byte) error {
	var list []T

	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	*s = list

	return nil
}

type Map[K comparable, V any] map[K]V

func (m Map[K, V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(map[K]V(m))
}

func (m *Map[K, V]) UnmarshalJSON(data []byte) error {
	var t map[K]V

	if err := json.Unmarshal(data, &t); err != nil {
		return err
	}

	*m = t

	return nil
}

func Scan[T any](dest *T, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, errors.New("invalid nil pointer")
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

type Runner struct {
	ctx       context.Context
	text      *template.Template
	sqlWriter *sqlWriter
	Value     any
	args      []any
	dest      []any
	mapper    []func() error
}

func (r *Runner) SetValue(key, val any) {
	if r.ctx == nil {
		r.ctx = context.Background()
	}

	r.ctx = context.WithValue(r.ctx, key, val)
}

func (r *Runner) GetValue(key any) any {
	if r.ctx == nil {
		return nil
	}

	return r.ctx.Value(key)
}

func (r *Runner) Template() string {
	return r.text.Name()
}

func (r *Runner) SQL() string {
	return r.sqlWriter.String()
}

func (r *Runner) Args() []any {
	return r.args
}

func (r *Runner) Exec(ctx context.Context, db DB, param any) (sql.Result, error) {
	if err := r.text.Execute(r.sqlWriter, param); err != nil {
		return nil, err
	}

	return db.ExecContext(ctx, r.sqlWriter.String(), r.args...)
}

func (r *Runner) RowsAffected(ctx context.Context, db DB, param any) (int64, error) {
	result, err := r.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (r *Runner) LastInsertId(ctx context.Context, db DB, param any) (int64, error) {
	result, err := r.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func (r *Runner) Query(ctx context.Context, db DB, param any) (*sql.Rows, error) {
	if err := r.text.Execute(r.sqlWriter, param); err != nil {
		return nil, err
	}

	return db.QueryContext(ctx, r.sqlWriter.String(), r.args...)
}

func (r *Runner) QueryRow(ctx context.Context, db DB, param any) (*sql.Row, error) {
	if err := r.text.Execute(r.sqlWriter, param); err != nil {
		return nil, err
	}

	return db.QueryRowContext(ctx, r.sqlWriter.String(), r.args...), nil
}

func (r *Runner) ScanFirst(ctx context.Context, db DB, param any, dest any) error {
	r.Value = dest

	row, err := r.QueryRow(ctx, db, param)
	if err != nil {
		return err
	}

	if len(r.dest) == 0 {
		r.dest = []any{dest}
	}

	if err = row.Scan(r.dest...); err != nil {
		return err
	}

	for _, m := range r.mapper {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return err
		}
	}

	return nil
}

var ErrTooManyRows = fmt.Errorf("too many rows")

func (r *Runner) ScanOne(ctx context.Context, db DB, param any, dest any) error {
	seq, close := r.ScanIter(ctx, db, param, dest)

	for index := range seq {
		if index > 0 {
			return ErrTooManyRows
		}
	}

	return close()
}

func (r *Runner) ScanIter(ctx context.Context, db DB, param any, dest any) (iter.Seq[int], func() error) {
	var (
		err   error
		rows  *sql.Rows
		index int
	)

	r.Value = dest

	if err = r.text.Execute(r.sqlWriter, param); err != nil {
		return nil, func() error { return err }
	}

	if len(r.dest) == 0 {
		r.dest = append(r.dest, dest)
	}

	rows, err = db.QueryContext(ctx, r.sqlWriter.String(), r.args...)
	if err != nil {
		return nil, func() error { return err }
	}

	return func(yield func(int) bool) {
			for rows.Next() {
				if err = rows.Scan(r.dest...); err != nil {
					return
				}

				for _, m := range r.mapper {
					if m == nil {
						continue
					}

					if err = m(); err != nil {
						return
					}
				}

				if !yield(index) {
					return
				}

				index++
			}
		}, func() error {
			return errors.Join(err, rows.Err(), rows.Close())
		}
}

type sqlWriter struct {
	buf []byte
}

func (s *sqlWriter) Write(data []byte) (int, error) {
	for _, b := range data {
		switch b {
		case ' ', '\n', '\r', '\t':
			if len(s.buf) > 0 && s.buf[len(s.buf)-1] != ' ' {
				s.buf = append(s.buf, ' ')
			}
		default:
			s.buf = append(s.buf, b)
		}
	}

	return len(data), nil
}

func (s *sqlWriter) String() string {
	if len(s.buf) > 0 && s.buf[len(s.buf)-1] == ' ' {
		return string(s.buf[:len(s.buf)-1])
	}

	return string(s.buf)
}

func (s *sqlWriter) Reset() {
	s.buf = s.buf[:0]
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

func New(name string) *Template {
	t := &Template{
		text: template.New(name).Funcs(template.FuncMap{
			"Dest": func() any {
				return nil
			},
			"Raw": func(str string) Raw {
				return Raw(str)
			},
			"Scan": func(dest sql.Scanner, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, errors.New("invalid nil pointer")
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, errors.New("invalid nil pointer")
				}

				var data []byte

				return Scanner{
					SQL:  str,
					Dest: &data,
					Map: func() error {
						if err := dest.UnmarshalJSON(data); err != nil {
							return err
						}

						return nil
					},
				}, nil
			},
			"ScanString":   Scan[string],
			"ScanBytes":    Scan[[]byte],
			"ScanInt":      Scan[int],
			"ScanInt8":     Scan[int8],
			"ScanInt16":    Scan[int16],
			"ScanInt32":    Scan[int32],
			"ScanInt64":    Scan[int64],
			"ScanUint":     Scan[uint],
			"ScanUint8":    Scan[uint8],
			"ScanUint16":   Scan[uint16],
			"ScanUint32":   Scan[uint32],
			"ScanUint64":   Scan[uint64],
			"ScanBool":     Scan[bool],
			"ScanFloat32":  Scan[float32],
			"ScanFloat64":  Scan[float64],
			"ScanTime":     Scan[time.Time],
			"ScanDuration": Scan[time.Duration],
		}),
		placeholder: "?",
	}

	return t
}

type Template struct {
	text        *template.Template
	beforeRun   func(r *Runner)
	afterRun    func(err error, r *Runner) error
	pool        *sync.Pool
	placeholder string
	positional  bool
}

func (t *Template) make(text *template.Template) *Template {
	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
	}
}

func (t *Template) New(name string) *Template {
	return &Template{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
	}
}

func (t *Template) Name() string {
	return t.text.Name()
}

func (t *Template) Placeholder(placeholder string, positional bool) *Template {
	t.placeholder = placeholder
	t.positional = positional

	return t
}

func (t *Template) Question() *Template {
	return t.Placeholder("?", false)
}

func (t *Template) Dollar() *Template {
	return t.Placeholder("$", true)
}

func (t *Template) Colon() *Template {
	return t.Placeholder(":", true)
}

func (t *Template) AtP() *Template {
	return t.Placeholder("@p", true)
}

func (t *Template) BeforeRun(handle func(r *Runner)) *Template {
	t.beforeRun = handle

	return t
}

func (t *Template) AfterRun(handle func(err error, r *Runner) error) *Template {
	t.afterRun = handle

	return t
}

func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)

	return t
}

func (t *Template) Parse(str string) (*Template, error) {
	var err error

	t.text, err = t.text.Parse(str)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Template) MustParse(str string) *Template {
	return Must(t.Parse(str))
}

func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	var err error

	t.text, err = t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Template) MustParseFS(fsys fs.FS, patterns ...string) *Template {
	return Must(t.ParseFS(fsys, patterns...))
}

func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	var err error

	t.text, err = t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Template) MustParseFiles(filenames ...string) *Template {
	return Must(t.ParseFiles(filenames...))
}

func (t *Template) ParseGlob(pattern string) (*Template, error) {
	var err error

	t.text, err = t.text.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return t, nil
}

func (t *Template) MustParseGlob(pattern string) *Template {
	return Must(t.ParseGlob(pattern))
}

func (t *Template) Funcs(fm template.FuncMap) *Template {
	t.text.Funcs(fm)

	return t
}

func (t *Template) Lookup(name string) (*Template, error) {
	text := t.text.Lookup(name)
	if text == nil {
		return nil, fmt.Errorf("template name '%s' not found", name)
	}

	return t.make(text), nil
}

func (t *Template) MustLookup(name string) *Template {
	return Must(t.Lookup(name))
}

func (t *Template) GetRunner(ctx context.Context) *Runner {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner{
					text:      escape(text),
					sqlWriter: &sqlWriter{buf: make([]byte, 0)},
				}

				r.text.Funcs(template.FuncMap{
					"Dest": func() any {
						return r.Value
					},
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						case Scanner:
							r.dest = append(r.dest, a.Dest)
							r.mapper = append(r.mapper, a.Map)

							return Raw(a.SQL)
						default:
							r.args = append(r.args, arg)

							if t.positional {
								return Raw(t.placeholder + strconv.Itoa(len(r.args)))
							}

							return Raw(t.placeholder)
						}
					},
				})

				return r
			},
		}
	}

	switch r := t.pool.Get().(type) {
	case *Runner:
		r.ctx = ctx

		if t.beforeRun != nil {
			t.beforeRun(r)
		}

		return r
	default:
		panic(r)
	}
}

func (t *Template) PutRunner(err error, r *Runner) error {
	if t.afterRun != nil {
		err = t.afterRun(err, r)
	}

	r.sqlWriter.Reset()
	r.args = r.args[:0]
	r.dest = r.dest[:0]
	r.mapper = r.mapper[:0]

	t.pool.Put(r)

	return err
}

func (t *Template) Exec(ctx context.Context, db DB, param any) (sql.Result, error) {
	r := t.GetRunner(ctx)

	result, err := r.Exec(ctx, db, param)

	return result, t.PutRunner(err, r)
}

func (t *Template) RowsAffected(ctx context.Context, db DB, param any) (int64, error) {
	r := t.GetRunner(ctx)

	aff, err := r.RowsAffected(ctx, db, param)

	return aff, t.PutRunner(err, r)
}

func (t *Template) LastInsertId(ctx context.Context, db DB, param any) (int64, error) {
	r := t.GetRunner(ctx)

	id, err := r.LastInsertId(ctx, db, param)

	return id, t.PutRunner(err, r)
}

func (t *Template) Query(ctx context.Context, db DB, param any) (*sql.Rows, error) {
	r := t.GetRunner(ctx)

	rows, err := r.Query(ctx, db, param)

	return rows, t.PutRunner(err, r)
}

func (t *Template) QueryRow(ctx context.Context, db DB, param any) (*sql.Row, error) {
	r := t.GetRunner(ctx)

	row, err := r.QueryRow(ctx, db, param)

	return row, t.PutRunner(err, r)
}

func (t *Template) ScanFirst(ctx context.Context, db DB, param any, dest any) error {
	r := t.GetRunner(ctx)

	return t.PutRunner(r.ScanFirst(ctx, db, param, dest), r)
}

func (t *Template) ScanOne(ctx context.Context, db DB, param any, dest any) error {
	r := t.GetRunner(ctx)

	return t.PutRunner(r.ScanOne(ctx, db, param, dest), r)
}

func (t *Template) ScanIter(ctx context.Context, db DB, param any, dest any) (iter.Seq[int], func() error) {
	r := t.GetRunner(ctx)

	seq, close := r.ScanIter(ctx, db, param, dest)

	return seq, func() error {
		return t.PutRunner(close(), r)
	}
}

func MustType[Dest, Param any](t *Template) *TypedTemplate[Dest, Param] {
	tpl, err := Type[Dest, Param](t)
	if err != nil {
		panic(err)
	}

	return tpl
}

func Type[Dest, Param any](t *Template) (*TypedTemplate[Dest, Param], error) {
	t.text.Funcs(template.FuncMap{
		"Dest": func() *Dest {
			return new(Dest)
		},
	})

	return &TypedTemplate[Dest, Param]{
		Template: t,
	}, templatecheck.CheckText(t.text, *new(Param))
}

type TypedTemplate[Dest, Param any] struct {
	Template *Template
}

func (t *TypedTemplate[Dest, Param]) Exec(ctx context.Context, db DB, param Param) (sql.Result, error) {
	return t.Template.Exec(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) RowsAffected(ctx context.Context, db DB, param Param) (int64, error) {
	return t.Template.RowsAffected(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) LastInsertId(ctx context.Context, db DB, param Param) (int64, error) {
	return t.Template.LastInsertId(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) QueryRow(ctx context.Context, db DB, param Param) (*sql.Row, error) {
	return t.Template.QueryRow(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) First(ctx context.Context, db DB, param Param) (Dest, error) {
	var dest Dest

	return dest, t.Template.ScanFirst(ctx, db, param, &dest)
}

func (t *TypedTemplate[Dest, Param]) ScanFirst(ctx context.Context, db DB, param Param, dest *Dest) error {
	return t.Template.ScanFirst(ctx, db, param, dest)
}

func (t *TypedTemplate[Dest, Param]) Query(ctx context.Context, db DB, param Param) (*sql.Rows, error) {
	return t.Template.Query(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) One(ctx context.Context, db DB, param Param) (Dest, error) {
	var dest Dest

	return dest, t.Template.ScanOne(ctx, db, param, &dest)
}

func (t *TypedTemplate[Dest, Param]) ScanOne(ctx context.Context, db DB, param Param, dest *Dest) error {
	return t.Template.ScanOne(ctx, db, param, dest)
}

func (t *TypedTemplate[Dest, Param]) ScanIter(ctx context.Context, db DB, param Param, dest *Dest) (iter.Seq[int], func() error) {
	return t.Template.ScanIter(ctx, db, param, dest)
}

func (t *TypedTemplate[Dest, Param]) All(ctx context.Context, db DB, param Param) ([]Dest, error) {
	var (
		values = []Dest{}
		dest   Dest
	)

	seq, close := t.ScanIter(ctx, db, param, &dest)

	for range seq {
		values = append(values, dest)
	}

	return values, close()
}

var ident = "__sqlt__"

// stolen from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func escape(text *template.Template) *template.Template {
	for _, tpl := range text.Templates() {
		escapeTree(tpl.Tree)
	}

	return text
}

func escapeTree(s *parse.Tree) *parse.Tree {
	if s.Root == nil {
		return s
	}

	escapeNode(s, s.Root)

	return s
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
