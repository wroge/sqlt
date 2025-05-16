package sqlt_test

import (
	"context"
	"database/sql"
	"encoding/csv"
	"iter"
	"math/big"
	"net/url"
	"os"
	"slices"
	"testing"
	"text/template"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/go-sqlt/sqlt"
	_ "modernc.org/sqlite"
)

type Pokemon struct {
	Number         int64
	BigNumber      big.Int
	NumberP        *int64
	Bisafans       url.URL
	Name           string
	Height         float64
	HeightP        *float64
	Weight         sql.Null[float64]
	Generation     uint64
	GenerationP    *uint64
	Legendary      bool
	LegendaryP     *bool
	Types          []*string
	Classification *string
	Abilities      []string
	SomeDate       time.Time
	SomeDateP      *time.Time
	Today          time.Time
	TodayP         *time.Time
	Meta           map[string]string
	MetaBytes      []byte
}

func NewPointer[T any](t T) Pointer[T] {
	return &t
}

type Pointer[T any] *T

type Query struct {
	HeightRange    Pointer[[2]float64]
	WeightRange    iter.Seq[float64]
	Generation     Pointer[uint64]
	Legendary      Pointer[bool]
	TypeOneOf      []string
	Classification Pointer[string]
	AbilityOneOf   map[string]struct{}
	Date           time.Time
	URL            *url.URL
	Big            big.Int
}

var (
	config = sqlt.Sqlite().With(
		sqlt.Funcs(sprig.TxtFuncMap()),
		sqlt.Funcs(template.FuncMap{
			"SetToSeq": func(m map[string]struct{}) iter.Seq2[int, string] {
				return func(yield func(int, string) bool) {
					var index int

					for v := range m {
						if !yield(index, v) {
							return
						}

						index++
					}
				}
			},
		}),
		sqlt.ParseFiles("./testdata/queries.sql"),
		sqlt.Log(func(ctx context.Context, info sqlt.Info) {}),
	)

	create                       = sqlt.Exec[any](config, sqlt.Lookup("create"))
	insertTypes                  = sqlt.Exec[[][]string](config, sqlt.Lookup("insert_types"))
	insertClassifications        = sqlt.Exec[[][]string](config, sqlt.Lookup("insert_classifications"))
	insertAbilities              = sqlt.Exec[[][]string](config, sqlt.Dollar(), sqlt.Lookup("insert_abilities"))
	insertPokemons               = sqlt.All[[][]string, int](config, sqlt.Lookup("insert_pokemons"))
	insertPokemonTypes           = sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_types"))
	insertPokemonClassifications = sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_classifications"))
	insertPokemonAbilities       = sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_abilities"))
	query                        = sqlt.All[Query, Pokemon](config, sqlt.Lookup("query"))
	queryFirst                   = sqlt.First[Query, Pokemon](config, sqlt.ExpressionExpiration(time.Second), sqlt.Lookup("query"))
	queryOne                     = sqlt.One[Query, Pokemon](config, sqlt.ExpressionSize(100), sqlt.Lookup("query"))
)

func TestQueryPokemon(t *testing.T) {
	db, err := sql.Open("sqlite", "file:pokemon.db?mode=memory")
	if err != nil {
		t.Fatal(err)
	}

	ctx := context.Background()

	file, err := os.Open("./testdata/pokemon_data_pokeapi.csv")
	if err != nil {
		t.Fatal(err)
	}

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		t.Fatal(err)
	}

	_, err = create.Exec(ctx, db, nil)
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertTypes.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertClassifications.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertAbilities.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertPokemons.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertPokemonTypes.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertPokemonClassifications.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	_, err = insertPokemonAbilities.Exec(ctx, db, records[1:])
	if err != nil {
		t.Fatal(err)
	}

	pokemons, err := query.Exec(ctx, db, Query{
		TypeOneOf:  []string{"Dragon"},
		Generation: NewPointer[uint64](1),
	})
	if err != nil {
		t.Fatal(err)
	}

	if len(pokemons) != 3 {
		t.Errorf("Expected 3 Pokémon, got %d", len(pokemons))
	}

	rattata, err := queryFirst.Exec(ctx, db, Query{
		Classification: NewPointer("Mouse Pokémon"),
		AbilityOneOf: map[string]struct{}{
			"Run-away": {},
		},
		Big:  *big.NewInt(10),
		URL:  &url.URL{Host: "localhost"},
		Date: time.Now(),
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}

	rattata, err = queryOne.Exec(ctx, db, Query{
		Classification: NewPointer("Mouse Pokémon"),
		WeightRange:    slices.Values([]float64{3, 4}),
		AbilityOneOf: map[string]struct{}{
			"Run-away": {},
			"Hustle":   {},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}

	rattata, err = queryOne.Exec(ctx, db, Query{
		Classification: NewPointer("Mouse Pokémon"),
		WeightRange:    slices.Values([]float64{3, 4}),
		AbilityOneOf: map[string]struct{}{
			"Run-away": {},
			"Hustle":   {},
		},
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}
}
