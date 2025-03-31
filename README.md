# A Go Template-Based SQL Builder and Struct Mapper

[![go.dev reference](https://img.shields.io/badge/go.dev-reference-007d9c?logo=go&logoColor=white)](https://pkg.go.dev/github.com/wroge/sqlt)
[![GitHub tag (latest SemVer)](https://img.shields.io/github/tag/wroge/sqlt.svg?style=social)](https://github.com/wroge/sqlt/tags)
[![codecov](https://codecov.io/github/wroge/sqlt/graph/badge.svg?token=GDAWVVKGMR)](https://codecov.io/github/wroge/sqlt)

```go
import "github.com/wroge/sqlt"
```

`sqlt` uses Go’s template engine to create a flexible, powerful, and type-safe SQL builder and struct mapper.  

- Vertical Slice Architecture [Example](https://github.com/wroge/vertical-slice-architecture)

# Example

- Dataset from [kaggle](https://www.kaggle.com/datasets/mohitbansal31s/pokemon-dataset)

```sql
{{ define "create" }}
    CREATE TABLE IF NOT EXISTS types (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS classifications (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS abilities (
        id INTEGER PRIMARY KEY,
        name TEXT UNIQUE NOT NULL
    );

    CREATE TABLE IF NOT EXISTS pokemons (
        number INTEGER PRIMARY KEY,
        name TEXT UNIQUE,
        height NUMERIC,
        weight NUMERIC,
        generation INTEGER,
        legendary BOOLEAN
    );

    CREATE TABLE IF NOT EXISTS pokemon_types (
        pokemon_number INTEGER REFERENCES pokemons (number),
        type_id INTEGER REFERENCES types (id)
    );

    CREATE TABLE IF NOT EXISTS pokemon_classifications (
        pokemon_number INTEGER REFERENCES pokemons (number),
        classification_id INTEGER REFERENCES classifications (id)
    );

    CREATE TABLE IF NOT EXISTS pokemon_abilities (
        pokemon_number INTEGER REFERENCES pokemons (number),
        ability_id INTEGER REFERENCES abilities (id)
    );
{{ end }}

{{ define "insert_types" }}
    INSERT INTO types (name) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ with (index . 2) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}

        {{ with (index . 3) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}
    {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_classifications" }}
    INSERT INTO classifications (name) VALUES 
        {{ range $i, $p := . }}
            {{ if $i }}, {{ end }}
            ({{ index $p 4 }})
        {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_abilities" }}
    INSERT INTO abilities (name) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ range (splitList ", " (index . 7)) }}
            {{ if not $first }}, {{ end }}
            ({{ . }})
            {{ $first = false }}
        {{ end }}
    {{ end }}
    ON CONFLICT DO NOTHING;
{{ end }}

{{ define "insert_pokemons" }}
    INSERT INTO pokemons (number, name, height, weight, generation, legendary) VALUES
    {{ range $i, $p := . }}
        {{ if $i }}, {{ end }}
        (
            {{ atoi (index $p 1) }}
            , {{ index $p 0 }}
            , {{ float64 (index $p 5) }}
            , {{ float64 (index $p 6) }}
            , {{ atoi (index $p 8) }}
            , {{ eq (index $p 9) "Yes" }}
        )
    {{ end }};
{{ end }}

{{ define "insert_pokemon_types" }}
    INSERT INTO pokemon_types (pokemon_number, type_id) VALUES
    {{ $first := true }}
    {{ range . }}
        {{ $number := atoi (index . 1) }}

        {{ if (index . 2) }}
            {{ if not $first }},{{ end }}
            ({{ $number }}, (SELECT id FROM types WHERE name = {{ index . 2 }}))
            {{ $first = false }}
        {{ end }}

        {{ if (index . 3) }}
            {{ if not $first }},{{ end }}
            ({{ $number }}, (SELECT id FROM types WHERE name = {{ index . 3 }}))
            {{ $first = false }}
        {{ end }}
    {{ end }};
{{ end }}

{{ define "insert_pokemon_classifications" }}
    INSERT INTO pokemon_classifications (pokemon_number, classification_id) VALUES
    {{ range $i, $p := . }}
        {{ if $i }}, {{ end }}
        ({{ atoi (index $p 1) }}, (SELECT id FROM classifications WHERE name = {{ index $p 4 }}))
    {{ end }};
{{ end }}

{{ define "insert_pokemon_abilities" }}
    INSERT INTO pokemon_abilities (pokemon_number, ability_id) VALUES
    {{ $first := true }}
    {{ range $p := . }}
        {{ range (splitList ", " (index $p 7)) }}
            {{ if not $first }},{{ end }}
            ({{ atoi (index $p 1) }}, (SELECT id FROM abilities WHERE name = {{ . }}))
            {{ $first = false }}
        {{ end }}
    {{ end }};
{{ end }}

{{ define "query" }}
    SELECT 
        p.number,                       {{ Scan "Number" }}
        p.name,                         {{ Scan "Name" }}
        p.height,                       {{ Scan "Height" }}	
        p.weight,                       {{ Scan "Weight" }}
        p.generation,                   {{ Scan "Generation" }}
        p.legendary,                    {{ Scan "Legendary" }}
        pt.type_names,                  {{ ScanSplit "Types" "," }}
        c.name,                         {{ Scan "Classification" }}
        pa.ability_names                {{ ScanSplit "Abilities" "," }}
    FROM pokemons p
    LEFT JOIN (
        SELECT pokemon_number, GROUP_CONCAT(types.name, ',') AS type_names
        FROM pokemon_types 
        JOIN types ON types.id = pokemon_types.type_id
        GROUP BY pokemon_number
    ) pt ON p.number = pt.pokemon_number
    LEFT JOIN (
        SELECT pokemon_number, GROUP_CONCAT(abilities.name, ',') AS ability_names
        FROM pokemon_abilities 
        JOIN abilities ON abilities.id = pokemon_abilities.ability_id
        GROUP BY pokemon_number
    ) pa ON p.number = pa.pokemon_number
    LEFT JOIN pokemon_classifications pc ON p.number = pc.pokemon_number
    LEFT JOIN classifications c ON c.id = pc.classification_id
    WHERE 1=1
    {{ if .HeightRange }}
        AND p.height >= {{ index .HeightRange 0 }} AND p.height <= {{ index .HeightRange 1 }}
    {{ end }}
    {{ if .WeightRange }}
        AND p.weight >= {{ index .WeightRange 0 }} AND p.weight <= {{ index .WeightRange 1 }}
    {{ end }}
    {{ if .Generation }}
        AND p.generation = {{ .Generation }}
    {{ end }}
    {{ if .TypeOneOf }}
        AND p.number IN (
            SELECT pokemon_number 
            FROM pokemon_types 
            JOIN types ON types.id = pokemon_types.type_id 
            WHERE types.name IN (
                {{ range $i, $t := .TypeOneOf }}
                    {{ if $i }}, {{ end }}
                    {{ $t }}
                {{ end }}
            )
        )
    {{ end }}
    {{ if .Classification }}
        AND p.classification = {{ .Classification }}
    {{ end }}
    {{ if .AbilityOneOf }}
        AND p.number IN (
            SELECT pokemon_number 
            FROM pokemon_abilities 
            JOIN abilities ON abilities.id = pokemon_abilities.ability_id 
            WHERE abilities.name IN (
                {{ range $i, $a := .AbilityOneOf }}
                    {{ if $i }}, {{ end }}
                    {{ $a }}
                {{ end }}
            )
        )
    {{ end }}
    ORDER BY p.number;
{{ end }}
```

- Example with sqlite

```go
package main

import (
	"context"
	"database/sql"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"

	"github.com/Masterminds/sprig/v3"
	"github.com/wroge/sqlt"

	_ "modernc.org/sqlite"
)

type Pokemon struct {
	Number         int      `json:"number"`
	Name           string   `json:"name"`
	Height         float64  `json:"height"`
	Weight         float64  `json:"weight"`
	Generation     int      `json:"generation"`
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
	Generation     Pointer[int]
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
			sqlt.ParseFiles("./queries.sql"),
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
)

func main() {
	db, err := sql.Open("sqlite", "file:pokemon.db?mode=memory") // ?mode=memory
	if err != nil {
		panic(err)
	}

	ctx := context.Background()

	file, err := os.Open("./pokemon_data_pokeapi.csv")
	if err != nil {
		panic(err)
	}

	records, err := csv.NewReader(file).ReadAll()
	if err != nil {
		panic(err)
	}

	_, err = create.Exec(ctx, db, nil)
	if err != nil {
		panic(err)
	}

	_, err = insert.Exec(ctx, db, records[1:])
	if err != nil {
		panic(err)
	}

	pokemons, err := query.Exec(ctx, db, Query{
		TypeOneOf:  NewPointer([]string{"Dragon"}),
		Generation: NewPointer(1),
	})
	if err != nil {
		panic(err)
	}

	js, err := json.MarshalIndent(pokemons, "", "   ")
	if err != nil {
		panic(err)
	}

	fmt.Println(string(js))
	// [
	//	   {
	//	      "number": 147,
	//	      "name": "Dratini",
	//	      "height": 1.8,
	//	      "weight": 3.3,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Shed-skin",
	//	         "Marvel-scale"
	//	      ]
	//	   },
	//	   {
	//	      "number": 148,
	//	      "name": "Dragonair",
	//	      "height": 4,
	//	      "weight": 16.5,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Shed-skin",
	//	         "Marvel-scale"
	//	      ]
	//	   },
	//	   {
	//	      "number": 149,
	//	      "name": "Dragonite",
	//	      "height": 2.2,
	//	      "weight": 210,
	//	      "generation": 1,
	//	      "legendary": false,
	//	      "types": [
	//	         "Dragon",
	//	         "Flying"
	//	      ],
	//	      "classification": "Dragon Pokémon",
	//	      "abilities": [
	//	         "Inner-focus",
	//	         "Multiscale"
	//	      ]
	//	   }
	//
	// ]
}
```
