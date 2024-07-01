package sqlt

import (
	"context"
	"database/sql"
	stdsql "database/sql"
	"fmt"
	"io/fs"
	"text/template"
)

type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*stdsql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *stdsql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (stdsql.Result, error)
}

func InTx(ctx context.Context, opts *stdsql.TxOptions, db *stdsql.DB, do func(db DB) error) error {
	var (
		tx  *stdsql.Tx
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

func Dest[Dest, A any](t *Template[A]) *Template[Dest] {
	return &Template[Dest]{
		text:        t.text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}
}

func Must[Dest, A any](t *Template[A], err error) *Template[Dest] {
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

func (t *Template[Dest]) MustParseFS(fsys fs.FS, patterns ...string) *Template[Dest] {
	return Must[Dest](t.ParseFS(fsys, patterns...))
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

func (t *Template[Dest]) MustParseFiles(filenames ...string) *Template[Dest] {
	return Must[Dest](t.ParseFiles(filenames...))
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

func (t *Template[Dest]) MustClone() *Template[Dest] {
	return Must[Dest](t.Clone())
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

func (t *Template[Dest]) Lookup(name string) (*Template[Dest], error) {
	text := t.text.Lookup(name)
	if text == nil {
		return nil, fmt.Errorf("template name '%s' not found", name)
	}

	return &Template[Dest]{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
	}, nil
}

func (t *Template[Dest]) MustLookup(name string) *Template[Dest] {
	return Must[Dest](t.Lookup(name))
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
	SQL   string
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

	return db.ExecContext(ctx, r.SQL, r.Args...)
}

func (r Runner[Dest]) Query(ctx context.Context, db DB) (*sql.Rows, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	return db.QueryContext(ctx, r.SQL, r.Args...)
}

func (r Runner[Dest]) QueryRow(ctx context.Context, db DB) (*sql.Row, error) {
	if r.Err != nil {
		return nil, r.Err
	}

	return db.QueryRowContext(ctx, r.SQL, r.Args...), nil
}

func (r Runner[Dest]) QueryAll(ctx context.Context, db DB) ([]Dest, error) {
	if r.Err != nil {
		return nil, r.Err
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

	row := db.QueryRowContext(ctx, r.SQL, r.Args...)

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
