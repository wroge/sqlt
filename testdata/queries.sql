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
        legendary BOOLEAN,
        today DATE
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
    INSERT INTO pokemons (number, name, height, weight, generation, legendary, today) VALUES
    {{ range $i, $p := . }}
        {{ if $i }}, {{ end }}
        (
            {{ atoi (index $p 1) }}
            , {{ index $p 0 }}
            , {{ float64 (index $p 5) }}
            , {{ float64 (index $p 6) }}
            , {{ atoi (index $p 8) }}
            , {{ eq (index $p 9) "Yes" }}
            , {{ now }}
        )
    {{ end }}
    RETURNING number;
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
        p.number, 									{{ ScanInt "Number" }}	
        cast(p.number AS TEXT), 					{{ ScanText "BigNumber" }}	
        p.number, 									{{ ScanInt "NumberP" }}	
        CONCAT('https://www.bisafans.de/pokedex/', 
            printf('%03d', p.number) ,'.php'),      {{ ScanBinary "Bisafans" }}	
        p.name, 							        {{ ScanString "Name" }}
        p.height, 							        {{ ScanFloat "Height" }}	
        p.height, 							        {{ ScanFloat "HeightP" }}	
        p.weight, 							        {{ Scan "Weight" }}
        p.generation, 						        {{ ScanUint "Generation" }}
        p.generation, 						        {{ ScanUint "GenerationP" }}
        p.legendary, 						        {{ ScanBool "Legendary" }}
        p.legendary, 						        {{ ScanBool "LegendaryP" }}
        pt.type_names, 		                        {{ ScanStringSlice "Types" "," }}
        c.name, 						            {{ ScanString "Classification" }}
        pa.ability_names,	                        {{ ScanStringSlice "Abilities" "," }}
        '2000-01-01',                               {{ ScanStringTime "SomeDate" "DateOnly" "UTC" }}
        p.today,                                    {{ ScanTime "Today" }}
        JSON('{"hello": "world"}'),                 {{ ScanJSON "Meta" }}
        JSON('{"hello": "world"}'),                 {{ ScanBytes "MetaBytes" }}
        '100,-200,300',                             {{ ScanIntSlice "IntSlice" "," }}
        '100,200,300',                              {{ ScanUintSlice "UintSlice" "," }}
        '1.23,4.56',                                {{ ScanFloatSlice "FloatSlice" "," }}
        'true,false'                                {{ ScanBoolSlice "BoolSlice" "," }}
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
        AND c.name = {{ .Classification }}
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

{{ define "delete" }}
    DELETE FROM pokemon_abilities;
    DELETE FROM pokemon_classifications;
    DELETE FROM pokemon_types;
    DELETE FROM pokemons;
    DELETE FROM abilities;
    DELETE FROM classifications;
    DELETE FROM types;
{{ end }}