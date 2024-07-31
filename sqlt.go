package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"slices"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
)

type DB interface {
	QueryContext(ctx context.Context, str string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, str string, args ...any) *sql.Row
	ExecContext(ctx context.Context, str string, args ...any) (sql.Result, error)
}

func InTx(ctx context.Context, opts *sql.TxOptions, db *sql.DB, do func(db DB) error) error {
	var (
		tx  *sql.Tx
		err error
	)

	tx, err = db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return do(tx)
}

type Raw string

type Scanner struct {
	SQL  string
	Dest any
	Map  func() error
}

func getTypes(list []any) []reflect.Type {
	types := make([]reflect.Type, len(list))

	for i, each := range list {
		types[i] = reflect.TypeOf(each)
	}

	return types
}

type Error struct {
	Err      error
	Template string
	SQL      string
	Args     []any
	Dest     []reflect.Type
}

func Dest[T, A any](t *Template[A]) *Template[T] {
	return &Template[T]{
		text:        t.text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
	}
}

func New(name string) *Template[any] {
	t := &Template[any]{
		text: template.New(name).Funcs(template.FuncMap{
			"Dest": func() any {
				return nil
			},
			"Raw": func(str string) Raw {
				return Raw(str)
			},
			"Scan": func(dest sql.Scanner, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Scanner at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, fmt.Errorf("invalid sqlt.JSON at '%s'", str)
				}

				var data []byte

				return Scanner{
					SQL:  str,
					Dest: &data,
					Map: func() error {
						return json.Unmarshal(data, dest)
					},
				}, nil
			},
			"ScanBytes": func(dest *[]byte, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.ByteSlice at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanString": func(dest *string, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.String at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt16": func(dest *int16, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Int16 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt32": func(dest *int32, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Int32 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt64": func(dest *int64, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Int64 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanFloat32": func(dest *float32, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Float32 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanFloat64": func(dest *float64, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Float64 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanBool": func(dest *bool, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Bool at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanTime": func(dest *time.Time, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid sqlt.Time at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
		}),
		placeholder: "?",
	}

	return t
}

type Template[T any] struct {
	text        *template.Template
	placeholder string
	positional  bool
	errHandler  func(err Error) error
	pool        *sync.Pool
}

func (t *Template[T]) New(name string) *Template[T] {
	return &Template[T]{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}
}

func (t *Template[T]) Placeholder(placeholder string, positional bool) *Template[T] {
	t.placeholder = placeholder
	t.positional = positional

	return t
}

func (t *Template[T]) Question() *Template[T] {
	return t.Placeholder("?", false)
}

func (t *Template[T]) Dollar() *Template[T] {
	return t.Placeholder("$", true)
}

func (t *Template[T]) Colon() *Template[T] {
	return t.Placeholder(":", true)
}

func (t *Template[T]) AtP() *Template[T] {
	return t.Placeholder("@p", true)
}

func (t *Template[T]) HandleErr(handle func(err Error) error) *Template[T] {
	t.errHandler = handle

	return t
}

func (t *Template[T]) Option(opt ...string) *Template[T] {
	t.text.Option(opt...)

	return t
}

func must[T any](n *Template[T], err error) *Template[T] {
	if err != nil {
		panic(err)
	}

	return n
}

func (t *Template[T]) Parse(str string) (*Template[T], error) {
	text, err := t.text.Parse(str)
	if err != nil {
		return nil, err
	}

	return &Template[T]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}, nil
}

func (t *Template[T]) MustParse(str string) *Template[T] {
	return must(t.Parse(str))
}

func (t *Template[T]) ParseFS(fsys fs.FS, patterns ...string) (*Template[T], error) {
	text, err := t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template[T]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}, nil
}

func (t *Template[T]) MustParseFS(fsys fs.FS, patterns ...string) *Template[T] {
	return must(t.ParseFS(fsys, patterns...))
}

func (t *Template[T]) ParseFiles(filenames ...string) (*Template[T], error) {
	text, err := t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template[T]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}, nil
}

func (t *Template[T]) MustParseFiles(filenames ...string) *Template[T] {
	return must(t.ParseFiles(filenames...))
}

func (t *Template[T]) ParseGlob(pattern string) (*Template[T], error) {
	text, err := t.text.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return &Template[T]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}, nil
}

func (t *Template[T]) MustParseGlob(pattern string) *Template[T] {
	return must(t.ParseGlob(pattern))
}

func (t *Template[T]) Funcs(fm template.FuncMap) *Template[T] {
	t.text.Funcs(fm)

	return t
}

func (t *Template[T]) Lookup(name string) (*Template[T], error) {
	text := t.text.Lookup(name)
	if text == nil {
		return nil, fmt.Errorf("template name '%s' not found", name)
	}

	return &Template[T]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		errHandler:  t.errHandler,
		pool:        t.pool,
	}, nil
}

func (t *Template[T]) MustLookup(name string) *Template[T] {
	return must(t.Lookup(name))
}

type Runner[T any] struct {
	Text  *template.Template
	SQL   *bytes.Buffer
	Args  []any
	Value *T
	Dest  []any
	Map   []func() error
}

func (t *Template[T]) Run(use func(runner *Runner[T]) error) error {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner[T]{
					Text:  escape(text),
					SQL:   &bytes.Buffer{},
					Value: new(T),
				}

				r.Text.Funcs(template.FuncMap{
					"Dest": func() any {
						return r.Value
					},
					ident: func(arg any) string {
						switch a := arg.(type) {
						case Scanner:
							r.Dest = append(r.Dest, a.Dest)
							r.Map = append(r.Map, a.Map)

							return a.SQL
						case Raw:
							return string(a)
						default:
							r.Args = append(r.Args, arg)

							if t.positional {
								return fmt.Sprintf("%s%d", t.placeholder, len(r.Args))
							}

							return t.placeholder
						}
					},
				})

				return r
			},
		}
	}

	switch r := t.pool.Get().(type) {
	case *Runner[T]:
		var err error

		if err = use(r); err != nil {
			if t.errHandler != nil {
				err = t.errHandler(Error{
					Err:      err,
					Template: r.Text.Name(),
					SQL:      strings.Join(strings.Fields(r.SQL.String()), " "),
					Args:     slices.Clone(r.Args),
					Dest:     getTypes(r.Dest),
				})
			}
		}

		go func() {
			r.SQL.Reset()
			r.Args = r.Args[:0]
			r.Dest = r.Dest[:0]
			r.Map = r.Map[:0]

			t.pool.Put(r)
		}()

		return err
	case error:
		return r
	default:
		panic(r)
	}
}

func (t *Template[T]) Exec(ctx context.Context, db DB, params any) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)

	err = t.Run(func(r *Runner[T]) error {
		if err := r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		result, err = db.ExecContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

func (t *Template[T]) Query(ctx context.Context, db DB, params any) (*sql.Rows, error) {
	var (
		rows *sql.Rows
		err  error
	)

	err = t.Run(func(r *Runner[T]) error {
		if err := r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		return nil
	})

	return rows, err
}

func (t *Template[T]) QueryRow(ctx context.Context, db DB, params any) (*sql.Row, error) {
	var (
		row *sql.Row
		err error
	)

	err = t.Run(func(r *Runner[T]) error {
		if err := r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		row = db.QueryRowContext(ctx, r.SQL.String(), r.Args...)

		return nil
	})

	return row, err
}

func (t *Template[T]) FetchEach(ctx context.Context, db DB, params any, each func(value T) (bool, error)) error {
	var err error

	err = t.Run(func(r *Runner[T]) error {
		if err := r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		if len(r.Dest) == 0 {
			r.Dest = []any{r.Value}
		}

		var rows *sql.Rows

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			if err = rows.Scan(r.Dest...); err != nil {
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

			next, err := each(*r.Value)
			if err != nil {
				return err
			}

			if !next {
				break
			}
		}

		if err = rows.Err(); err != nil {
			return err
		}

		if err = rows.Close(); err != nil {
			return err
		}

		return nil
	})

	return err
}

func (t *Template[T]) FetchAll(ctx context.Context, db DB, params any) ([]T, error) {
	var (
		values []T
		err    error
	)

	err = t.FetchEach(ctx, db, params, func(value T) (bool, error) {
		values = append(values, value)

		return true, nil
	})

	return values, err
}

var ErrTooManyRows = fmt.Errorf("sqlt: too many rows")

func (t *Template[T]) FetchAllN(ctx context.Context, db DB, params any, n int) ([]T, error) {
	var (
		values = make([]T, n)
		index  int
		err    error
	)

	err = t.FetchEach(ctx, db, params, func(value T) (bool, error) {
		if index >= n {
			return false, ErrTooManyRows
		}

		values[index] = value

		index++

		return true, nil
	})
	if err != nil {
		return nil, err
	}

	if index != n {
		return values, fmt.Errorf("sqlt: not enough rows")
	}

	return values, nil
}

func (t *Template[T]) FetchFirstN(ctx context.Context, db DB, params any, n int) ([]T, error) {
	var (
		values = make([]T, n)
		index  int
		err    error
	)

	err = t.FetchEach(ctx, db, params, func(value T) (bool, error) {
		values[index] = value

		index++

		return n > index, nil
	})

	return values, err
}

func (t *Template[T]) FetchFirst(ctx context.Context, db DB, params any) (T, error) {
	var (
		val T
		err error
	)

	err = t.FetchEach(ctx, db, params, func(value T) (bool, error) {
		val = value

		return false, nil
	})

	return val, err
}

func (t *Template[T]) FetchOne(ctx context.Context, db DB, params any) (T, error) {
	var (
		val T
		one bool
		err error
	)

	err = t.FetchEach(ctx, db, params, func(value T) (bool, error) {
		if one {
			return false, ErrTooManyRows
		}

		val = value

		one = true

		return false, nil
	})

	return val, err
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
