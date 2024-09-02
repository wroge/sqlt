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

	insert = sqlt.Typed[[]string](t.New("insert").MustParse(`
		INSERT INTO books (id, title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			({{ uuidv4 }}, {{ $t }}, {{ now }})
		{{ end }}
		RETURNING id;
	`))

	query = sqlt.TypedQuery[Book, string](t.New("query").MustParse(`
		SELECT
			{{ Scan Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
			{{ ScanTime Dest.CreatedAt ", created_at" }}
		FROM books
		WHERE INSTR(title, {{ . }}) > 0
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
	// insert 275.667µs INSERT INTO books (id, title, created_at) VALUES ( $1 , $2 , $3 ) , ( $4 , $5 , $6 ) , ( $7 , $8 , $9 ) , ( $10 , $11 , $12 ) RETURNING id;
	// [c66a2f34-d5ab-44d1-95f9-497d086e9d84 The Bitcoin Standard 2024-09-01 12:33:01.02574 +0200 CEST m=+0.007448335 8fbc0b2b-b96f-43dc-ab25-7bcf18174c72 Sapiens: A Brief History of Humankind 2024-09-01 12:33:01.025745 +0200 CEST m=+0.007452710 e8496edf-eb35-4b36-bf60-b9e65f8df67b 100 Go Mistakes and How to Avoid Them 2024-09-01 12:33:01.025747 +0200 CEST m=+0.007455668 a9dc918e-e517-4fc8-ad06-9cc6956377b5 Mastering Bitcoin 2024-09-01 12:33:01.02575 +0200 CEST m=+0.007458585]

	books, err := query.All(ctx, db, "Bitcoin")
	if err != nil {
		panic(err)
	}
	// query 62.583µs SELECT id , title , created_at FROM books WHERE INSTR(title, $1 ) > 0 [Bitcoin]

	fmt.Println(books)
	// [{c66a2f34-d5ab-44d1-95f9-497d086e9d84 The Bitcoin Standard 2024-09-01 12:33:01.02574 +0200 CEST} {a9dc918e-e517-4fc8-ad06-9cc6956377b5 Mastering Bitcoin 2024-09-01 12:33:01.02575 +0200 CEST}]
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
BenchmarkSqltAll-12                33141             86666 ns/op           10945 B/op        101 allocs/op
BenchmarkSquirrelAll-12            35822            104785 ns/op           12323 B/op        108 allocs/op
PASS
ok      github.com/wroge/sqlt   7.576s
```

## Inspiration

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
