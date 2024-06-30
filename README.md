# sqlt - SQL Templates

This module (ab)uses Go's template engine to create an SQL builder and ORM.  
Just take a look at the code and let me know what you think of this approach.  
Might be dumb, but it surprisingly works pretty well.

## Quick Notes

- It prevents SQL injection by replacing values with placeholders.
- Use sqlt.ParseFS oder sqlt.ParseFiles to load templates from the file system.
- Implement own scanner by adding a template function with the return type sqlt.Scanner.
- Expr function can be used for better readability - '?' is replaced at execution time with the correct placeholder.

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

	insert = sqlt.Dest[int64](t.New("insert").MustParse(`
		INSERT INTO books (title, created_at) VALUES
		{{ range $i, $t := . }} {{ if $i }}, {{ end }}
			{{ Expr "(?, ?)" $t now }}
		{{ end }}
		RETURNING {{ Int64 Dest "id" }};
	`))

	query = sqlt.Dest[Book](t.New("query").MustParse(`
		SELECT 
			{{ Int64 Dest.ID "id" }}
			{{ String Dest.Title ", title" }}
			{{ Time Dest.CreatedAt ", created_at" }}
		FROM books 
		WHERE instr(title, {{ .Search }}) > 0
	`))
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

	ids, err := insert.QueryAll(ctx, db, []string{
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

	books, err := query.QueryAll(ctx, db, map[string]any{
		"Search": "Bitcoin",
	})
	if err != nil {
		panic(err)
	}
	// SELECT id, title, created_at FROM books WHERE instr(title, ?) > 0

	fmt.Println(books)
	// [{1 The Bitcoin Standard 2024-06-29 12:32:35.41204 +0200 +0200} {4 Mastering Bitcoin 2024-06-29 12:32:35.412049 +0200 +0200}]
}
```

The inspiration comes from:

- [VauntDev/tqla](https://github.com/VauntDev/tqla)
- [mhilton/sqltemplate](https://github.com/mhilton/sqltemplate)
