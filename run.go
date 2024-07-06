package sqlt

import (
	"bytes"
	"context"
	"database/sql"
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

type Raw string

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
	)

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
					runner.Args = append(runner.Args, a.Args[0])

					if t.positional {
						buf.WriteString(fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)))
					} else {
						buf.WriteString(t.placeholder)
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
