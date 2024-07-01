package sqlt

import (
	"database/sql"
	stdsql "database/sql"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"
)

type namespace struct{}

func (namespace) Raw(sql string) Raw {
	return Raw(sql)
}

func (namespace) Expr(sql string, args ...any) Expression {
	return Expression{
		SQL:  sql,
		Args: args,
	}
}

func (namespace) Scanner(dest sql.Scanner, sql string, args ...any) (Scanner, error) {
	if reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.Scanner at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) ByteSlice(dest *[]byte, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.ByteSlice at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) String(dest *string, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.String at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullString(dest *stdsql.Null[string], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullString at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) StringP(dest **string, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.StringP at '%s'", sql)
	}

	var data stdsql.Null[string]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Int16(dest *int16, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int16 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullInt16(dest *stdsql.Null[int32], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullInt16 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) Int16P(dest **int16, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int16P at '%s'", sql)
	}

	var data stdsql.Null[int16]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Int32(dest *int32, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int32 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullInt32(dest *stdsql.Null[int32], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullInt32 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) Int32P(dest **int32, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int32P at '%s'", sql)
	}

	var data stdsql.Null[int32]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Int64(dest *int64, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int64 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullInt64(dest *stdsql.Null[int64], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullInt64 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) Int64P(dest **int64, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Int64P at '%s'", sql)
	}

	var data stdsql.Null[int64]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Float32(dest *float32, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float32 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullFloat32(dest *stdsql.Null[float32], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullFloat32 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) Float32P(dest **float32, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float32P at '%s'", sql)
	}

	var data stdsql.Null[float32]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Float64(dest *float64, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float64 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullFloat64(dest *stdsql.Null[float64], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullFloat64 at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) Float64P(dest **float64, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Float64P at '%s'", sql)
	}

	var data stdsql.Null[float64]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Bool(dest *bool, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Bool at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullBool(dest *stdsql.Null[bool], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullBool at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) BoolP(dest **bool, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.BoolP at '%s'", sql)
	}

	var data stdsql.Null[bool]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) Time(dest *time.Time, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.Time at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) NullTime(dest *stdsql.Null[time.Time], sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.NullTime at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: dest,
	}, nil
}

func (namespace) TimeP(dest **time.Time, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.TimeP at '%s'", sql)
	}

	var data stdsql.Null[time.Time]

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}

func (namespace) ParseTime(layout string, dest *time.Time, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.ParseTime at '%s'", sql)
	}

	var data string

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			v, err := time.Parse(layout, data)
			if err != nil {
				return err
			}

			*dest = v

			return nil
		},
	}, nil
}

func (namespace) JSON(dest json.Unmarshaler, sql string, args ...any) (Scanner, error) {
	if reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.JSON at '%s'", sql)
	}

	var data []byte

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			return json.Unmarshal(data, dest)
		},
	}, nil
}

func (namespace) RawJSON(dest *json.RawMessage, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.RawJSON at '%s'", sql)
	}

	var data []byte

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			return json.Unmarshal(data, dest)
		},
	}, nil
}

func (namespace) MapJSON(dest *map[string]any, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.MapJSON at '%s'", sql)
	}

	var data []byte

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			var m map[string]any

			if err := json.Unmarshal(data, &m); err != nil {
				return err
			}

			*dest = m

			return nil
		},
	}, nil
}

func (namespace) SplitString(sep string, dest *[]string, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.SplitString at '%s'", sql)
	}

	var data string

	return Scanner{
		SQL:  sql,
		Args: args,
		Dest: &data,
		Map: func() error {
			*dest = strings.Split(sep, data)

			return nil
		},
	}, nil
}
