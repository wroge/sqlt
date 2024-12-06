package sqlt_test

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"text/template"

	"github.com/DATA-DOG/go-sqlmock"
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

func TestOneError(t *testing.T) {
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

	_, err = stmt.One(context.Background(), db, Param{Title: "TEST"})
	if err == nil || err.Error() != "ERROR" {
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

	mock.ExpectQuery("INSERT INTO books (title, json) VALUES ($1, $2) , ($3, $4) RETURNING id;").WithArgs("TEST", json.RawMessage(`"data"`), "TEST 2", json.RawMessage(`"data 2"`)).WillReturnRows(
		sqlmock.NewRows([]string{"id"}).AddRow(1).AddRow(2),
	)

	type Book struct {
		ID    int64
		Title string
		JSON  json.RawMessage
	}

	config := sqlt.Config{
		Placeholder: sqlt.Dollar(),
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
