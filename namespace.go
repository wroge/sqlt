package sqlt

import (
	stdsql "database/sql"
	"fmt"
	"reflect"
	"time"
)

type namespace struct{}

func (namespace) Raw(sql string) Raw {
	return Raw(sql)
}

func (namespace) Scanner(dest stdsql.Scanner, sql string, args ...any) (Scanner, error) {
	if reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("invalid sqlt.Scanner at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Dest: dest,
	}, nil
}

func (ns namespace) ByteSlice(dest *[]byte, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.ByteSlice at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
		Dest: dest,
	}, nil
}

func (namespace) String(dest *string, sql string, args ...any) (Scanner, error) {
	if dest == nil {
		return Scanner{}, fmt.Errorf("invalid sqlt.String at '%s'", sql)
	}

	return Scanner{
		SQL:  sql,
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
		Dest: &data,
		Map: func() error {
			if data.Valid {
				*dest = &data.V
			}

			return nil
		},
	}, nil
}
