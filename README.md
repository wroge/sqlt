# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible, powerful and type-safe SQL builder and ORM.

- Type-safety without a build step (using [templatecheck](https://github.com/jba/templatecheck)),
- Avoiding SQL injection by escaping the templates (idea first found at [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)),
- Compact and versatile query building (using well-known template functions like [sprig](https://masterminds.github.io/sprig/)),
- Definition of struct mapping directly in the template,
- Abstraction allows the SQL logic to be placed close to the business logic without deeply nested layers (Locality of behavior).

## Quickstart

### Example 1

- using a simple insert statement.

```go
type Params struct {
	ID    int64
	Title string
}

insert := sqlt.Stmt[Params](
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES ({{ .ID }}, {{ .Title }});
	`),
)

result, err := insert.Exec(ctx, db, Params{ID: 1, Title: "Harry Potter"})
// INSERT INTO books (id, title) VALUES (?, ?); [1 "Harry Potter"]
```

### Example 2

- using a slice of structs as input param.

```go
insert := sqlt.Stmt[[]Params](
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES
			{{ range $i, $p := . }} 
				{{ if $i }}, {{ end }}
				({{ $p.ID }}, {{ $p.Title }})
			{{ end }}
		;
	`),
)

result, err := insert.Exec(ctx, db, []Params{
	{ID: 1, Title: "Harry Potter"},
	{ID: 2, Title: "Lord of the Rings"},
})
// INSERT INTO books (id, title) VALUES (?, ?), (?, ?); [1 "Harry Potter" 2 "Lord of the Rings"]
```

### Example 3

- returning a single column (for example auto incrementing id).

```go
query := sqlt.QueryStmt[[]string, int64](
	sqlt.Parse(`
		INSERT INTO books (title) VALUES
			{{ range $i, $t := . }} 
				{{ if $i }}, {{ end }}
				({{ $t }})
			{{ end }}
		RETURNING id;
	`),
)

ids, err := query.All(ctx, db, []string{"Harry Potter", "Lord of the Rings"})
// INSERT INTO books (title) VALUES (?), (?); ["Harry Potter" "Lord of the Rings"]
```

### Example 4

- querying multiple columns using scanners (```Book``` (and the alias ```Dest```) function returns the pointer to the destination struct).
- using ```lower``` function from sprig.

```go
type Book struct {
	ID    uuid.UUID
	Title string
}

query := sqlt.QueryStmt[string, Book](
	sqlt.Funcs(sprig.TxtFuncMap()),
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Book.ID "id" }}
			{{ ScanString Book.Title ", title" }}
		FROM books WHERE INSTR(LOWER(title), {{ lower . }});
	`),
)

book, err := query.First(ctx, db, "Harry Potter")
// SELECT id, title FROM books WHERE INSTR(LOWER(title), ?); ["harry potter"]
```

### Example 5

- supporting different placeholders and multiple sql dialects.

```go
query := sqlt.QueryStmt[string, Book](
	sqlt.Dollar(),
	sqlt.Funcs(template.FuncMap{
		"Dialect": func() string {
			return "postgres"
		},
	}),
	sqlt.Funcs(sprig.TxtFuncMap()),
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
		FROM books WHERE
		{{ if eq Dialect "sqlite" }}
			INSTR(LOWER(title), {{ lower . }})
		{{ else if eq Dialect "postgres" }}
			POSITION({{ lower . }} IN LOWER(title)) > 0
		{{ else }}
			{{ fail "invalid dialect" }}
		{{ end }};
	`),
)

books, err := query.All(ctx, db, "Harry Potter")
// SELECT id, title FROM books WHERE POSITION($1 IN LOWER(title)) > 0; ["harry potter"]
```

### Example 6

- outsourcing options into a config for reusability.
- applying logging or monitoring at the end of each "run".

```go
type StartTime struct{}

config := sqlt.Config{
	Start: func(runner *sqlt.Runner) {
		runner.Context = context.WithValue(runner.Context, StartTime{}, time.Now())
	},
	End: func(err error, runner *sqlt.Runner) {
		level := slog.LevelInfo

		if err != nil {
			level = slog.LevelError
		}

		slog.Log(runner.Context, level, "log stmt",
			"err", err,
			"sql", runner.SQL,
			"args", runner.Args,
			"location", fmt.Sprintf("[%s:%d]", runner.File, runner.Line),
			"duration", time.Since(runner.Context.Value(StartTime{}).(time.Time)),
		)
	},
	Placeholder: sqlt.Dollar(),
	TemplateOptions: []sqlt.TemplateOption{
		sqlt.Funcs(template.FuncMap{
			"Dialect": func() string {
				return "postgres"
			},
		}),
		sqlt.Funcs(sprig.TxtFuncMap()),
	},
}

query := sqlt.QueryStmt[string, Book](
	config,
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Dest.ID "id" }}
			{{ ScanString Dest.Title ", title" }}
		FROM books WHERE
		{{ if eq Dialect "sqlite" }}
			INSTR(LOWER(title), {{ lower . }})
		{{ else if eq Dialect "postgres" }}
			POSITION({{ lower . }} IN LOWER(title)) > 0
		{{ else }}
			{{ fail "invalid dialect" }}
		{{ end }};
	`),
)

book, err := query.One(ctx, db, "Harry Potter")
// SELECT id, title FROM books WHERE POSITION($1 IN LOWER(title)) > 0; ["harry potter"]
```

### Example 7

- Loading templates from a file (queries.go.tpl).

```go
{{ define "query" }}
	SELECT
		{{ ScanInt64 Dest.ID "id" }}
		{{ ScanString Dest.Title ", title" }}
	FROM books WHERE
	{{ if eq Dialect "sqlite" }}
		INSTR(LOWER(title), {{ lower . }})
	{{ else if eq Dialect "postgres" }}
		POSITION({{ lower . }} IN LOWER(title)) > 0
	{{ else }}
		{{ fail "invalid dialect" }}
	{{ end }};
{{ end }}
```

```go
query := sqlt.QueryStmt[string, Book](
	config,
	sqlt.ParseFiles("queries.go.tpl"),
	sqlt.Lookup("query"),
)

book, err := query.One(ctx, db, "Harry Potter")
// SELECT id, title FROM books WHERE POSITION($1 IN LOWER(title)) > 0; ["harry potter"]
```

## Any more Questions?

- Take a look into this example project: [vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture).
