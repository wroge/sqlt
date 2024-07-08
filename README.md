# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible and powerful SQL builder and ORM.

```go
go get github.com/wroge/sqlt@latest
```

## How does it work?

- All input values are [escaped](https://github.com/wroge/sqlt/blob/main/escape.go) and [replaced](https://github.com/wroge/sqlt/blob/main/run.go) at execution time with the correct placeholders.
- Functions like 'sqlt.Int64' create 'sqlt.Scanner`s' that hold pointers to the destination and optionally a mapper.
- These 'sqlt.Scanner`s' are collected at execution time.
- The 'Dest' function is a stub, that gets replaced at execution time with the generic type.
- This package aims to provide the functionalities of the 'text/template' package.
- SQL templates can be loaded from the filesystem using ParseFS or ParseFiles.
- All predefined 'sqlt' functions can be found [here](https://github.com/wroge/sqlt/blob/main/namespace.go).

## Example

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/sprig/v3"
	_ "github.com/mattn/go-sqlite3"
	"github.com/wroge/sqlt"
)

type Book struct {
	ID        int64
	Title     string
	CreatedAt time.Time
}

var (
	t = sqlt.New("db", "?", false).Funcs(sprig.TxtFuncMap())

	insert = t.New("insert").MustParse(`
		INSERT INTO books (title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			({{ $t }}, {{ now }})
		{{ end }}
		RETURNING id;
	`)

	query = t.New("query").MustParse(`
		SELECT
			{{ sqlt.Int64 Dest.ID "id" }}
			{{ sqlt.String Dest.Title ", title" }}
			{{ sqlt.Time Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE instr(title, {{ .Search }}) > 0
	`)
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite3", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		panic(err)
	}

	_, err = db.Exec("CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT, created_at DATE)")
	if err != nil {
		panic(err)
	}

	ids, err := sqlt.QueryAll[int64](ctx, db, insert, []string{
		"The Bitcoin Standard",
		"Sapiens: A Brief History of Humankind",
		"100 Go Mistakes and How to Avoid Them",
		"Mastering Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// INSERT INTO books (title, created_at) VALUES (?, ?) , (?, ?) , (?, ?) , (?, ?) RETURNING id;

	fmt.Println(ids)
	// [1 2 3 4]

	books, err := sqlt.QueryAll[Book](ctx, db, query, map[string]any{
		"Search": "Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// SELECT id, title, created_at FROM books WHERE instr(title, ?) > 0

	fmt.Println(books)
	// [{1 The Bitcoin Standard 2024-07-06 17:29:43.375399 +0200 +0200} {4 Mastering Bitcoin 2024-07-06 17:29:43.37544 +0200 +0200}]
}
```

## Example API using huma and sqlt

[https://github.com/wroge/vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture)

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
