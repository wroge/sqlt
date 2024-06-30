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

func Dest[Dest, Src any](t *Template[Src]) *Template[Dest] {
	return &Template[Dest]{
		text:        t.text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func Must[Dest, Src any](t *Template[Src], err error) *Template[Dest] {
	if err != nil {
		panic(err)
	}

	return &Template[Dest]{
		text:        t.text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func New(name string, placeholder string, positional bool) *Template[any] {
	return &Template[any]{
		text:        template.New(name).Funcs(DefaultFuncs),
		placeholder: placeholder,
		positional:  positional,
	}
}

func ParseFS(placeholder string, positional bool, fsys fs.FS, patterns ...string) (*Template[any], error) {
	text, err := template.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template[any]{
		text:        escape(text.Funcs(DefaultFuncs)),
		placeholder: placeholder,
		positional:  positional,
	}, nil
}

func ParseFiles(placeholder string, positional bool, filenames ...string) (*Template[any], error) {
	text, err := template.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template[any]{
		text:        escape(text.Funcs(DefaultFuncs)),
		placeholder: placeholder,
		positional:  positional,
	}, nil
}

type Template[Dest any] struct {
	text        *template.Template
	placeholder string
	positional  bool
}

func (t *Template[Dest]) New(name string) *Template[Dest] {
	return &Template[Dest]{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func (t *Template[Dest]) Option(opt ...string) *Template[Dest] {
	t.text.Option(opt...)

	return t
}

func (t *Template[Dest]) Parse(sql string) (*Template[Dest], error) {
	text, err := t.text.Parse(sql)
	if err != nil {
		return nil, err
	}

	return &Template[Dest]{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template[Dest]) MustParse(sql string) *Template[Dest] {
	return Must[Dest](t.Parse(sql))
}

func (t *Template[Dest]) ParseFS(fsys fs.FS, patterns ...string) (*Template[Dest], error) {
	text, err := t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template[Dest]{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template[Dest]) ParseFiles(filenames ...string) (*Template[Dest], error) {
	text, err := t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template[Dest]{
		text:        escape(text),
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template[Dest]) Clone() (*Template[Dest], error) {
	text, err := t.text.Clone()
	if err != nil {
		return nil, err
	}

	return &Template[Dest]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template[Dest]) Delims(left, right string) *Template[Dest] {
	t.text.Delims(left, right)

	return t
}

func (t *Template[Dest]) Funcs(fm template.FuncMap) *Template[Dest] {
	t.text.Funcs(fm)

	t.text.Clone()

	return t
}

func (t *Template[Dest]) Lookup(name string) *Template[Dest] {
	text := t.text.Lookup(name)
	if text == nil {
		return nil
	}

	return &Template[Dest]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func (t *Template[Dest]) Run(params any) Runner[Dest] {
	var runner = Runner[Dest]{
		SQL:   strings.Builder{},
		Value: new(Dest),
	}

	t.text.Funcs(template.FuncMap{
		"Dest": func() any {
			return runner.Value
		},
		ident: func(arg any) (string, error) {
			if s, ok := arg.(Scanner); ok {
				runner.Dest = append(runner.Dest, s.Dest)
				runner.Map = append(runner.Map, s.Map)

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
						runner.SQL.WriteString(a.SQL)

						return "", nil
					}

					if index < len(a.SQL)-1 && a.SQL[index+1] == '?' {
						runner.SQL.WriteString(a.SQL[:index+1])
						a.SQL = a.SQL[index+2:]

						continue
					}

					if len(a.Args) == 0 {
						return "", errors.New("invalid numer of arguments")
					}

					runner.SQL.WriteString(a.SQL[:index])
					runner.Args = append(runner.Args, a.Args[0])

					if t.positional {
						runner.SQL.WriteString(fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)))
					} else {
						runner.SQL.WriteString(t.placeholder)
					}

					a.Args = a.Args[1:]
					a.SQL = a.SQL[index+1:]
				}
			}

			runner.Args = append(runner.Args, arg)

			if t.positional {
				return fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)), nil
			}

			return t.placeholder, nil
		},
	})

	if runner.Err = t.text.Execute(&runner.SQL, params); runner.Err != nil {
		return runner
	}

	return runner
}

func (t *Template[Dest]) Exec(ctx context.Context, db DB, params any) (sql.Result, error) {
	return t.Run(params).Exec(ctx, db)
}

func (t *Template[Dest]) Query(ctx context.Context, db DB, params any) (*sql.Rows, error) {
	return t.Run(params).Query(ctx, db)
}

func (t *Template[Dest]) QueryRow(ctx context.Context, db DB, params any) (*sql.Row, error) {
	return t.Run(params).QueryRow(ctx, db)
}

func (t *Template[Dest]) QueryAll(ctx context.Context, db DB, params any) ([]Dest, error) {
	return t.Run(params).QueryAll(ctx, db)
}

func (t *Template[Dest]) QueryFirst(ctx context.Context, db DB, params any) (Dest, error) {
	return t.Run(params).QueryFirst(ctx, db)
}

type Runner[Dest any] struct {
	SQL   strings.Builder
	Args  []any
	Err   error
	Value *Dest
	Dest  []any
	Map   []func() error
}

func (r Runner[Dest]) Exec(ctx context.Context, db DB) (sql.Result, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	return db.ExecContext(ctx, r.SQL.String(), r.Args...)
}

func (r Runner[Dest]) Query(ctx context.Context, db DB) (*sql.Rows, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	return db.QueryContext(ctx, r.SQL.String(), r.Args...)
}

func (r Runner[Dest]) QueryRow(ctx context.Context, db DB) (*sql.Row, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	return db.QueryRowContext(ctx, r.SQL.String(), r.Args...), nil
}

func (r Runner[Dest]) QueryAll(ctx context.Context, db DB) ([]Dest, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	var (
		values []Dest
		err    error
	)

	rows, err := db.QueryContext(ctx, r.SQL.String(), r.Args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(r.Dest...); err != nil {
			return nil, err
		}

		for _, m := range r.Map {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return nil, err
			}
		}

		values = append(values, *r.Value)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return values, nil
}

func (r Runner[Dest]) QueryFirst(ctx context.Context, db DB) (Dest, error) {
	if r.Err != nil {
		return *r.Value, r.Err
	}

	row := db.QueryRowContext(ctx, r.SQL.String(), r.Args...)

	if r.Err = row.Scan(r.Dest...); r.Err != nil {
		return *r.Value, r.Err
	}

	for _, m := range r.Map {
		if m == nil {
			continue
		}

		if r.Err = m(); r.Err != nil {
			return *r.Value, r.Err
		}
	}

	return *r.Value, nil
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
