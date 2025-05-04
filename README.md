# A Go Template-Based SQL Builder and Struct Mapper

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/go-sqlt/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/go-sqlt/sqlt.svg?style=social)](https://github.com/go-sqlt/sqlt/tags)
[![Coverage](https://img.shields.io/badge/Coverage-74.5%25-brightgreen)](https://github.com/go-sqlt/sqlt/actions)

```go
import "github.com/go-sqlt/sqlt"
```

`sqlt` uses Go’s template engine to create a flexible, powerful, and type-safe SQL builder and struct mapper.  

- [Website](https://go-sqlt.github.io)
- [Go Doc](https://pkg.go.dev/github.com/go-sqlt/sqlt)

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
	IntSlice []int32
	JSON     map[string]any
}

var (
	query = sqlt.All[any, Data](sqlt.Parse(`
		SELECT
			100                                    {{ Data.Int }}
			, '200'                                {{ Data.String }}
			, true                                 {{ Data.Bool }}
			, '2025-05-01'                         {{ Data.Time.String (ParseTime "DateOnly" "UTC") }}
			, '300'                                {{ Data.Big.Bytes UnmarshalText }}
			, 'https://example.com/path?query=yes' {{ Data.URL.Bytes UnmarshalBinary }}
			, '400,500,600'                        {{ Data.IntSlice.String (Split "," (ParseInt 10 64)) }}
			, '{"hello":"world"}'                  {{ Data.JSON.Bytes UnmarshalJSON }}
	`))
)

func main() {
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		panic(err)
	}

	data, err := query.Exec(context.Background(), db, nil)
	if err != nil {
		panic(err)
	}

	fmt.Println(data)
	// [{100 0x140000112b0 true 2025-05-01 00:00:00 +0000 UTC 300 https://example.com/path?query=yes [400 500 600] map[hello:world]}]
}
```