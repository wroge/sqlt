package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strconv"
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
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
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
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid Scan at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, fmt.Errorf("invalid ScanJSON at '%s'", str)
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
					return Scanner{}, fmt.Errorf("invalid ScanBytes at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanString": func(dest *string, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanString at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt16": func(dest *int16, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanInt16 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt32": func(dest *int32, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanInt32 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanInt64": func(dest *int64, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanInt64 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanFloat32": func(dest *float32, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanFloat32 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanFloat64": func(dest *float64, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanFloat64 at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanBool": func(dest *bool, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanBool at '%s'", str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanTime": func(dest *time.Time, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, fmt.Errorf("invalid ScanTime at '%s'", str)
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

type Template struct {
	text         *template.Template
	placeholder  string
	positional   bool
	beforeRun    func(name string, r *Runner)
	afterRun     func(err error, name string, r *Runner) error
	pool         *sync.Pool
	size         int
	withDuration bool
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

func (t *Template) WithDuration() *Template {
	t.withDuration = true

	return t
}

func (t *Template) BeforeRun(handle func(name string, r *Runner)) *Template {
	t.beforeRun = handle

	return t
}

func (t *Template) AfterRun(handle func(err error, name string, r *Runner) error) *Template {
	t.afterRun = handle

	return t
}

func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)

	return t
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

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
	SQL     *bytes.Buffer
	Args    []any
	Dest    []any
	Map     []func() error
}

func (t *Template) Run(ctx context.Context, dest any, use func(runner *Runner) error) error {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner{
					Text: escape(text),
					SQL:  bytes.NewBuffer(make([]byte, 0, t.size)),
				}

				r.Text.Funcs(template.FuncMap{
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
								return t.placeholder + strconv.Itoa(len(r.Args))
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
	case *Runner:
		r.Context = ctx

		r.Text.Funcs(template.FuncMap{
			"Dest": func() any {
				return dest
			},
		})

		if t.beforeRun != nil {
			t.beforeRun(t.text.Name(), r)
		}

		err := use(r)

		if t.afterRun != nil {
			err = t.afterRun(err, t.text.Name(), r)
		}

		go func() {
			if size := r.SQL.Len(); size > t.size {
				t.size = size
			}

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

func (t *Template) Exec(ctx context.Context, db DB, params any) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)

	err = t.Run(ctx, nil, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
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

func (t *Template) Query(ctx context.Context, db DB, params any) (*sql.Rows, error) {
	var (
		rows *sql.Rows
		err  error
	)

	err = t.Run(ctx, nil, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
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

func (t *Template) QueryRow(ctx context.Context, db DB, params any) (*sql.Row, error) {
	var (
		row *sql.Row
		err error
	)

	err = t.Run(ctx, nil, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		row = db.QueryRowContext(ctx, r.SQL.String(), r.Args...)

		return nil
	})

	return row, err
}

func FetchEach[T any](ctx context.Context, t *Template, db DB, params any, each func(value T) (bool, error)) error {
	var (
		dest T
		rows *sql.Rows
		next bool
		err  error
	)

	err = t.Run(ctx, &dest, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		if len(r.Dest) == 0 {
			r.Dest = []any{&dest}
		}

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

			next, err = each(dest)
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

func FetchAll[T any](ctx context.Context, t *Template, db DB, params any) ([]T, error) {
	var (
		values []T
		err    error
	)

	err = FetchEach(ctx, t, db, params, func(value T) (bool, error) {
		values = append(values, value)

		return true, nil
	})

	return values, err
}

func FetchFirst[T any](ctx context.Context, t *Template, db DB, params any) (T, error) {
	var (
		val T
		err error
	)

	err = FetchEach(ctx, t, db, params, func(value T) (bool, error) {
		val = value

		return false, nil
	})

	return val, err
}

var ErrTooManyRows = fmt.Errorf("sqlt: too many rows")

func FetchOne[T any](ctx context.Context, t *Template, db DB, params any) (T, error) {
	var (
		val T
		one bool
		err error
	)

	err = FetchEach(ctx, t, db, params, func(value T) (bool, error) {
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
