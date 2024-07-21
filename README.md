# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible and powerful SQL builder and ORM.

```go
go get -u github.com/wroge/sqlt
```

## How does it work?

- All input values are safely escaped and replaced with the correct placeholders at execution time.
- Functions like ```sqlt.Int64``` generate ```sqlt.Scanner`s```, which hold pointers to the destination and optionally a mapper. These scanners are collected at execution time.
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
	t = sqlt.New("db").Dollar().Funcs(sprig.TxtFuncMap()).HandleErr(func(err sqlt.Error) error {
		if errors.Is(err.Err, sql.ErrNoRows) {
			return nil
		}

		// apply logging here
		fmt.Println(err.Template, err.SQL, err.Args)

		return err.Err
	})

	insert = t.New("insert").MustParse(`
		INSERT INTO books (id, title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			({{ uuidv4 }}, {{ $t }}, {{ now }})
		{{ end }}
		RETURNING id;
	`)

	query = sqlt.Dest[Book](t.New("query").MustParse(`
		SELECT
			{{ sqlt.Scanner Dest.ID "id" }}
			{{ sqlt.String Dest.Title ", title" }}
			{{ sqlt.Time Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE instr(title, {{ .Search }}) > 0
	`))
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
	// INSERT INTO books (title, created_at) VALUES (?, ?) , (?, ?) , (?, ?) , (?, ?) RETURNING id;

	books, err := query.QueryAll(ctx, db, map[string]any{
		"Search": "Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// SELECT id, title, created_at FROM books WHERE instr(title, ?) > 0

	fmt.Println(books)
	// [{76b87e40-0d47-429e-8a80-563e7bf0cf13 The Bitcoin Standard 2024-07-21 22:50:37.672297 +0200 +0200} {b63ec5c4-1fd9-448a-96c6-5abc54c05aa1 Mastering Bitcoin 2024-07-21 22:50:37.672306 +0200 +0200}]
}
```

## Example API using huma and sqlt

[https://github.com/wroge/vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture)

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
