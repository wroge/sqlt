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

func Exec(ctx context.Context, db DB, t *Template, params any) (sql.Result, error) {
	sql, args, err := t.ToSQL(params)
	if err != nil {
		return nil, err
	}

	return db.ExecContext(ctx, sql, args...)
}

func Query(ctx context.Context, db DB, t *Template, params any) (*sql.Rows, error) {
	sql, args, err := t.ToSQL(params)
	if err != nil {
		return nil, err
	}

	return db.QueryContext(ctx, sql, args...)
}

func QueryRow(ctx context.Context, db DB, t *Template, params any) (*sql.Row, error) {
	sql, args, err := t.ToSQL(params)
	if err != nil {
		return nil, err
	}

	return db.QueryRowContext(ctx, sql, args...), nil
}

func QueryAll[Dest any](ctx context.Context, db DB, t *Template, params any) ([]Dest, error) {
	var (
		dest   []any
		mapper []func() error
		value  = new(Dest)
		list   []Dest
	)

	sql, args, err := t.Funcs(template.FuncMap{
		"Dest": func() any {
			return value
		},
	}).ToSQL(params, func(arg any) any {
		switch a := arg.(type) {
		case Scanner:
			dest = append(dest, a.Dest)
			mapper = append(mapper, a.Map)
		}

		return arg
	})
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, sql, args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(dest...); err != nil {
			return nil, err
		}

		for _, m := range mapper {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return nil, err
			}
		}

		list = append(list, *value)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return list, nil
}

func QueryFirst[Dest any](ctx context.Context, db DB, t *Template, params any) (Dest, error) {
	var (
		dest   []any
		mapper []func() error
		value  = new(Dest)
	)

	sql, args, err := t.Funcs(template.FuncMap{
		"Dest": func() any {
			return value
		},
	}).ToSQL(params, func(arg any) any {
		switch a := arg.(type) {
		case Scanner:
			dest = append(dest, a.Dest)
			mapper = append(mapper, a.Map)
		}

		return arg
	})
	if err != nil {
		return *value, err
	}

	row := db.QueryRowContext(ctx, sql, args...)

	if err = row.Scan(dest...); err != nil {
		return *value, err
	}

	for _, m := range mapper {
		if m == nil {
			continue
		}

		if err = m(); err != nil {
			return *value, err
		}
	}

	return *value, nil
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

func (t *Template) ToSQL(params any, inject ...func(arg any) any) (string, []any, error) {
	var (
		buf  strings.Builder
		args []any
	)

	t.text.Funcs(template.FuncMap{
		ident: func(arg any) (string, error) {
			for _, inj := range inject {
				arg = inj(arg)
			}

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
		return "", nil, err
	}

	return buf.String(), args, nil
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
