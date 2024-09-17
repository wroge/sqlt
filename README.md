# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible, powerful and type-safe SQL builder and ORM.

```go
go get -u github.com/wroge/sqlt
```

## How does it work?

- All input values are safely escaped and replaced with the correct placeholders at execution time.
- ```Scan```-functions generate ```sqlt.Scanner`s```, which hold pointers to the destination and optionally a mapper. These scanners are collected at execution time.
- The ```Dest``` function is a placeholder that is replaced at execution time with the appropriate generic type.
- SQL templates can be loaded from the filesystem using ```ParseFS``` or ```ParseFiles```.
- ```Type``` and ```MustType``` functions do type-safe checks using: [jba/templatecheck](https://github.com/jba/templatecheck).

## Example

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/google/uuid"
	"github.com/wroge/sqlt"
	_ "modernc.org/sqlite"
)

type startKey struct{}

type Book struct {
	ID        uuid.UUID
	Title     string
	CreatedAt time.Time
}

type Query struct {
	Title string
}

var (
	t = sqlt.New("db").
		Dollar().
		Funcs(sprig.TxtFuncMap()).
		BeforeRun(func(r *sqlt.Runner) {
			r.Context = context.WithValue(r.Context, startKey{}, time.Now())
		}).
		AfterRun(func(err error, r *sqlt.Runner) error {
			var duration = time.Since(r.Context.Value(startKey{}).(time.Time))

			if err != nil {
				// apply error logging here
				fmt.Println(err, r.Text.Name(), duration, r.SQL, r.Args)

				return err
			}

			// apply normal logging here
			fmt.Println(r.Text.Name(), duration, r.SQL, r.Args)

			return nil
		})

	insert = sqlt.MustType[any, []string](t.New("insert").MustParse(`
		INSERT INTO books (id, title, created_at) VALUES
		{{ range $i, $t := . -}}
			{{ if $i }}, {{ end }}
			({{ uuidv4 }}, {{ $t }}, {{ now }})
		{{- end }}
		RETURNING id;
	`))

	query = sqlt.MustType[Book, Query](t.New("query").MustParse(`
		SELECT
			{{ Scan Dest.ID "id" -}}
			{{ ScanString Dest.Title ", title" -}}
			{{ ScanTime Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE INSTR(title, {{ .Title }}) > 0;
	`))
)

func main() {
	ctx := context.Background()

	db, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=memory")
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
	// insert 262.084µs INSERT INTO books (id, title, created_at) VALUES ($1, $2, $3), ($4, $5, $6), ($7, $8, $9), ($10, $11, $12) RETURNING id;
	// [001fa71a-bb2e-46bd-a545-0ce12a8229c1 The Bitcoin Standard 2024-09-12 20:00:19.963652 +0200 CEST m=+0.009764167 e056bec8-609d-48fc-a769-c5ff8d06bd1e Sapiens: A Brief History of Humankind 2024-09-12 20:00:19.963658 +0200 CEST m=+0.009769626 86065b16-0aa1-43f6-b589-17fbdbddeffc 100 Go Mistakes and How to Avoid Them 2024-09-12 20:00:19.963661 +0200 CEST m=+0.009773292 b7905ecc-b023-482b-b5d2-68fc26fefc1c Mastering Bitcoin 2024-09-12 20:00:19.963665 +0200 CEST m=+0.009777001]

	books, err := query.All(ctx, db, Query{Title: "Bitcoin"})
	if err != nil {
		panic(err)
	}
	// query 77.291µs SELECT id, title, created_at FROM books WHERE INSTR(title, $1) > 0; [Bitcoin]

	fmt.Println(books)
	// [{001fa71a-bb2e-46bd-a545-0ce12a8229c1 The Bitcoin Standard 2024-09-12 20:00:19.963652 +0200 CEST} {b7905ecc-b023-482b-b5d2-68fc26fefc1c Mastering Bitcoin 2024-09-12 20:00:19.963665 +0200 CEST}]

	book, err := query.One(ctx, db, Query{Title: "The Bitcoin Standard"})
	if err != nil {
		panic(err)
	}
	// query 29.75µs SELECT id, title, created_at FROM books WHERE INSTR(title, $1) > 0; [The Bitcoin Standard]

	fmt.Println(book)
	// {001fa71a-bb2e-46bd-a545-0ce12a8229c1 The Bitcoin Standard 2024-09-12 20:00:19.963652 +0200 CEST}
}
```

## Example & Benchmarks

[https://github.com/wroge/vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture)

```
go test -bench . -benchmem ./...
goos: darwin
goarch: arm64
pkg: github.com/wroge/sqlt
cpu: Apple M3 Pro
BenchmarkSqltAll-12                32410             88496 ns/op           11236 B/op        108 allocs/op
BenchmarkSquirrelAll-12            34914             92241 ns/op           12341 B/op        108 allocs/op
BenchmarkSqltOne-12                35199             93324 ns/op           10006 B/op         96 allocs/op
BenchmarkSquirrelOne-12            35876             93790 ns/op           11353 B/op        101 allocs/op
BenchmarkSqltFirst-12              34674             92799 ns/op            9976 B/op         93 allocs/op
BenchmarkSquirreFirst-12           35218             98886 ns/op           11374 B/op        101 allocs/op
PASS
ok      github.com/wroge/sqlt   21.741s
```

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
