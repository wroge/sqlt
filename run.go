package sqlt

import (
	"bytes"
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

type Scanner struct {
	SQL  string
	Args []any
	Dest any
	Map  func() error
}

type Null[V any] sql.Null[V]

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

type Expression struct {
	SQL  string
	Args []any
}

func Run[Dest any](t *Template, params any) (*Runner[Dest], error) {
	var (
		buf bytes.Buffer

		runner = &Runner[Dest]{
			Value: new(Dest),
		}

		unwrap func(expr Expression) error
	)

	unwrap = func(expr Expression) error {
		for {
			index := strings.IndexByte(expr.SQL, '?')
			if index < 0 {
				buf.WriteString(expr.SQL)

				return nil
			}

			if index < len(expr.SQL)-1 {
				if expr.SQL[index+1] == '?' {
					buf.WriteString(expr.SQL[:index+1])
					expr.SQL = expr.SQL[index+2:]

					continue
				}
			}

			if len(expr.Args) == 0 {
				return errors.New("invalid numer of arguments")
			}

			buf.WriteString(expr.SQL[:index])

			switch e := expr.Args[0].(type) {
			case Expression:
				if err := unwrap(e); err != nil {
					return err
				}
			default:
				runner.Args = append(runner.Args, expr.Args[0])

				if t.positional {
					buf.WriteString(fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)))
				} else {
					buf.WriteString(t.placeholder)
				}
			}

			expr.Args = expr.Args[1:]
			expr.SQL = expr.SQL[index+1:]
		}
	}

	t.text.Funcs(template.FuncMap{
		"Dest": func() any {
			return runner.Value
		},
		ident: func(arg any) (string, error) {
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

				arg = Expression{
					SQL:  s.SQL,
					Args: s.Args,
				}
			}

			switch a := arg.(type) {
			case Expression:
				return "", unwrap(a)
			}

			runner.Args = append(runner.Args, arg)

			if t.positional {
				return fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)), nil
			}

			return t.placeholder, nil
		},
	})

	if err := t.text.Execute(&buf, params); err != nil {
		return nil, err
	}

	runner.SQL = buf.String()

	return runner, nil
}

type Runner[Dest any] struct {
	SQL   string
	Args  []any
	Value *Dest
	Dest  []any
	Map   func() error
}

func (r *Runner[Dest]) QueryAll(ctx context.Context, db DB) ([]Dest, error) {
	if len(r.Dest) == 0 {
		r.Dest = []any{r.Value}
	}

	var (
		values []Dest
		err    error
	)

	rows, err := db.QueryContext(ctx, r.SQL, r.Args...)
	if err != nil {
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(r.Dest...); err != nil {
			return nil, err
		}

		if r.Map != nil {
			if err = r.Map(); err != nil {
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

var ErrTooManyRows = errors.New("sql: too many rows")

func (r *Runner[Dest]) QueryOne(ctx context.Context, db DB) (Dest, error) {
	if len(r.Dest) == 0 {
		r.Dest = []any{r.Value}
	}

	rows, err := db.QueryContext(ctx, r.SQL, r.Args...)
	if err != nil {
		return *r.Value, err
	}

	defer rows.Close()

	if !rows.Next() {
		return *r.Value, sql.ErrNoRows
	}

	if err = rows.Scan(r.Dest...); err != nil {
		return *r.Value, err
	}

	if r.Map != nil {
		if err = r.Map(); err != nil {
			return *r.Value, err
		}
	}

	if rows.Next() {
		return *r.Value, ErrTooManyRows
	}

	if err = rows.Err(); err != nil {
		return *r.Value, err
	}

	if err = rows.Close(); err != nil {
		return *r.Value, err
	}

	return *r.Value, nil
}

func (r *Runner[Dest]) QueryFirst(ctx context.Context, db DB) (Dest, error) {
	if len(r.Dest) == 0 {
		r.Dest = []any{r.Value}
	}

	row := db.QueryRowContext(ctx, r.SQL, r.Args...)

	if err := row.Scan(r.Dest...); err != nil {
		return *r.Value, err
	}

	if r.Map != nil {
		if err := r.Map(); err != nil {
			return *r.Value, err
		}
	}

	return *r.Value, nil
}
