package sqlt_test

import (
	"context"
	"database/sql"
	"encoding/csv"
	"fmt"
	"math/big"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/Masterminds/sprig/v3"
	"github.com/wroge/sqlt"
	_ "modernc.org/sqlite"
)

type Pokemon struct {
	Number         int64             `json:"number"`
	BigNumber      big.Int           `json:"bignumber"`
	NumberP        *int64            `json:"numberp"`
	Bisafans       url.URL           `json:"bisafans"`
	Name           string            `json:"name"`
	Height         float64           `json:"height"`
	HeightP        *float64          `json:"heightp"`
	Weight         sql.Null[float64] `json:"weight"`
	Generation     uint64            `json:"generation"`
	GenerationP    *uint64           `json:"generationp"`
	Legendary      bool              `json:"legendary"`
	LegendaryP     *bool             `json:"legendaryp"`
	Types          []string          `json:"types"`
	Classification *string           `json:"classification"`
	Abilities      []string          `json:"abilities"`
	SomeDate       time.Time         `json:"some_date,omitzero"`
	Today          time.Time         `json:"today,omitzero"`
}

func NewPointer[T any](t T) Pointer[T] {
	return &t
}

type Pointer[T any] *T

type Query struct {
	HeightRange    Pointer[[2]float64]
	WeightRange    Pointer[[2]float64]
	Generation     Pointer[uint64]
	Legendary      Pointer[bool]
	TypeOneOf      Pointer[[]string]
	Classification Pointer[string]
	AbilityOneOf   Pointer[[]string]
}

var (
	config = sqlt.Config{
		Placeholder: sqlt.Question,
		Cache:       &sqlt.Cache{},
		Templates: []sqlt.Template{
			sqlt.MissingKeyError(),
			sqlt.Funcs(sprig.TxtFuncMap()),
			sqlt.ParseFiles("./testdata/queries.sql"),
		},
		Log: func(ctx context.Context, info sqlt.Info) {
			fmt.Println(info.Template, info.SQL)
		},
	}
	create                       = sqlt.Exec[any](config, sqlt.Lookup("create"))
	insertTypes                  = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Lookup("insert_types"))
	insertClassifications        = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Lookup("insert_classifications"))
	insertAbilities              = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Dollar, sqlt.Lookup("insert_abilities"))
	insertPokemons               = sqlt.All[[][]string, int](config, sqlt.NoCache(), sqlt.Lookup("insert_pokemons"))
	insertPokemonTypes           = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Lookup("insert_pokemon_types"))
	insertPokemonClassifications = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Lookup("insert_pokemon_classifications"))
	insertPokemonAbilities       = sqlt.Exec[[][]string](config, sqlt.NoCache(), sqlt.Lookup("insert_pokemon_abilities"))
	query                        = sqlt.All[Query, Pokemon](config, sqlt.NoExpirationCache(100), sqlt.Lookup("query"))
	queryFirst                   = sqlt.First[Query, Pokemon](config, sqlt.UnlimitedSizeCache(time.Second), sqlt.Lookup("query"))
	queryOne                     = sqlt.One[Query, Pokemon](config, sqlt.Lookup("query"))
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
		TypeOneOf:  NewPointer([]string{"Dragon"}),
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
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}

	rattata, err = queryOne.Exec(ctx, db, Query{
		Classification: NewPointer("Mouse Pokémon"),
		WeightRange:    NewPointer([2]float64{3, 4}),
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}

	rattata, err = queryOne.Exec(ctx, db, Query{
		Classification: NewPointer("Mouse Pokémon"),
		WeightRange:    NewPointer([2]float64{3, 4}),
	})
	if err != nil {
		t.Fatal(err)
	}

	if rattata.Name != "Rattata" {
		t.Errorf("Expected Rattata, got %s", rattata.Name)
	}
}
