package sqlt

import (
	"bytes"
	"errors"
	"fmt"
	"strings"
	"text/template"
)

func Run[Dest any](t *Template, params any) Runner[Dest] {
	var (
		buf bytes.Buffer

		runner = Runner[Dest]{
			Value: new(Dest),
		}
	)

	t.text.Funcs(template.FuncMap{
		"Dest": func() any {
			return runner.Value
		},
		ident: func(arg any) (string, error) {
			if s, ok := arg.(Scanner); ok {
				runner.Dest = append(runner.Dest, s.Dest)
				runner.Map = append(runner.Map, s.Map)

				arg = Expression{
					SQL:  s.SQL,
					Args: s.Args,
				}
			}

			switch a := arg.(type) {
			case Raw:
				return string(a), nil
			case Expression:
				for {
					index := strings.IndexByte(a.SQL, '?')
					if index < 0 {
						buf.WriteString(a.SQL)

						return "", nil
					}

					if index < len(a.SQL)-1 && a.SQL[index+1] == '?' {
						buf.WriteString(a.SQL[:index+1])
						a.SQL = a.SQL[index+2:]

						continue
					}

					if len(a.Args) == 0 {
						return "", errors.New("invalid numer of arguments")
					}

					buf.WriteString(a.SQL[:index])
					runner.Args = append(runner.Args, a.Args[0])

					if t.positional {
						buf.WriteString(fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)))
					} else {
						buf.WriteString(t.placeholder)
					}

					a.Args = a.Args[1:]
					a.SQL = a.SQL[index+1:]
				}
			}

			runner.Args = append(runner.Args, arg)

			if t.positional {
				return fmt.Sprintf("%s%d", t.placeholder, len(runner.Args)), nil
			}

			return t.placeholder, nil
		},
	})

	if runner.Err = t.text.Execute(&buf, params); runner.Err != nil {
		return runner
	}

	runner.SQL = buf.String()

	return runner
}
