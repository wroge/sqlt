package sqlt_test

import (
	"context"
	"database/sql"
	"fmt"
	"io"

	"github.com/wroge/sqlt"
	_ "modernc.org/sqlite"
)

func Example_one() {
	var (
		ctx    = context.Background()
		create = sqlt.Exec[any](sqlt.Parse(`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT);`))
		insert = sqlt.Exec[string](sqlt.Parse(`INSERT INTO books (title) VALUES ({{ . }});`))
		query  = sqlt.First[string, int64](sqlt.Parse(`SELECT id FROM books WHERE title = {{ . }};`))
	)

	db, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	if _, err := create.Exec(ctx, db, nil); err != nil {
		panic(err)
	}

	if _, err := insert.Exec(ctx, db, "One"); err != nil {
		panic(err)
	}

	if _, err := insert.Exec(ctx, db, "Two"); err != nil {
		panic(err)
	}

	id, err := query.Exec(ctx, db, "Two")
	if err != nil {
		panic(err)
	}

	fmt.Println(id)

	// Output: 2
}

func Example_two() {
	type Book struct {
		ID    int64
		Title string
	}

	var (
		ctx    = context.Background()
		config = sqlt.Config{
			Placeholder: sqlt.Dollar,
			Log: func(ctx context.Context, info sqlt.Info) {
				fmt.Printf("sql: '%s' cached: '%v'\n", info.SQL, info.Cached)
			},
			Hasher: sqlt.DefaultHasher(),
		}
		create = sqlt.Exec[any](
			config,
			sqlt.Parse(`CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT);`),
		)
		insert = sqlt.All[[]string, int64](
			config,
			sqlt.Parse(`
				INSERT INTO books (title) VALUES
					{{ range $i, $v := . }} 
						{{ if $i }}, {{ end }}
						({{ $v }})
					{{ end }}
				RETURNING id;`,
			),
		)
		query = sqlt.All[any, Book](
			config,
			sqlt.NoExpirationCache(1000),
			sqlt.Hasher(func(param any, writer io.Writer) error {
				return nil
			}),
			sqlt.Parse(`
				SELECT 
					{{ Scan "ID" "id" }}
					{{ Scan "Title" ", title" }}
				FROM books;`,
			),
		)
	)

	db, err := sql.Open("sqlite", "file:test.db?cache=shared&mode=memory")
	if err != nil {
		panic(err)
	}

	defer db.Close()

	if _, err := create.Exec(ctx, db, nil); err != nil {
		panic(err)
	}

	ids, err := insert.Exec(ctx, db, []string{"One", "Two", "Three"})
	if err != nil {
		panic(err)
	}

	fmt.Println(ids)

	books, err := query.Exec(ctx, db, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(books)

	books, err = query.Exec(ctx, db, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(books)

	// Output:
	// sql: 'CREATE TABLE books (id INTEGER PRIMARY KEY, title TEXT);' cached: 'false'
	// sql: 'INSERT INTO books (title) VALUES ($1) , ($2) , ($3) RETURNING id;' cached: 'false'
	// [1 2 3]
	// sql: 'SELECT id , title FROM books;' cached: 'false'
	// [{1 One} {2 Two} {3 Three}]
	// sql: 'SELECT id , title FROM books;' cached: 'true'
	// [{1 One} {2 Two} {3 Three}]
}
