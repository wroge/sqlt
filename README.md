# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible, powerful and type-safe SQL builder and ORM.

- Type-safety without a build step (using [templatecheck](https://github.com/jba/templatecheck)),
- Compact and versatile query building (using well-known template functions like [sprig](https://masterminds.github.io/sprig/)),
- Definition of struct mapping directly in the template,
- Abstraction allows the SQL logic to be placed close to the business logic without deeply nested layers (Locality of behavior).

Example: [vertical-slice-architecture](https://github.com/wroge/vertical-slice-architecture).

```go
// config example with ? placeholder, statement logging and template functions using sprig.
config := &sqlt.Config{
	Context: func(ctx context.Context, runner sqlt.Runner) context.Context {
		return context.WithValue(ctx, startKey{}, time.Now())
	},
	Log: func(ctx context.Context, err error, runner sqlt.Runner) {
		attrs := append(attrs,
			slog.String("sql", runner.SQL().String()),
			slog.Any("args", runner.Args()),
			slog.String("location", fmt.Sprintf("[%s:%d]", runner.File(), runner.Line())),
		)

		if start, ok := ctx.Value(startKey{}).(time.Time); ok {
			attrs = append(attrs, slog.Duration("duration", time.Since(start)))
		}

		if err != nil {
			logger.LogAttrs(ctx, slog.LevelError, err.Error(), attrs...)
		} else {
			logger.LogAttrs(ctx, slog.LevelInfo, "log stmt", attrs...)
		}
	},
	Placeholder: "?",
	Positional:  false,
	Options: []sqlt.Option{
		sqlt.Funcs(sprig.TxtFuncMap()),
	},
}

type Params struct {
	Title string
}

type Book struct {
	ID    uuid.UUID
	Title string
}

// Insert one.
insert := sqlt.Stmt[Params](config, 
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES ({{ uuidv4 }}, {{ .Title }});
	`),
)

result, err := insert.Exec(ctx, db, Params{...})

// Insert many.
insertMany := sqlt.Stmt[[]Params](config,
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES
			{{ range $i, $p := . }} 
			 	{{ if $i }}, {{ end }}
				({{ uuidv4 }}, {{ $p.Title }})
			{{ end }}
		;
	`),
)

result, err := insertMany.Exec(ctx, db, []Params{...})

// Query a single column.
insertReturning := sqlt.QueryStmt[[]Params, int64](config,
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES
			{{ range $i, $p := . }} 
			 	{{ if $i }}, {{ end }}
				({{ uuidv4 }}, {{ $p.Title }})
			{{ end }}
		RETURNING id;
	`),
)

ids, err := insertReturning.All(ctx, db, []Params{...})

// Map query results into a struct (multiple columns require sqlt.Scanner's).
// Book (alias for Dest) is a function, that returns a pointer to the destination struct.
query := sqlt.QueryStmt[string, Book](config,
	sqlt.Parse(`
		SELECT
			{{ ScanInt64 Book.ID "id" }}
			{{ ScanString Book.Title ", title" }}
		FROM books WHERE INSTR(LOWER(title), {{ lower . }}) 
	`)
)

book, err := query.First(ctx, db, "Harry Potter")

// Using different dialects.
dialect := "postgres"
config.Placeholder = "$"
config.Positional = true

query := sqlt.QueryStmt[string, Book](config,
	sqlt.Funcs(template.FuncMap{
		"Dialect": func() string {
			return dialect
		},
	}),
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
		{{ end }}
	`),
)

books, err := query.All(ctx, db, "Harry Potter")
```
