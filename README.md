# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible, powerful and type-safe SQL builder and ORM.

```go
go get -u github.com/wroge/sqlt
```

## How does it work?

- All input values are safely escaped and replaced with the correct placeholders at execution time.
- Functions like ```ScanInt64``` generate ```sqlt.Scanner`s```, which hold pointers to the destination and optionally a mapper. These scanners are collected at execution time.
- The ```Dest``` function is a placeholder that is replaced at execution time with the appropriate generic type.
- SQL templates can be loaded from the filesystem using ```ParseFS``` or ```ParseFiles```.
- ```Type``` and ```MustType``` functions do type-safe checks using: [jba/templatecheck](https://github.com/jba/templatecheck).
- Additionally, ```Type``` the type of the arguments.

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
		BeforeRun(func(runner *sqlt.Runner) {
			runner.Context = context.WithValue(runner.Context, startKey{}, time.Now())
		}).
		AfterRun(func(err error, runner *sqlt.Runner) error {
			var duration = time.Since(runner.Context.Value(startKey{}).(time.Time))

			if err != nil {
				// apply error logging here
				fmt.Println(err, runner.Text.Name(), duration, runner.SQL, runner.Args)

				return err
			}

			// apply normal logging here
			fmt.Println(runner.Text.Name(), duration, runner.SQL, runner.Args)

			return nil
		})

	insert = sqlt.MustType[any, []string](t.New("insert").MustParse(`
		{{ $now := now }}
		INSERT INTO books (id, title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			({{ uuidv4 | Type "string" }}, {{ $t | Type "string" }}, {{ $now | Type "time.Time" }})
		{{ end }}
		RETURNING id;
	`))

	query = sqlt.MustType[Book, Query](t.New("query").MustParse(`
		SELECT
			{{ Scan Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
			{{ ScanTime Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE INSTR(title, {{ .Title | Type "string" }}) > 0
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
	// insert 331.542µs INSERT INTO books (id, title, created_at) VALUES ( $1 , $2 , $3 ) , ( $4 , $5 , $6 ) , ( $7 , $8 , $9 ) , ( $10 , $11 , $12 ) RETURNING id;
	// [7110b963-ef0e-446c-aa0f-75002eea16c7 The Bitcoin Standard 2024-09-10 13:25:11.559146 +0200 CEST m=+0.012192335 82304326-9b79-4180-8801-647f3acaa4d9 Sapiens: A Brief History of Humankind 2024-09-10 13:25:11.559146 +0200 CEST m=+0.012192335 165fb6c1-f707-493b-8db2-ab20ac743098 100 Go Mistakes and How to Avoid Them 2024-09-10 13:25:11.559146 +0200 CEST m=+0.012192335 a846f4af-87b1-4a5f-98a3-96c87efac522 Mastering Bitcoin 2024-09-10 13:25:11.559146 +0200 CEST m=+0.012192335]

	books, err := query.All(ctx, db, Query{Title: "Bitcoin"})
	if err != nil {
		panic(err)
	}
	// query 98.375µs SELECT id , title , created_at FROM books WHERE INSTR(title, $1 ) > 0 [Bitcoin]

	fmt.Println(books)
	// [{7110b963-ef0e-446c-aa0f-75002eea16c7 The Bitcoin Standard 2024-09-10 13:25:11.559146 +0200 CEST} {a846f4af-87b1-4a5f-98a3-96c87efac522 Mastering Bitcoin 2024-09-10 13:25:11.559146 +0200 CEST}]
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
BenchmarkSqltAll-12                33496             89417 ns/op           10944 B/op        101 allocs/op
BenchmarkSquirrelAll-12            36298             96829 ns/op           12304 B/op        108 allocs/op
PASS
ok      github.com/wroge/sqlt   7.404s
```

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
