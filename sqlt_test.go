package sqlt_test

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/wroge/sqlt"
)

func TestOne(t *testing.T) {
	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherEqual))
	if err != nil {
		t.Fatal(err)
	}

	mock.ExpectQuery("SELECT id, title FROM books WHERE title = ?").WithArgs("TEST").WillReturnRows(
		sqlmock.NewRows([]string{"id", "title"}).
			AddRow(1, "TEST"),
	)

	type Param struct {
		Title string
	}

	type Book struct {
		ID    int64
		Title string
	}

	stmt := sqlt.QueryStmt[Param, Book](
		sqlt.Parse(`
			SELECT
				{{ ScanInt64 Dest.ID "id" }}
				{{- ScanString Dest.Title ", title" }}
			FROM books WHERE title = {{ .Title }}
		`),
	)

	book, err := stmt.One(context.Background(), db, Param{Title: "TEST"})
	if err != nil {
		t.Fatal(err)
	}

	if book.ID != 1 || book.Title != "TEST" {
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
