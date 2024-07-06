package sqlt

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"io/fs"
	"text/template"
)

func QueryAll[Dest any](ctx context.Context, db DB, t *Template, params any) ([]Dest, error) {
	r, err := Run[Dest](t, params)
	if err != nil {
		return nil, err
	}

	return r.QueryAll(ctx, db)
}

func QueryFirst[Dest any](ctx context.Context, db DB, t *Template, params any) (Dest, error) {
	r, err := Run[Dest](t, params)
	if err != nil {
		return *new(Dest), err
	}

	return r.QueryFirst(ctx, db)
}

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

func (t *Template) MustParse(sql string) *Template {
	return Must(t.Parse(sql))
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

func (t *Template) MustParseFS(fsys fs.FS, patterns ...string) *Template {
	return Must(t.ParseFS(fsys, patterns...))
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

func (t *Template) MustParseFiles(filenames ...string) *Template {
	return Must(t.ParseFiles(filenames...))
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

func (t *Template) MustClone() *Template {
	return Must(t.Clone())
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

func (t *Template) Exec(ctx context.Context, db DB, params any) (stdsql.Result, error) {
	r, err := Run[any](t, params)
	if err != nil {
		return nil, err
	}

	return db.ExecContext(ctx, r.SQL, r.Args...)
}

func (t *Template) Query(ctx context.Context, db DB, params any) (*stdsql.Rows, error) {
	r, err := Run[any](t, params)
	if err != nil {
		return nil, err
	}

	return db.QueryContext(ctx, r.SQL, r.Args...)
}

func (t *Template) QueryRow(ctx context.Context, db DB, params any) (*stdsql.Row, error) {
	r, err := Run[any](t, params)
	if err != nil {
		return nil, err
	}

	return db.QueryRowContext(ctx, r.SQL, r.Args...), nil
}
