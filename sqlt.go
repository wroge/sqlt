package sqlt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"iter"
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

func must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

func New(name string) *Template {
	t := &Template{
		text: template.New(name).Funcs(template.FuncMap{
			// ident is a stub function
			ident: func(arg any) Raw {
				return ""
			},
			// Dest is a stub function
			"Dest": func() any {
				return nil
			},
			"Raw": func(str string) Raw {
				return Raw(str)
			},
			"Scan": func(dest sql.Scanner, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, errors.New("invalid nil pointer")
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil {
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
	cap         int
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

func (t *Template) Delims(left, right string) *Template {
	t.text.Delims(left, right)

	return t
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

	t.text = escape(t.text)

	return t, nil
}

func (t *Template) MustParse(str string) *Template {
	return must(t.Parse(str))
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
	return must(t.ParseFS(fsys, patterns...))
}

func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	var err error

	t.text, err = t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	t.text = escape(t.text)

	return t, nil
}

func (t *Template) MustParseFiles(filenames ...string) *Template {
	return must(t.ParseFiles(filenames...))
}

func (t *Template) ParseGlob(pattern string) (*Template, error) {
	var err error

	t.text, err = t.text.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	t.text = escape(t.text)

	return t, nil
}

func (t *Template) MustParseGlob(pattern string) *Template {
	return must(t.ParseGlob(pattern))
}

func (t *Template) Funcs(fm template.FuncMap) *Template {
	t.text.Funcs(fm)

	return t
}

func (t *Template) Lookup(name string) (*Template, error) {
	text := t.text.Lookup(name)
	if text == nil {
		return nil, fmt.Errorf("template '%s' not found", name)
	}

	return t.make(text), nil
}

func (t *Template) MustLookup(name string) *Template {
	return must(t.Lookup(name))
}

type Runner struct {
	Context  context.Context
	Template *template.Template
	SQL      []byte
	Value    any
	Args     []any
	Dest     []any
	Map      []func() error
}

func (r *Runner) Write(data []byte) (int, error) {
	for _, b := range data {
		switch b {
		case ' ', '\n', '\r', '\t':
			if len(r.SQL) > 0 && r.SQL[len(r.SQL)-1] != ' ' {
				r.SQL = append(r.SQL, ' ')
			}
		default:
			r.SQL = append(r.SQL, b)
		}
	}

	return len(data), nil
}

func (r *Runner) Execute(param any) error {
	if err := r.Template.Execute(r, param); err != nil {
		return err
	}

	if len(r.SQL) > 0 && r.SQL[len(r.SQL)-1] == ' ' {
		r.SQL = r.SQL[:len(r.SQL)-1]
	}

	return nil
}

func (r *Runner) Reset() {
	r.Context = nil
	r.SQL = r.SQL[:0]
	r.Args = r.Args[:0]
	r.Dest = r.Dest[:0]
	r.Map = r.Map[:0]
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
					Template: text,
					SQL:      make([]byte, 0, t.cap),
				}

				r.Template.Funcs(template.FuncMap{
					"Dest": func() any {
						return r.Value
					},
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Raw:
							return a
						case Scanner:
							r.Dest = append(r.Dest, a.Dest)
							r.Map = append(r.Map, a.Map)

							return Raw(a.SQL)
						default:
							r.Args = append(r.Args, arg)

							if t.positional {
								return Raw(t.placeholder + strconv.Itoa(len(r.Args)))
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
		r.Context = ctx

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

	if c := cap(r.SQL); c > t.cap {
		t.cap = c
	}

	r.Reset()

	t.pool.Put(r)

	return err
}

func (t *Template) Exec(ctx context.Context, db DB, param any) (res sql.Result, err error) {
	r := t.GetRunner(ctx)

	defer func() {
		err = t.PutRunner(err, r)
	}()

	if err = r.Execute(param); err != nil {
		return nil, err
	}

	return db.ExecContext(ctx, string(r.SQL), r.Args...)
}

func (t *Template) RowsAffected(ctx context.Context, db DB, param any) (int64, error) {
	res, err := t.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return res.RowsAffected()
}

func (t *Template) LastInsertId(ctx context.Context, db DB, param any) (int64, error) {
	res, err := t.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return res.LastInsertId()
}

func (t *Template) Query(ctx context.Context, db DB, param any) (rows *sql.Rows, err error) {
	r := t.GetRunner(ctx)

	defer func() {
		err = t.PutRunner(err, r)
	}()

	if err = r.Execute(param); err != nil {
		return nil, err
	}

	return db.QueryContext(ctx, string(r.SQL), r.Args...)
}

func (t *Template) QueryRow(ctx context.Context, db DB, param any) (row *sql.Row, err error) {
	r := t.GetRunner(ctx)

	defer func() {
		err = t.PutRunner(err, r)
	}()

	if err = r.Execute(param); err != nil {
		return nil, err
	}

	return db.QueryRowContext(ctx, string(r.SQL), r.Args...), nil
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

	return dest, t.ScanFirst(ctx, db, param, &dest)
}

func (t *TypedTemplate[Dest, Param]) ScanFirst(ctx context.Context, db DB, param Param, dest *Dest) (err error) {
	r := t.Template.GetRunner(ctx)

	defer func() {
		err = t.Template.PutRunner(err, r)
	}()

	r.Value = dest

	if err = r.Execute(param); err != nil {
		return err
	}

	if len(r.Dest) == 0 {
		r.Dest = []any{dest}
	}

	if err = db.QueryRowContext(ctx, string(r.SQL), r.Args...).Scan(r.Dest...); err != nil {
		return err
	}

	for _, m := range r.Map {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return err
		}
	}

	return nil
}

func (t *TypedTemplate[Dest, Param]) Query(ctx context.Context, db DB, param Param) (*sql.Rows, error) {
	return t.Template.Query(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) One(ctx context.Context, db DB, param Param) (Dest, error) {
	var dest Dest

	return dest, t.ScanOne(ctx, db, param, &dest)
}

var ErrTooManyRows = fmt.Errorf("too many rows")

func (t *TypedTemplate[Dest, Param]) ScanOne(ctx context.Context, db DB, param Param, dest *Dest) (err error) {
	seq, close := t.ScanIter(ctx, db, param, dest)
	defer func() {
		err = errors.Join(err, close())
	}()

	err = sql.ErrNoRows

	for index := range seq {
		if index > 0 {
			return ErrTooManyRows
		}

		err = nil
	}

	return err
}

func (t *TypedTemplate[Dest, Param]) ScanIter(ctx context.Context, db DB, param Param, dest *Dest) (iter.Seq[int], func() error) {
	r := t.Template.GetRunner(ctx)

	var (
		err   error
		rows  *sql.Rows
		index int
	)

	r.Value = dest

	if err = r.Execute(param); err != nil {
		return func(yield func(int) bool) {}, func() error { return t.Template.PutRunner(err, r) }
	}

	if len(r.Dest) == 0 {
		r.Dest = append(r.Dest, dest)
	}

	rows, err = db.QueryContext(ctx, string(r.SQL), r.Args...)
	if err != nil {
		return func(yield func(int) bool) {}, func() error { return t.Template.PutRunner(err, r) }
	}

	return func(yield func(int) bool) {
			for rows.Next() {
				if err = rows.Scan(r.Dest...); err != nil {
					return
				}

				for _, m := range r.Map {
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
		},
		func() error {
			return errors.Join(t.Template.PutRunner(err, r), rows.Err(), rows.Close())
		}
}

func (t *TypedTemplate[Dest, Param]) ScanAll(ctx context.Context, db DB, param Param, dest *[]Dest) (err error) {
	var d Dest

	seq, close := t.ScanIter(ctx, db, param, &d)
	defer func() {
		err = errors.Join(err, close())
	}()

	for range seq {
		*dest = append(*dest, d)
	}

	return nil
}

func (t *TypedTemplate[Dest, Param]) All(ctx context.Context, db DB, param Param) ([]Dest, error) {
	var values = []Dest{}

	return values, t.ScanAll(ctx, db, param, &values)
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
