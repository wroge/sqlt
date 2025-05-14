# A Go Template-Based SQL Builder and Struct Mapper

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-sqlt/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/go-sqlt/sqlt.svg?style=social)](https://github.com/go-sqlt/sqlt/tags)
[![Coverage](https://img.shields.io/badge/Coverage-71.9%25-brightgreen)](https://github.com/go-sqlt/sqlt/actions)

```go
go get -u github.com/go-sqlt/sqlt
```

`sqlt` uses Go’s template engine to create a flexible, powerful, and type-safe SQL builder and struct mapper.

## Example

```go
package main

import (
	"context"
	"database/sql"
	"fmt"
	"math/big"
	"net/url"
	"time"

	"github.com/go-sqlt/sqlt"
	_ "modernc.org/sqlite"
)

type Data struct {
	Int      int64
	String   *string
	Bool     bool
	Time     time.Time
	Big      *big.Int
	URL      *url.URL
	IntSlice []int
	JSON     map[string]any
}

var (
	query = sqlt.All[string, Data](
		sqlt.Parse(`
			SELECT
				100                                    {{ Scan "Int" }}
				, '200'                                {{ Scan "String" }}
				, true                                 {{ Scan "Bool" }}
				, {{ . }}                              {{ Scan "Time" (ParseTimeInLocation DateOnly UTC) }}
				, '300'                                {{ Scan "Big" UnmarshalText }}
				, 'https://example.com/path?query=yes' {{ Scan "URL" UnmarshalBinary }}
				, '400,500,600'                        {{ Scan "IntSlice" (Split "," (ParseInt 10 64)) }}
				, '{"hello":"world"}'                  {{ Scan "JSON" UnmarshalJSON }}
		`),
	)
)

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}

	data, err := query.Exec(context.Background(), db, time.Now().Format(time.DateOnly))
	if err != nil {
		panic(err)
	}

	fmt.Println(data)
	// [{100 0x140000116e0 true 2025-05-14 00:00:00 +0000 UTC 300 https://example.com/path?query=yes [400 500 600] map[hello:world]}]
}
```