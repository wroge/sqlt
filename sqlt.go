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
	"unsafe"

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
	SQL  string
	Dest any
	Map  func() error
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

var ErrInvalidNilPointer = errors.New("invalid nil pointer")

func Scan[T any](dest *T, str string) (Scanner, error) {
	if dest == nil || reflect.ValueOf(dest).IsNil() {
		return Scanner{}, ErrInvalidNilPointer
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
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
					return Scanner{}, ErrInvalidNilPointer
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, ErrInvalidNilPointer
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
			"Type": func(typ string, arg any) (any, error) {
				if got := reflect.TypeOf(arg).String(); got != typ {
					return nil, fmt.Errorf("expected arg with type '%s' but got '%s'", typ, got)
				}

				return arg, nil
			},
			"NotNil": func(arg any) (any, error) {
				if arg == nil || reflect.ValueOf(arg).IsNil() {
					return nil, errors.New("arg is nil")
				}

				return arg, nil
			},
			"NotZero": func(arg any) (any, error) {
				if arg == nil || reflect.ValueOf(arg).IsZero() {
					return nil, errors.New("arg is zero")
				}

				return arg, nil
			},
		}),
		placeholder: "?",
	}

	return t
}

type Template struct {
	text        *template.Template
	beforeRun   func(runner *Runner)
	afterRun    func(err error, runner *Runner) error
	pool        *sync.Pool
	placeholder string
	positional  bool
}

func (t *Template) New(name string) *Template {
	return &Template{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
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

func (t *Template) BeforeRun(handle func(runner *Runner)) *Template {
	t.beforeRun = handle

	return t
}

func (t *Template) AfterRun(handle func(err error, runner *Runner) error) *Template {
	t.afterRun = handle

	return t
}

func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)

	return t
}

func (t *Template) Parse(str string) (*Template, error) {
	text, err := t.text.Parse(str)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

func (t *Template) MustParse(str string) *Template {
	return Must(t.Parse(str))
}

func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	text, err := t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

func (t *Template) MustParseFS(fsys fs.FS, patterns ...string) *Template {
	return Must(t.ParseFS(fsys, patterns...))
}

func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	text, err := t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

func (t *Template) MustParseFiles(filenames ...string) *Template {
	return Must(t.ParseFiles(filenames...))
}

func (t *Template) ParseGlob(pattern string) (*Template, error) {
	text, err := t.text.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
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

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

func (t *Template) MustLookup(name string) *Template {
	return Must(t.Lookup(name))
}

type Runner struct {
	Context context.Context
	Text    *template.Template
	SQL     *SQL
	Value   any
	Args    []any
	Dest    []any
	Map     []func() error
}

func (r *Runner) Query(ctx context.Context, db DB, param any) (*sql.Rows, error) {
	if err := r.Text.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	return db.QueryContext(ctx, r.SQL.String(), r.Args...)
}

func (r *Runner) QueryRow(ctx context.Context, db DB, param any) (*sql.Row, error) {
	if err := r.Text.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, r.SQL.String(), r.Args...)

	if err := row.Err(); err != nil {
		return nil, err
	}

	return row, nil
}

func (r *Runner) Exec(ctx context.Context, db DB, param any) (sql.Result, error) {
	if err := r.Text.Execute(r.SQL, param); err != nil {
		return nil, err
	}

	return db.ExecContext(ctx, r.SQL.String(), r.Args...)
}

func (r *Runner) ScanOne(ctx context.Context, db DB, param any, dest any) error {
	next, stop := iter.Pull(r.Scan(ctx, db, param, dest))

	defer stop()

	err, ok := next()
	if err != nil {
		return err
	}

	if !ok {
		return sql.ErrNoRows
	}

	err, ok = next()
	if err != nil {
		return err
	}

	if ok {
		return ErrTooManyRows
	}

	return nil
}

func (r *Runner) Scan(ctx context.Context, db DB, param any, dest any) iter.Seq[error] {
	return func(yield func(error) bool) {
		r.Value = dest

		if err := r.Text.Execute(r.SQL, param); err != nil {
			yield(err)

			return
		}

		if len(r.Dest) == 0 {
			r.Dest = append(r.Dest, dest)
		}

		rows, err := db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			yield(err)

			return
		}

		defer func() {
			err = errors.Join(err, rows.Close())
		}()

		for rows.Next() {
			if err = rows.Scan(r.Dest...); err != nil {
				yield(err)

				return
			}

			for _, m := range r.Map {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					yield(err)

					return
				}
			}

			if !yield(nil) {
				return
			}
		}

		if err = rows.Err(); err != nil {
			yield(err)

			return
		}

		if err = rows.Close(); err != nil {
			yield(err)

			return
		}
	}
}

func newSQL() *SQL {
	return &SQL{
		buf: make([]byte, 0),
	}
}

type SQL struct {
	buf []byte
}

func (s *SQL) Write(data []byte) (int, error) {
	start, end := 0, 0
	bufLen := len(s.buf)

	for start < len(data) {
		for start < len(data) && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
			start++
		}

		end = start
		for end < len(data) && !(data[end] == ' ' || data[end] == '\n' || data[end] == '\r' || data[end] == '\t') {
			end++
		}

		if start < end {
			wordLen := end - start

			if bufLen > 0 {
				s.buf = append(s.buf, ' ')
				bufLen++
			}

			s.buf = append(s.buf, data[start:end]...)
			bufLen += wordLen
		}

		start = end
	}

	return len(data), nil
}

func (s *SQL) String() string {
	return *(*string)(unsafe.Pointer(&s.buf))
}

func (s *SQL) Reset() {
	s.buf = s.buf[:0]
}

func (t *Template) GetRunner(ctx context.Context) (*Runner, error) {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner{
					Text: escape(text),
					SQL:  newSQL(),
				}

				r.Text.Funcs(template.FuncMap{
					"Dest": func() any {
						return r.Value
					},
					ident: func(arg any) Raw {
						switch a := arg.(type) {
						case Scanner:
							r.Dest = append(r.Dest, a.Dest)
							r.Map = append(r.Map, a.Map)

							return Raw(a.SQL)
						case Raw:
							return a
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

		return r, nil
	case error:
		return nil, r
	}

	return nil, errors.New("invalid runner")
}

func (t *Template) PutRunner(err error, r *Runner) error {
	if t.afterRun != nil {
		err = t.afterRun(err, r)
	}

	if r == nil {
		return err
	}

	r.SQL.Reset()
	r.Args = r.Args[:0]
	r.Dest = r.Dest[:0]
	r.Map = r.Map[:0]

	t.pool.Put(r)

	return err
}

func (t *Template) Exec(ctx context.Context, db DB, param any) (sql.Result, error) {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	result, err := r.Exec(ctx, db, param)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	return result, t.PutRunner(nil, r)
}

func (t *Template) Query(ctx context.Context, db DB, param any) (*sql.Rows, error) {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	rows, err := r.Query(ctx, db, param)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	return rows, t.PutRunner(nil, r)
}

func (t *Template) QueryRow(ctx context.Context, db DB, param any) (*sql.Row, error) {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	row, err := r.QueryRow(ctx, db, param)
	if err != nil {
		return nil, t.PutRunner(err, r)
	}

	return row, t.PutRunner(nil, r)
}

func (t *Template) RowsAffected(ctx context.Context, db DB, param any) (int64, error) {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	result, err := r.Exec(ctx, db, param)
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	aff, err := result.RowsAffected()
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	return aff, t.PutRunner(nil, r)
}

func (t *Template) LastInsertId(ctx context.Context, db DB, param any) (int64, error) {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	result, err := t.Exec(ctx, db, param)
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return 0, t.PutRunner(err, r)
	}

	return id, t.PutRunner(nil, r)
}

func (t *Template) ScanOne(ctx context.Context, db DB, param any, dest any) error {
	r, err := t.GetRunner(ctx)
	if err != nil {
		return t.PutRunner(err, r)
	}

	err = r.ScanOne(ctx, db, param, dest)
	if err != nil {
		return t.PutRunner(err, r)
	}

	return t.PutRunner(nil, r)
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

func (t *TypedTemplate[Dest, Param]) Query(ctx context.Context, db DB, param Param) (*sql.Rows, error) {
	return t.Template.Query(ctx, db, param)
}

func (t *TypedTemplate[Dest, Param]) QueryRow(ctx context.Context, db DB, param Param) (*sql.Row, error) {
	return t.Template.QueryRow(ctx, db, param)
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

func (t *TypedTemplate[Dest, Param]) All(ctx context.Context, db DB, param Param) ([]Dest, error) {
	r, err := t.Template.GetRunner(ctx)
	if err != nil {
		return nil, t.Template.PutRunner(err, r)
	}

	var (
		values = []Dest{}
		dest   Dest
	)

	for err := range r.Scan(ctx, db, param, &dest) {
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return nil, t.Template.PutRunner(nil, r)
			}

			return nil, t.Template.PutRunner(err, r)
		}

		values = append(values, dest)
	}

	return values, t.Template.PutRunner(nil, r)
}

var ErrTooManyRows = fmt.Errorf("too many rows")

func (t *TypedTemplate[Dest, Param]) One(ctx context.Context, db DB, param Param) (Dest, error) {
	var dest Dest

	return dest, t.Template.ScanOne(ctx, db, param, &dest)
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
