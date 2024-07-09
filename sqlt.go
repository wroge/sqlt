package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"reflect"
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

func Exec(ctx context.Context, db DB, t *Template, params any) (sql.Result, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	result, err := db.ExecContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s args: %v err: %w", trimSQL(runner.SQL), runner.Args, err)
	}

	return result, nil
}

func Query(ctx context.Context, db DB, t *Template, params any) (*sql.Rows, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s args: %v err: %w", trimSQL(runner.SQL), runner.Args, err)
	}

	return rows, nil
}

func QueryRow(ctx context.Context, db DB, t *Template, params any) (*sql.Row, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, runner.SQL, runner.Args...)
	if err = row.Err(); err != nil {
		return nil, fmt.Errorf("sql: %s args: %v err: %w", trimSQL(runner.SQL), runner.Args, err)
	}

	return row, nil
}

func QueryAll[Dest any](ctx context.Context, db DB, t *Template, params any) ([]Dest, error) {
	var (
		values []Dest
		value  Dest
		err    error
	)

	runner, err := t.Run(params, &value)
	if err != nil {
		return nil, err
	}

	if len(runner.Dest) == 0 {
		runner.Dest = []any{&value}
	}

	rows, err := db.QueryContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s args: %v err: %w", trimSQL(runner.SQL), runner.Args, err)
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(runner.Dest...); err != nil {
			return nil, err
		}

		if runner.Map != nil {
			if err = runner.Map(); err != nil {
				return nil, err
			}
		}

		values = append(values, value)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return values, nil
}

func QueryFirst[Dest any](ctx context.Context, db DB, t *Template, params any) (Dest, error) {
	var value Dest

	runner, err := t.Run(params, &value)
	if err != nil {
		return value, err
	}

	if len(runner.Dest) == 0 {
		runner.Dest = []any{&value}
	}

	row := db.QueryRowContext(ctx, runner.SQL, runner.Args...)
	if err = row.Err(); err != nil {
		return value, fmt.Errorf("sql: %s args: %v err: %w", trimSQL(runner.SQL), runner.Args, err)
	}

	if err := row.Scan(runner.Dest...); err != nil {
		return value, err
	}

	if runner.Map != nil {
		if err := runner.Map(); err != nil {
			return value, err
		}
	}

	return value, nil
}

func trimSQL(str string) string {
	return strings.Join(strings.Fields(str), " ")
}

type Raw string

type Scanner struct {
	SQL  string
	Dest any
	Map  func() error
}

type Value[V any] struct {
	any
}

func (v Value[V]) Get() (V, bool) {
	switch t := v.any.(type) {
	case V:
		return t, true
	default:
		return *new(V), false
	}
}

func (v *Value[V]) Scan(value any) error {
	n := new(sql.Null[V])

	if err := n.Scan(value); err != nil {
		v.any = nil

		return err
	}

	if n.Valid {
		v.any = n.V
	} else {
		v.any = nil
	}

	return nil
}

func (v Value[V]) Value() (driver.Value, error) {
	return v.any, nil
}

func (v *Value[V]) UnmarshalJSON(data []byte) error {
	object := new(V)

	if err := json.Unmarshal(data, object); err != nil {
		return err
	}

	v.any = object

	return nil
}

func (v Value[V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.any)
}

func (v Value[V]) String() string {
	return fmt.Sprintf("%v", v.any)
}

type namespace struct{}

func (namespace) Raw(str string) Raw {
	return Raw(str)
}

func (namespace) Scanner(dest sql.Scanner, str string, args ...any) (Scanner, error) {
	if reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.Scanner at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) JSON(dest json.Unmarshaler, str string, args ...any) (Scanner, error) {
	if reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.JSON at '%s'; try to use sqlt.Value instead", str)
	}

	var data []byte

	return Scanner{
		SQL:  str,
		Dest: &data,
		Map: func() error {
			return json.Unmarshal(data, dest)
		},
	}, nil
}

func (ns namespace) ByteSlice(dest *[]byte, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.ByteSlice at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) String(dest *string, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.String at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int16(dest *int16, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int16 at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int32(dest *int32, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int32 at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int64(dest *int64, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int64 at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Float32(dest *float32, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float32 at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Float64(dest *float64, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float64 at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Bool(dest *bool, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Bool at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Time(dest *time.Time, str string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Time at '%s'; try to use sqlt.Value instead", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func New(name string, placeholder string, positional bool) *Template {
	return &Template{
		text: template.New(name).Funcs(template.FuncMap{
			"Dest": func() any {
				return map[string]any{}
			},
			"sqlt": func() any {
				return namespace{}
			},
		}),
		placeholder: placeholder,
		positional:  positional,
	}
}

func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

type Template struct {
	text        *template.Template
	placeholder string
	positional  bool
	pool        *sync.Pool
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

func (t *Template) Parse(str string) (*Template, error) {
	text, err := t.text.Parse(str)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
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
	}, nil
}

func (t *Template) MustParseFiles(filenames ...string) *Template {
	return Must(t.ParseFiles(filenames...))
}

type Runner struct {
	SQL  string
	Args []any
	Dest []any
	Map  func() error
}

var bufPool = &sync.Pool{
	New: func() any {
		return &bytes.Buffer{}
	},
}

func (t *Template) Run(params any, value any) (Runner, error) {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				return escape(text)
			},
		}
	}

	if value == nil {
		value = map[string]any{}
	}

	switch text := t.pool.Get().(type) {
	case *template.Template:
		var (
			runner Runner
			buf    = bufPool.Get().(*bytes.Buffer)
		)

		buf.Reset()

		text.Funcs(template.FuncMap{
			"Dest": func() any {
				return value
			},
			ident: func(arg any) string {
				if s, ok := arg.(Scanner); ok {
					runner.Dest = append(runner.Dest, s.Dest)

					if s.Map != nil {
						m := runner.Map

						runner.Map = func() error {
							if m != nil {
								if err := m(); err != nil {
									return err
								}
							}

							return s.Map()
						}
					}

					return s.SQL
				}

				switch a := arg.(type) {
				case Raw:
					return string(a)
				}

				runner.Args = append(runner.Args, arg)

				if t.positional {
					return fmt.Sprintf("%s%d", t.placeholder, len(runner.Args))
				}

				return t.placeholder
			},
		})

		if err := text.Execute(buf, params); err != nil {
			return Runner{}, err
		}

		runner.SQL = buf.String()

		bufPool.Put(buf)
		t.pool.Put(text)

		return runner, nil
	case error:
		return Runner{}, text
	default:
		return Runner{}, errors.New("sqlt: pooling error")
	}
}

func (t *Template) Delims(left, right string) *Template {
	t.text.Delims(left, right)

	return t
}

func (t *Template) Funcs(fm template.FuncMap) *Template {
	t.text.Funcs(fm)

	return t
}

func (t *Template) Value(name string, value any) *Template {
	t.text.Funcs(template.FuncMap{
		name: func() any {
			return value
		},
	})

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
	}, nil
}

func (t *Template) MustLookup(name string) *Template {
	return Must(t.Lookup(name))
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
