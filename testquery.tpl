
{{ define "query" }}
    SELECT
        100                                    {{ Dest.Int.Int }}
        , NULL                                 {{ Dest.String.String.Default "default" }}
        , true                                 {{ Dest.Bool.Bool }}
        , '2000-12-31'                         {{ Dest.Time.ParseTime DateOnly }}
        , '300'                                {{ Dest.Big.UnmarshalText }}
        , 'https://example.com/path?query=yes' {{ Dest.URL.UnmarshalBinary }}
        , 'hello,world'                        {{ Dest.Slice.Split "," }}
        , '{"hello":"world"}'                  {{ Dest.JSON.UnmarshalJSON }}
{{ end }}