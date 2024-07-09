package sqlt

import (
	"bytes"
	"database/sql"
	stdsql "database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"sync"
	"text/template"
)

type Raw string

type Scanner struct {
	SQL  string
	Dest any
	Map  func() error
}

type Null[V any] stdsql.Null[V]

func (n *Null[V]) Scan(value any) error {
	v := new(sql.Null[V])

	if err := v.Scan(value); err != nil {
		return err
	}

	return nil
}

func (n Null[V]) Value() (driver.Value, error) {
	return sql.Null[V]{
		V:     n.V,
		Valid: n.Valid,
	}.Value()
}

func (n *Null[V]) UnmarshalJSON(data []byte) error {
	if err := json.Unmarshal(data, &n.V); err != nil {
		n.Valid = false

		return nil
	}

	n.Valid = true

	return nil
}

func (n *Null[V]) MarshalJSON() ([]byte, error) {
	if !n.Valid {
		return []byte("null"), nil
	}

	return json.Marshal(n.V)
}

type JSON[V any] struct {
	Data V
}

func (v *JSON[V]) Scan(value any) error {
	switch t := value.(type) {
	case string:
		return v.UnmarshalJSON([]byte(t))
	case []byte:
		return v.UnmarshalJSON(t)
	}

	return errors.New("invalid scan value for json bytes")
}

func (v *JSON[V]) Value() (driver.Value, error) {
	return json.Marshal(v.Data)
}

func (v *JSON[V]) UnmarshalJSON(data []byte) error {
	var value V

	if err := json.Unmarshal(data, &value); err != nil {
		return err
	}

	v.Data = value

	return nil
}

func (v JSON[V]) MarshalJSON() ([]byte, error) {
	return json.Marshal(v.Data)
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

func (t *Template) Parse(sql string) (*Template, error) {
	text, err := t.text.Parse(sql)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template) MustParse(sql string) *Template {
	return Must(t.Parse(sql))
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
