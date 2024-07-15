package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"io/fs"
	"reflect"
	"strings"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
)

type Raw string

type Scanner struct {
	SQL  string
	Dest any
	Map  func() error
}

type Value[T any] struct {
	any
}

func (v Value[T]) Get() (T, bool) {
	switch t := v.any.(type) {
	case T:
		return t, true
	default:
		return *new(T), false
	}
}

func (v *Value[T]) Scan(value any) error {
	n := new(sql.Null[T])

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

func (v Value[T]) Value() (driver.Value, error) {
	return v.any, nil
}

func (v *Value[T]) UnmarshalJSON(data []byte) error {
	t := new(T)

	if err := json.Unmarshal(data, t); err != nil {
		v.any = nil

		return err
	}

	v.any = t

	return nil
}

func (v Value[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.any)
}

func (v Value[T]) String() string {
	return fmt.Sprintf("%v", v.any)
}

type namespace struct{}

func (namespace) Raw(str string) Raw {
	return Raw(str)
}

func (namespace) Scanner(dest sql.Scanner, str string) (Scanner, error) {
	if dest == nil || reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.Scanner at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) JSON(dest json.Unmarshaler, str string) (Scanner, error) {
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
}

func (ns namespace) ByteSlice(dest *[]byte, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.ByteSlice at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) String(dest *string, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.String at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int16(dest *int16, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int16 at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int32(dest *int32, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int32 at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Int64(dest *int64, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int64 at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Float32(dest *float32, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float32 at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Float64(dest *float64, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float64 at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Bool(dest *bool, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Bool at '%s'", str)
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

func (namespace) Time(dest *time.Time, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Time at '%s'", str)
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

func New(name string, placeholder string, positional bool) *Template {
	return &Template{
		text: template.New(name).Funcs(template.FuncMap{
			"Dest": func() any {
				return nil
			},
			"sqlt": func() any {
				return namespace{}
			},
		}),
		placeholder: placeholder,
		positional:  positional,
	}
}

type Template struct {
	text        *template.Template
	placeholder string
	positional  bool
	poolMap     sync.Map
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

type Runner[T any] struct {
	Text  *template.Template
	SQL   *bytes.Buffer
	Args  []any
	Value *T
	Dest  []any
	Map   []func() error
}

func (r *Runner[T]) Reset() {
	r.SQL.Reset()
	r.Args = r.Args[:0]
	r.Dest = r.Dest[:0]
	r.Map = r.Map[:0]
}

func Run[T any](t *Template, use func(*Runner[T])) {
	pool, _ := t.poolMap.LoadOrStore(reflect.TypeFor[T]().Name(), &sync.Pool{
		New: func() any {
			text, err := t.text.Clone()
			if err != nil {
				panic(err)
			}

			r := &Runner[T]{
				Text:  escape(text),
				SQL:   &bytes.Buffer{},
				Value: new(T),
			}

			r.Text.Funcs(template.FuncMap{
				"Dest": func() any {
					return r.Value
				},
				ident: func(arg any) string {
					if s, ok := arg.(Scanner); ok {
						r.Dest = append(r.Dest, s.Dest)
						r.Map = append(r.Map, s.Map)

						return s.SQL
					}

					switch a := arg.(type) {
					case Raw:
						return string(a)
					}

					r.Args = append(r.Args, arg)

					if t.positional {
						return fmt.Sprintf("%s%d", t.placeholder, len(r.Args))
					}

					return t.placeholder
				},
			})

			return r
		},
	})

	p := pool.(*sync.Pool)

	r := p.Get().(*Runner[T])

	r.Reset()

	use(r)

	p.Put(r)
}

func Exec(ctx context.Context, db *sql.DB, t *Template, params any) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)

	Run(t, func(r *Runner[any]) {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return
		}

		result, err = db.ExecContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)
		}
	})

	return result, err
}

func getTypes(list []any) []reflect.Type {
	types := make([]reflect.Type, len(list))

	for i, a := range list {
		types[i] = reflect.TypeOf(a)
	}

	return types
}

func makeErr(err error, sql *bytes.Buffer, args []any, dest []any, mappers []func() error) Error {
	mapperExist := make([]bool, len(mappers))
	for i, m := range mappers {
		mapperExist[i] = m != nil
	}

	return Error{
		Err:  err,
		SQL:  sql.String(),
		Args: getTypes(args),
		Dest: getTypes(dest),
		Map:  mapperExist,
	}
}

type Error struct {
	Err  error
	SQL  string
	Args []reflect.Type
	Dest []reflect.Type
	Map  []bool
}

func (e Error) Unwrap() error {
	return e.Err
}

func (e Error) Error() string {
	return fmt.Sprintf("sql: %s; args: %v; dest: %v; map: %v; err: %s", strings.TrimSuffix(strings.Join(strings.Fields(e.SQL), " "), ","), e.Args, e.Dest, e.Map, e.Err)
}

func All[T any](ctx context.Context, db *sql.DB, t *Template, params any) ([]T, error) {
	var (
		values []T
		err    error
	)

	Run(t, func(r *Runner[T]) {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return
		}

		if len(r.Dest) == 0 {
			r.Dest = []any{r.Value}
		}

		var rows *sql.Rows

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

			return
		}

		defer rows.Close()

		for rows.Next() {
			if err = rows.Scan(r.Dest...); err != nil {
				err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

				return
			}

			for _, m := range r.Map {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

					return
				}
			}

			values = append(values, *r.Value)
		}

		if err = rows.Err(); err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

			return
		}

		if err = rows.Close(); err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

			return
		}
	})

	return values, err
}

func First[T any](ctx context.Context, db *sql.DB, t *Template, params any) (T, error) {
	var (
		value T
		err   error
	)

	Run(t, func(r *Runner[T]) {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return
		}

		if len(r.Dest) == 0 {
			r.Dest = []any{r.Value}
		}

		row := db.QueryRowContext(ctx, r.SQL.String(), r.Args...)
		if err = row.Err(); err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

			return
		}

		if err = row.Scan(r.Dest...); err != nil {
			err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

			return
		}

		for _, m := range r.Map {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				err = makeErr(err, r.SQL, r.Args, r.Dest, r.Map)

				return
			}
		}

		value = *r.Value
	})

	return value, err
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
