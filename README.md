# A Go Template-Based SQL Builder and Struct Mapper

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)
[![codecov](https://codecov.io/github/wroge/sqlt/graph/badge.svg?token=GDAWVVKGMR)](https://codecov.io/github/wroge/sqlt)

```go
import "github.com/wroge/sqlt"
```

`sqlt` uses Go’s template engine to create a flexible, powerful, and type-safe SQL builder and struct mapper.

## Type-Safety without a Build Step

- Define SQL statements at the global level using options like `New`, `Parse`, `ParseFiles`, `ParseFS`, `ParseGlob`, `Funcs` and `Lookup`.
- **Templates are validated via [jba/templatecheck](https://github.com/jba/templatecheck) during application startup**.
- Execute statements using methods such as `Exec`, `Query` or `QueryRow`.
- Execute query statements using `First`, `One` or `All`.
- Use `Scan` functions to map columns to struct fields (`Scan` for `sql.Scanner's`, `ScanInt64` for `int64`, `ScanString` for `string`, `ScanTime` for `time.Time`, `ScanStringP` for `*string`, etc.).
- Single-column queries do not require `Scan` functions.

```go
type Insert struct {
	ID    int64
	Title string
}

var insertBooks = sqlt.Stmt[[]Insert](
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES
		{{ range $i, $v := . }} 
			{{ if $i }}, {{ end }}
			({{ $v.ID }}, {{ $v.Title }})
		{{ end }};
	`),
)

type Query struct {
	Title string
}

type Book struct {
	ID    int64
	Title string
}

var queryBooks = sqlt.QueryStmt[Query, Book](
	sqlt.New("query_books"),
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
		FROM books
		WHERE title = {{ .Titel }};
	`),
)
// panic: location: [/.../main.go:17]: template: query_books:6:19: checking "query_books" at <.Titel>: can't use field Titel in type main.Query

var queryID = sqlt.QueryStmt[string, int64](
	sqlt.Parse(`SELECT id FROM books WHERE title = {{ . }};`),
)

result, err := insertBooks.Exec(ctx, db, []Insert{
	{ID: 1, Title: "The Hobbit"},
	{ID: 2, Title: "Harry Potter and the Philosopher's Stone"},
})

books, err := queryBooks.All(ctx, db, Query{Title: "The Hobbit"})

id, err := queryID.One(ctx, db, "The Hobbit")
```

## Support for multiple Dialects and Placeholders

- **Templates are escaped, ensuring the package is not vulnerable to SQL injection**.
- You can use both static placeholders (`?`) and positional placeholders (Go format strings like `$%d`).
- This package **supports any template functions** (like `lower` or `fail` from [Masterminds/sprig](https://github.com/Masterminds/sprig)).
- Multiple dialects can be used by implementing your own template functions.

```go
var queryBooks = sqlt.QueryStmt[string, Book](
	sqlt.Dollar(), // equivalent to sqlt.Placeholder("$%d")
	sqlt.Funcs(sprig.TxtFuncMap()),
	sqlt.Funcs(template.FuncMap{
		"Dialect": func() string {
			return "postgres"
		},
	}),
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
		FROM books
		WHERE
		{{ if eq Dialect "sqlite" }}
			INSTR(LOWER(title), {{ lower . }})
		{{ else if eq Dialect "postgres" }}
			POSITION({{ lower . }} IN LOWER(title)) > 0
		{{ else }}
			{{ fail "invalid dialect" }}
		{{ end }};
	`),
)

books, err := queryBooks.All(ctx, db, "The Hobbit")
// SELECT id, title FROM books WHERE POSITION($1 IN LOWER(title)) > 0; ["the hobbit"]
```

## Outsourcing Options into a Config

- All options can be grouped into a configuration struct for reusability.
- The `Start` and `End` functions enable monitoring and logging of SQL queries.

```go
type StartTime struct{}

var config = sqlt.Config{
	Placeholder: sqlt.Dollar(),
	TemplateOptions: []sqlt.TemplateOption{
		sqlt.Funcs(sprig.TxtFuncMap()),
		sqlt.Funcs(template.FuncMap{
			"Dialect": func() string {
				return "postgres"
			},
		}),
	},
	Start: func(runner *sqlt.Runner) {
		runner.Context = context.WithValue(runner.Context, StartTime{}, time.Now())
	},
	End: func(err error, runner *sqlt.Runner) {
		fmt.Println("location=%s, sql=%s, duration=%s", runner.Location, runner.SQL, time.Since(runner.Context.Value(StartTime{}).(time.Time)))
	},
}

var queryBooks = sqlt.QueryStmt[string, Book](
	config,
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
		FROM books
		WHERE
		{{ if eq Dialect "sqlite" }}
			INSTR(LOWER(title), {{ lower . }})
		{{ else if eq Dialect "postgres" }}
			POSITION({{ lower . }} IN LOWER(title)) > 0
		{{ else }}
			{{ fail "invalid dialect" }}
		{{ end }};
	`),
)
```

## Any more Questions?

- Take a look into my [vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture) example project.
- The template escape function is adapted from [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate).
