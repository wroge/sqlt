package sqlt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"strings"
	"text/template"
	"text/template/parse"
	"time"
)

type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *sql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (sql.Result, error)
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

type Scanner struct {
	SQL  string
	Args []any
	Dest any
	Map  func() error
}

type Raw string

type Expression struct {
	SQL  string
	Args []any
}

var DefaultFuncs = template.FuncMap{
	"Raw": func(sql string) Raw {
		return Raw(sql)
	},
	"Expr": func(sql string, args ...any) Expression {
		return Expression{
			SQL:  sql,
			Args: args,
		}
	},
	"Dest": func() any {
		return nil
	},
	"Scanner": func(dest sql.Scanner, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"ByteSlice": func(dest *[]byte, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"String": func(dest *string, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Int16": func(dest *int16, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Int32": func(dest *int32, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Int64": func(dest *int64, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Float32": func(dest *float32, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Float64": func(dest *float64, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Bool": func(dest *bool, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"Time": func(dest *time.Time, sql string, args ...any) Scanner {
		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: dest,
		}
	},
	"ParseTime": func(layout string, dest *time.Time, sql string, args ...any) Scanner {
		var data string

		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: &data,
			Map: func() error {
				v, err := time.Parse(layout, data)
				if err != nil {
					return err
				}

				*dest = v

				return nil
			},
		}
	},
	"JsonRaw": func(dest *json.RawMessage, sql string, args ...any) Scanner {
		var data []byte

		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: &data,
			Map: func() error {
				return json.Unmarshal(data, dest)
			},
		}
	},
	"JsonMap": func(dest *map[string]any, sql string, args ...any) Scanner {
		var data []byte

		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: &data,
			Map: func() error {
				var m map[string]any

				if err := json.Unmarshal(data, &m); err != nil {
					return err
				}

				*dest = m

				return nil
			},
		}
	},
	"SplitString": func(sep string, dest *[]string, sql string, args ...any) Scanner {
		var data string

		return Scanner{
			SQL:  sql,
			Args: args,
			Dest: &data,
			Map: func() error {
				*dest = strings.Split(sep, data)

				return nil
			},
		}
	},
}

func Dest[Dest any](t *Template) *DestTemplate[Dest] {
	return &DestTemplate[Dest]{
		Template: t,
	}
}

func MustDest[Dest any](t *Template, err error) *DestTemplate[Dest] {
	if err != nil {
		panic(err)
	}

	return &DestTemplate[Dest]{
		Template: t,
	}
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

func New(name string, placeholder string, positional bool) *Template {
	return &Template{
		text:        template.New(name).Funcs(DefaultFuncs),
		placeholder: placeholder,
		positional:  positional,
	}
}

func ParseFS(placeholder string, positional bool, fsys fs.FS, patterns ...string) (*Template, error) {
	text, err := template.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        escape(text.Funcs(DefaultFuncs)),
		placeholder: placeholder,
		positional:  positional,
	}, nil
}

func ParseFiles(placeholder string, positional bool, filenames ...string) (*Template, error) {
	text, err := template.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        escape(text.Funcs(DefaultFuncs)),
		placeholder: placeholder,
		positional:  positional,
	}, nil
}

type Template struct {
	text        *template.Template
	placeholder string
	positional  bool
}

func (t *Template) New(name string) *Template {
	return &Template{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)

	return t
}

func (t *Template) Parse(sql string) (*Template, error) {
	text, err := t.text.Parse(sql)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	text, err := t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	text, err := t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template) Clone() (*Template, error) {
	text, err := t.text.Clone()
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template) Delims(left, right string) *Template {
	t.text.Delims(left, right)

	return t
}

func (t *Template) Funcs(fm template.FuncMap) *Template {
	t.text.Funcs(fm)

	t.text.Clone()

	return t
}

func (t *Template) Lookup(name string) *Template {
	text := t.text.Lookup(name)
	if text == nil {
		return nil
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func (t *Template) ToExec(params any) Exec {
	var (
		buf  strings.Builder
		args []any
	)

	t.text.Funcs(template.FuncMap{
		"Dest": func() any {
			return map[string]any{}
		},
		ident: func(arg any) (string, error) {
			if s, ok := arg.(Scanner); ok {
				arg = Expression{
					SQL:  s.SQL,
					Args: s.Args,
				}
			}

			switch a := arg.(type) {
			case Raw:
				return string(a), nil
			case Expression:
				for {
					index := strings.IndexByte(a.SQL, '?')
					if index < 0 {
						buf.WriteString(a.SQL)

						return "", nil
					}

					if index < len(a.SQL)-1 && a.SQL[index+1] == '?' {
						buf.WriteString(a.SQL[:index+1])
						a.SQL = a.SQL[index+2:]

						continue
					}

					if len(a.Args) == 0 {
						return "", errors.New("invalid numer of arguments")
					}

					buf.WriteString(a.SQL[:index])
					args = append(args, a.Args[0])

					if t.positional {
						buf.WriteString(fmt.Sprintf("%s%d", t.placeholder, len(args)))
					} else {
						buf.WriteString(t.placeholder)
					}

					a.Args = a.Args[1:]
					a.SQL = a.SQL[index+1:]
				}
			}

			args = append(args, arg)

			if t.positional {
				return fmt.Sprintf("%s%d", t.placeholder, len(args)), nil
			}

			return t.placeholder, nil
		},
	})

	if err := t.text.Execute(&buf, params); err != nil {
		return Exec{
			Err: err,
		}
	}

	return Exec{
		SQL:  buf.String(),
		Args: args,
	}
}

func (t *Template) Exec(ctx context.Context, db DB, params any) (sql.Result, error) {
	return t.ToExec(params).Exec(ctx, db)
}

func (t *Template) Query(ctx context.Context, db DB, params any) (*sql.Rows, error) {
	return t.ToExec(params).Query(ctx, db)
}

func (t *Template) QueryRow(ctx context.Context, db DB, params any) (*sql.Row, error) {
	return t.ToExec(params).QueryRow(ctx, db)
}

type Exec struct {
	SQL  string
	Args []any
	Err  error
}

func (e Exec) Exec(ctx context.Context, db DB) (sql.Result, error) {
	if e.Err != nil {
		return nil, e.Err
	}

	return db.ExecContext(ctx, e.SQL, e.Args...)
}

func (e Exec) Query(ctx context.Context, db DB) (*sql.Rows, error) {
	if e.Err != nil {
		return nil, e.Err
	}

	return db.QueryContext(ctx, e.SQL, e.Args...)
}

func (e Exec) QueryRow(ctx context.Context, db DB) (*sql.Row, error) {
	if e.Err != nil {
		return nil, e.Err
	}

	return db.QueryRowContext(ctx, e.SQL, e.Args...), nil
}

type DestTemplate[Dest any] struct {
	Template *Template
}

func (t *DestTemplate[Dest]) ToQuery(params any) Query[Dest] {
	var (
		buf    strings.Builder
		args   []any
		dest   []any
		mapper []func() error
		value  = new(Dest)
	)

	t.Template.text.Funcs(template.FuncMap{
		"Dest": func() any {
			return value
		},
		ident: func(arg any) (string, error) {
			if s, ok := arg.(Scanner); ok {
				dest = append(dest, s.Dest)
				mapper = append(mapper, s.Map)

				arg = Expression{
					SQL:  s.SQL,
					Args: s.Args,
				}
			}

			switch a := arg.(type) {
			case Raw:
				return string(a), nil
			case Expression:
				for {
					index := strings.IndexByte(a.SQL, '?')
					if index < 0 {
						buf.WriteString(a.SQL)

						return "", nil
					}

					if index < len(a.SQL)-1 && a.SQL[index+1] == '?' {
						buf.WriteString(a.SQL[:index+1])
						a.SQL = a.SQL[index+2:]

						continue
					}

					if len(a.Args) == 0 {
						return "", errors.New("invalid numer of arguments")
					}

					buf.WriteString(a.SQL[:index])
					args = append(args, a.Args[0])

					if t.Template.positional {
						buf.WriteString(fmt.Sprintf("%s%d", t.Template.placeholder, len(args)))
					} else {
						buf.WriteString(t.Template.placeholder)
					}

					a.Args = a.Args[1:]
					a.SQL = a.SQL[index+1:]
				}
			}

			args = append(args, arg)

			if t.Template.positional {
				return fmt.Sprintf("%s%d", t.Template.placeholder, len(args)), nil
			}

			return t.Template.placeholder, nil
		},
	})

	if err := t.Template.text.Execute(&buf, params); err != nil {
		return Query[Dest]{
			Err: err,
		}
	}

	return Query[Dest]{
		SQL:   buf.String(),
		Args:  args,
		Value: value,
		Dest:  dest,
		Map:   mapper,
	}
}

func (t *DestTemplate[Dest]) QueryAll(ctx context.Context, db DB, params any) ([]Dest, error) {
	return t.ToQuery(params).QueryAll(ctx, db)
}

func (t *DestTemplate[Dest]) QueryFirst(ctx context.Context, db DB, params any) (Dest, error) {
	return t.ToQuery(params).QueryFirst(ctx, db)
}

type Query[Dest any] struct {
	SQL   string
	Args  []any
	Err   error
	Value *Dest
	Dest  []any
	Map   []func() error
}

func (q Query[Dest]) QueryAll(ctx context.Context, db DB) ([]Dest, error) {
	if q.Err != nil {
		return nil, q.Err
	}

	var (
		values []Dest
		err    error
	)

	rows, err := db.QueryContext(ctx, q.SQL, q.Args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(q.Dest...); err != nil {
			return nil, err
		}

		for _, m := range q.Map {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return nil, err
			}
		}

		values = append(values, *q.Value)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return values, nil
}

func (q Query[Dest]) QueryFirst(ctx context.Context, db DB) (Dest, error) {
	var (
		err   error
		value = new(Dest)
	)

	if q.Err != nil {
		return *q.Value, q.Err
	}

	row := db.QueryRowContext(ctx, q.SQL, q.Args...)

	if err = row.Scan(q.Dest...); err != nil {
		return *value, err
	}

	for _, m := range q.Map {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return *value, err
		}
	}

	return *value, nil
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
