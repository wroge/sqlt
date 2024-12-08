package sqlt_test

import (
	"context"
	"encoding/json"
	"errors"
	"io/fs"
	"testing"
	"text/template"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/spf13/afero"
	"github.com/wroge/sqlt"
)

func TestOne(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title, json FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title", "json"}).
			AddRow(1, "TEST", json.RawMessage(`"data"`)),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	type Key struct{}

	config := sqlt.Config{
		Start: func(runner *sqlt.Runner) {
			runner.Context = context.WithValue(runner.Context, Key{}, "VALUE")
		},
		End: func(err error, runner *sqlt.Runner) {
			if v, ok := runner.Context.Value(Key{}).(string); !ok || v != "VALUE" {
				t.Fatal(v, ok)
			}

			if runner.SQL.String() != "SELECT id, title, json FROM books WHERE title = ?" {
				t.Fail()
			}
		},
		TemplateOptions: []sqlt.TemplateOption{
			sqlt.Funcs(template.FuncMap{
				"ScanRawJSON": sqlt.ScanJSON[json.RawMessage],
			}),
		},
	}

	stmt := sqlt.QueryStmt[Param, Book](
		config,
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
				{{- ScanRawJSON Dest.JSON ", json" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	book, err := stmt.One(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if book.ID != 1 || book.Title != "TEST" || string(book.JSON) != `"data"` {
		t.Fail()
	}
}

func TestOneErrorInTx(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = @p1").WithArgs("TEST").WillReturnError(errors.New("ERROR"))

	mock.ExpectRollback()

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if err == nil || err.Error() != "ERROR" {
				t.Fail()
			}
		}),
		sqlt.AtP(),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	_ = sqlt.InTx(context.Background(), nil, db, func(db sqlt.DB) error {
		_, err = stmt.One(context.Background(), db, Param{Title: "TEST"})
		if err == nil || err.Error() != "ERROR" {
			t.Fail()
		}

		return err
	})
}

func TestOneParseError(t *testing.T) {
	defer func() {
		if r := recover(); r == nil {
			t.Errorf("The code did not panic")
		}
	}()

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	_ = sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if err == nil || err.Error() != "ERROR" {
				t.Fail()
			}
		}),
		sqlt.New("query"),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Titel ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)
}

func TestOneLookup(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title, json FROM books WHERE title = :1").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title", "json"}).
			AddRow(1, "TEST", json.RawMessage(`"data"`)),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	type Key struct{}

	config := sqlt.Config{
		Placeholder: sqlt.Colon(),
		Start: func(runner *sqlt.Runner) {
			runner.Context = context.WithValue(runner.Context, Key{}, "VALUE")
		},
		End: func(err error, runner *sqlt.Runner) {
			if v, ok := runner.Context.Value(Key{}).(string); !ok || v != "VALUE" {
				t.Fatal(v, ok)
			}

			if runner.SQL.String() != "SELECT id, title, json FROM books WHERE title = :1" {
				t.Fail()
			}
		},
		TemplateOptions: []sqlt.TemplateOption{
			sqlt.Funcs(template.FuncMap{
				"ScanRawJSON": sqlt.ScanJSON[json.RawMessage],
			}),
		},
	}

	stmt := sqlt.QueryStmt[Param, Book](
		config,
		sqlt.Parse(`
			{{ define "query" }}
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
				{{- ScanRawJSON Dest.JSON ", json" }}
			FROM books WHERE title = {{ .Title }}
			{{ end }}
		`),
		sqlt.Lookup("query"),
	)

	book, err := stmt.One(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if book.ID != 1 || book.Title != "TEST" || string(book.JSON) != `"data"` {
		t.Fail()
	}
}

type FS struct {
	FS afero.Fs
}

// Open implements fs.FS.
func (f FS) Open(name string) (fs.File, error) {
	return f.FS.Open(name)
}

func TestOneFile(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title, json FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title", "json"}).
			AddRow(1, "TEST", json.RawMessage(`"data"`)),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	fs := afero.NewOsFs()

	file, err := fs.Create("/tmp/query.sql")
	if err != nil {
		t.Fatal(err)
	}

	defer file.Close()

	_, err = file.Write([]byte(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{- ScanString Dest.Title ", title" }}
			{{- ScanRawJSON Dest.JSON ", json" }}
		FROM books WHERE title = {{ .Title }}
	`))
	if err != nil {
		t.Fatal(err)
	}

	type Key struct{}

	config := sqlt.Config{
		Start: func(runner *sqlt.Runner) {
			runner.Context = context.WithValue(runner.Context, Key{}, "VALUE")
		},
		End: func(err error, runner *sqlt.Runner) {
			if v, ok := runner.Context.Value(Key{}).(string); !ok || v != "VALUE" {
				t.Fatal(v, ok)
			}

			if runner.SQL.String() != "SELECT id, title, json FROM books WHERE title = ?" {
				t.Fail()
			}
		},
		TemplateOptions: []sqlt.TemplateOption{
			sqlt.Funcs(template.FuncMap{
				"ScanRawJSON": sqlt.ScanJSON[json.RawMessage],
			}),
		},
	}

	stmt := sqlt.QueryStmt[Param, Book](
		config,
		sqlt.ParseFS(FS{FS: fs}, "/tmp/query.sql"),
		sqlt.Lookup("query.sql"),
	)

	book, err := stmt.One(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if book.ID != 1 || book.Title != "TEST" || string(book.JSON) != `"data"` {
		t.Fail()
	}
}

func TestFirst(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).
			AddRow(1, "TEST").
			AddRow(2, "TEST 2"),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if runner.SQL.String() != "SELECT id, title FROM books WHERE title = ?" {
				t.Fail()
			}
		}),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	book, err := stmt.First(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if book.ID != 1 || book.Title != "TEST" {
		t.Fail()
	}
}

func TestFirstError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = ?").WithArgs("TEST").WillReturnError(errors.New("ERROR"))

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if err == nil || err.Error() != "ERROR" {
				t.Fail()
			}
		}),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	_, err = stmt.First(context.Background(), db, Param{Title: "TEST"})
	if err == nil || err.Error() != "ERROR" {
		t.Fail()
	}
}

func TestAll(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).
			AddRow(1, "TEST").
			AddRow(2, "TEST 2"),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if runner.SQL.String() != "SELECT id, title FROM books WHERE title = ?" {
				t.Fail()
			}
		}),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	books, err := stmt.All(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if len(books) != 2 {
		t.Fail()
	}
}

func TestAllError(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = ?").WithArgs("TEST").WillReturnError(errors.New("ERROR"))

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.End(func(err error, runner *sqlt.Runner) {
			if err == nil || err.Error() != "ERROR" {
				t.Fail()
			}
		}),
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	_, err = stmt.All(context.Background(), db, Param{Title: "TEST"})
	if err == nil || err.Error() != "ERROR" {
		t.Fail()
	}
}

func TestExec(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectExec("INSERT INTO books (id, title, json) VALUES ($1, $2, $3);").WithArgs(1, "TEST", json.RawMessage(`"data"`)).WillReturnResult(
		sqlmock.NewResult(1, 1),
	)

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	config := sqlt.Config{
		Placeholder: sqlt.Dollar(),
		TemplateOptions: []sqlt.TemplateOption{
			sqlt.MissingKeyZero(),
		},
	}

	stmt := sqlt.Stmt[Book](
		config,
		sqlt.Parse(`
			INSERT INTO books (id, title, json) VALUES ({{ .ID }}, {{ .Title }}, {{ .JSON }});
		`),
	)

	result, err := stmt.Exec(context.Background(), db, Book{ID: 1, Title: "TEST", JSON: json.RawMessage(`"data"`)})
	if err != nil {
		t.Fatal(err)
	}

	if aff, err := result.RowsAffected(); aff != 1 || err != nil {
		t.Fail()
	}
}

func TestQueryRow(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("INSERT INTO books (title, json) VALUES ($1, $2) RETURNING id;").WithArgs("TEST", json.RawMessage(`"data"`)).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1),
	)

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	config := sqlt.Config{
		Placeholder: sqlt.Dollar(),
	}

	stmt := sqlt.Stmt[Book](
		config,
		sqlt.MissingKeyInvalid(),
		sqlt.Parse(`
			INSERT INTO books (title, json) VALUES ({{ .Title }}, {{ .JSON }}) RETURNING id;
		`),
	)

	row, err := stmt.QueryRow(context.Background(), db, Book{Title: "TEST", JSON: json.RawMessage(`"data"`)})
	if err != nil {
		t.Fatal(err)
	}

	var id int64

	if err = row.Scan(&id); err != nil {
		t.Fatal(err)
	}

	if id != 1 {
		t.Fail()
	}
}

func TestQuery(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("INSERT INTO books (title, json) VALUES (?, ?) , (?, ?) RETURNING id;").WithArgs("TEST", json.RawMessage(`"data"`), "TEST 2", json.RawMessage(`"data 2"`)).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2),
	)

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	config := sqlt.Config{
		Placeholder: sqlt.Question(),
		TemplateOptions: []sqlt.TemplateOption{
			sqlt.MissingKeyError(),
		},
	}

	stmt := sqlt.Stmt[[]Book](
		config,
		sqlt.Parse(`
			INSERT INTO books (title, json) VALUES
			{{ range $i, $v := . -}}
				{{- if $i }}, {{ end -}}
			 	({{ $v.Title }}, {{ $v.JSON }})
			{{ end }}
			RETURNING id;
		`),
	)

	rows, err := stmt.Query(context.Background(), db, []Book{
		{Title: "TEST", JSON: json.RawMessage(`"data"`)},
		{Title: "TEST 2", JSON: json.RawMessage(`"data 2"`)},
	})
	if err != nil {
		t.Fatal(err)
	}

	var ids []int64

	for rows.Next() {
		var id int64

		if err = rows.Scan(&id); err != nil {
			t.Fatal(err)
		}

		ids = append(ids, id)
	}

	if len(ids) != 2 {
		t.Fail()
	}
}

func TestOneSingleColumnInTx(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectBegin()

	mock.ExpectQuery("SELECT id FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id"}).
			AddRow(1),
	)

	mock.ExpectCommit()

	type Param struct {
		Title string
	}

	type Key struct{}

	config := sqlt.Config{
		Start: func(runner *sqlt.Runner) {
			runner.Context = context.WithValue(runner.Context, Key{}, "VALUE")
		},
		End: func(err error, runner *sqlt.Runner) {
			if v, ok := runner.Context.Value(Key{}).(string); !ok || v != "VALUE" {
				t.Fatal(v, ok)
			}

			if runner.SQL.String() != "SELECT id FROM books WHERE title = ?" {
				t.Fail()
			}
		},
	}

	stmt := sqlt.QueryStmt[Param, int64](
		config,
		sqlt.Question(),
		sqlt.Parse(`
			SELECT {{ Raw "id" }} FROM books WHERE title = {{ .Title }}
		`),
	)

	err = sqlt.InTx(context.Background(), nil, db, func(db sqlt.DB) error {
		id, err := stmt.One(context.Background(), db, Param{Title: "TEST"})
		if err != nil {
			return err
		}

		if id != 1 {
			t.Fail()
		}

		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
