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

		// INSERT INTO books (id, title, created_at) VALUES

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
	// insert 423.75µs INSERT INTO books (id, title, created_at) VALUES ($1, $2, $3) , ($4, $5, $6) , ($7, $8, $9) , ($10, $11, $12) RETURNING id;
	// [979f9bff-a250-466a-8217-e6fc1dba6cb2 The Bitcoin Standard 2024-08-28 20:03:39.275247 +0200 CEST m=+0.011488501 f1e517b4-95af-4683-afba-76af0bf0a194 Sapiens: A Brief History of Humankind 2024-08-28 20:03:39.275253 +0200 CEST m=+0.011494835 0dbde871-6cfc-41ed-902d-84f10def6291 100 Go Mistakes and How to Avoid Them 2024-08-28 20:03:39.275258 +0200 CEST m=+0.011499251 1e318d79-3773-44b6-b10b-6cd3096b4f36 Mastering Bitcoin 2024-08-28 20:03:39.275262 +0200 CEST m=+0.011503543]

	books, err := sqlt.FetchAll[Book](ctx, query, db, map[string]any{
		"Search": "Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// query 97.042µs SELECT id , title , created_at FROM books WHERE INSTR(title, $1) > 0  [Bitcoin]

	fmt.Println(books)
	// [{979f9bff-a250-466a-8217-e6fc1dba6cb2 The Bitcoin Standard 2024-08-28 20:03:39.275247 +0200 CEST} {1e318d79-3773-44b6-b10b-6cd3096b4f36 Mastering Bitcoin 2024-08-28 20:03:39.275262 +0200 CEST}]
}
```

## Example & Benchmarks

[https://github.com/wroge/vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture)

```
go test -bench . -benchmem .                   
goos: darwin
goarch: arm64
pkg: github.com/wroge/sqlt
cpu: Apple M3 Pro
BenchmarkSqltAll-12                26454             85048 ns/op           11170 B/op        107 allocs/op
BenchmarkSquirrelAll-12            33936             93216 ns/op           12297 B/op        107 allocs/op
PASS
ok      github.com/wroge/sqlt   6.426s
```

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
