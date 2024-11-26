# Go Template SQL Builder & ORM

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)

This package uses Go’s template engine to create a flexible, powerful and type-safe SQL builder and ORM ([example](https://github.com/wroge/vertical-slice-architecture)).

```go
// config example with ? placeholder, statement logging and template functions using https://masterminds.github.io/sprig/.
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

// insert one
insert := sqlt.Stmt[Params](config, 
	sqlt.Parse(`
		INSERT INTO books (id, title) VALUES ({{ uuidv4 }}, {{ .Title }});
	`),
)

result, err := insert.Exec(ctx, db, Params{...})

// insert many
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

// query returning id
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

// query book
query := sqlt.QueryStmt[string, Book](config,
	sqlt.Parse(`
		SELECT id, title FROM books WHERE INSTR(LOWER(title), {{ lower . }}) 
	`)
)

book, err := query.First(ctx, db, "Harry Potter")

// using different dialects
query := sqlt.QueryStmt[string, Book](config,
	sqlt.Funcs(template.FuncMap{
		"Dialect": func() string {
			return "postgres"
		},
	}),
	sqlt.Parse(`
		SELECT id, title FROM books WHERE
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
