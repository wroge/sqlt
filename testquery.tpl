{{ define "query" }}
    SELECT
        100                                    {{ Scan.Int "Int" }}
        , NULL                                 {{ Scan.DefaultString "String" "default" }}
        , true                                 {{ Scan.Bool "Bool" }}
        , '2000-12-31'                         {{ Scan.ParseTime "Time" DateOnly }}
        , '300'                                {{ Scan.UnmarshalText "Big" }}
        , 'https://example.com/path?query=yes' {{ Scan.UnmarshalBinary "URL" }}
        , 'hello,world'                        {{ Scan.Split "Slice" "," }}
        , '{"hello":"world"}'                  {{ Scan.UnmarshalJSON "JSON" }}
{{ end }}
