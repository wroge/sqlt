package sqlt_test

import (
	"context"
	"database/sql"
	"encoding/csv"
	"os"
	"testing"

	"github.com/Masterminds/sprig/v3"
	"github.com/wroge/sqlt"

	_ "modernc.org/sqlite"
)

type Pokemon struct {
	Number         int64    `json:"number"`
	Name           string   `json:"name"`
	Height         float64  `json:"height"`
	Weight         float64  `json:"weight"`
	Generation     uint64   `json:"generation"`
	Legendary      bool     `json:"legendary"`
	Types          []string `json:"types"`
	Classification string   `json:"classification"`
	Abilities      []string `json:"abilities"`
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
			sqlt.Funcs(sprig.TxtFuncMap()),
			sqlt.ParseFiles("./testdata/queries.sql"),
		},
	}

	create = sqlt.Exec[any](config, sqlt.Lookup("create"))

	insert = sqlt.Transaction(
		nil,
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_types")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_classifications")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_abilities")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemons")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_types")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_classifications")),
		sqlt.Exec[[][]string](config, sqlt.Lookup("insert_pokemon_abilities")),
	)

	query = sqlt.All[Query, Pokemon](config, sqlt.Lookup("query"))

	queryFirst = sqlt.First[Query, Pokemon](config, sqlt.Lookup("query"))
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

	_, err = insert.Exec(ctx, db, records[1:])
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
}
