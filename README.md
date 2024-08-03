# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible and powerful SQL builder and ORM.

```go
go get -u github.com/wroge/sqlt
```

## How does it work?

- All input values are safely escaped and replaced with the correct placeholders at execution time.
- Functions like ```ScanInt64``` generate ```sqlt.Scanner`s```, which hold pointers to the destination and optionally a mapper. These scanners are collected at execution time.
- The ```Dest``` function is a placeholder that is replaced at execution time with the appropriate generic type.
- SQL templates can be loaded from the filesystem using ```ParseFS``` or ```ParseFiles```.

## Example

```go
package main

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/uuid"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wroge/sqlt"
)

type Book struct {
	ID        uuid.UUID
	Title     string
	CreatedAt time.Time
}

var (
	t = sqlt.New("db").
		Dollar().
		Funcs(sprig.TxtFuncMap()).
		AfterRun(func(err error, name string, r *sqlt.Runner) error {
			if err != nil {
				// ignore sql.ErrNoRows
				if errors.Is(err, sql.ErrNoRows) {
					return nil
				}

				// apply error logging here
				fmt.Println(err, name, strings.Join(strings.Fields(r.SQL.String()), " "))

				return err
			}

			// apply normal logging here
			fmt.Println(name, strings.Join(strings.Fields(r.SQL.String()), " "))

			return err
		})

	insert = t.New("insert").MustParse(`
		INSERT INTO books (id, title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			({{ uuidv4 }}, {{ $t }}, {{ now }})
		{{ end }}
		RETURNING id;
	`)

	query = t.New("query").MustParse(`
		SELECT
			{{ Scan Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
			{{ ScanTime Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE INSTR(title, {{ .Search }}) > 0
	`)
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("CREATE TABLE books (id, title, created_at DATE)")
	if err != nil {
		panic(err)
	}

	_, err = insert.Exec(ctx, db, []string{
		"The Bitcoin Standard",
		"Sapiens: A Brief History of Humankind",
		"100 Go Mistakes and How to Avoid Them",
		"Mastering Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// insert INSERT INTO books (id, title, created_at) VALUES ($1, $2, $3) , ($4, $5, $6) , ($7, $8, $9) , ($10, $11, $12) RETURNING id;

	books, err := sqlt.FetchAll[Book](ctx, query, db, map[string]any{
		"Search": "Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// query SELECT id , title , created_at FROM books WHERE INSTR(title, $1) > 0

	fmt.Println(books)
	// [{6bde9829-324e-4b06-a39e-e76f62398b15 The Bitcoin Standard 2024-08-03 11:08:57.432134 +0200 +0200} {60eef5a3-401c-4df9-93ee-6ad00902d0a8 Mastering Bitcoin 2024-08-03 11:08:57.432155 +0200 +0200}]
}
```

## Example API using huma and sqlt

[https://github.com/wroge/vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture)

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
