package sqlt_test

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"reflect"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Masterminds/squirrel"
	"github.com/wroge/sqlt"
)

type testCase struct {
	params         Param
	expectedResult interface{}
	tpl            *sqlt.Template
	mockRows       *sqlmock.Rows
	testFunc       func(t *testing.T, db *sql.DB, tc testCase)
	expectedSQL    string
	expectedArgs   []driver.Value
	expectError    bool
}

type Param struct {
	Time    time.Time
	Int64   int64
	String  string
	JSON    json.RawMessage
	Float64 float64
}

type Result struct {
	Time    time.Time
	String  string
	JSON    json.RawMessage
	Int     int
	Int64   int64
	Float64 float64
	Bool    bool
}

var (
	date, _ = time.Parse(time.DateTime, "2010-11-20 08:10:30")
)

func TestStuff(t *testing.T) {
	tests := []testCase{
		{
			tpl:            sqlt.New("test-1").MustParse(`SELECT {{ .Int64 }} {{ ScanInt64 Dest.Int64 "AS int64," }} {{ .String }} {{ ScanString Dest.String "AS string" }}`),
			params:         Param{Int64: 100, String: "hundred"},
			mockRows:       sqlmock.NewRows([]string{"int64", "string"}).AddRow(100, "hundred"),
			expectedSQL:    `SELECT \? AS int64, \? AS string`,
			expectedArgs:   []driver.Value{100, ("hundred")},
			expectedResult: Result{Int64: 100, String: "hundred"},
			expectError:    false,
			testFunc:       testOne,
		},
		{
			tpl:            sqlt.New("test-3").MustParse(`SELECT {{ .Time }} {{ ScanTime Dest.Time "AS time," }} {{ .JSON }} {{ ScanJSON Dest.JSON "AS json" }}`),
			params:         Param{Time: date, JSON: []byte(`{"hundred": 100}`)},
			mockRows:       sqlmock.NewRows([]string{"time", "json"}).AddRow(date, []byte(`{"hundred": 100}`)),
			expectedSQL:    `SELECT \? AS time, \? AS json`,
			expectedArgs:   []driver.Value{date, []byte(`{"hundred": 100}`)},
			expectedResult: []Result{{Time: date, JSON: []byte(`{"hundred": 100}`)}},
			expectError:    false,
			testFunc:       testAll,
		},
		{
			tpl:            sqlt.New("test-4").MustParse(`SELECT {{ .Int64 }} {{ ScanInt Dest.Int "AS int" }}`),
			params:         Param{Int64: 42},
			mockRows:       sqlmock.NewRows([]string{"int"}).AddRow(42),
			expectedSQL:    `SELECT \? AS int`,
			expectedArgs:   []driver.Value{42},
			expectedResult: Result{Int: 42},
			expectError:    false,
			testFunc:       testOne,
		},
		{
			tpl:            sqlt.New("test-6").MustParse(`SELECT {{ .Float64 }} {{ ScanFloat64 Dest.Float64 "AS float64," }} {{ .JSON }} {{ ScanJSON Dest.JSON "AS json" }}`),
			params:         Param{Float64: 3.14, JSON: []byte(`{"pi": 3.14}`)},
			mockRows:       sqlmock.NewRows([]string{"float64", "json"}).AddRow(3.14, []byte(`{"pi": 3.14}`)),
			expectedSQL:    `SELECT \? AS float64, \? AS json`,
			expectedArgs:   []driver.Value{3.14, []byte(`{"pi": 3.14}`)},
			expectedResult: []Result{{Float64: 3.14, JSON: []byte(`{"pi": 3.14}`)}},
			expectError:    false,
			testFunc:       testAll,
		},
	}

	for _, tt := range tests {
		t.Run(tt.tpl.Name(), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			if err != nil {
				t.Fatalf("failed to open sqlmock database: %s", err)
			}
			defer db.Close()

			mock.ExpectQuery(tt.expectedSQL).WithArgs(tt.expectedArgs...).WillReturnRows(tt.mockRows)

			tt.testFunc(t, db, tt)

			if err := mock.ExpectationsWereMet(); err != nil {
				t.Errorf("there were unfulfilled expectations: %s", err)
			}
		})
	}
}

func testOne(t *testing.T, db *sql.DB, tc testCase) {
	result, err := sqlt.TypedQuery[Result, Param](tc.tpl).One(context.Background(), db, tc.params)

	if (err != nil) != tc.expectError {
		t.Fatalf("expected error: %v, got: %v", tc.expectError, err)
	}

	if !tc.expectError && !equal(result, tc.expectedResult) {
		t.Fatalf("expected: %v, got: %v", tc.expectedResult, result)
	}
}

func testAll(t *testing.T, db *sql.DB, tc testCase) {
	result, err := sqlt.TypedQuery[Result, Param](tc.tpl).All(context.Background(), db, tc.params)

	if (err != nil) != tc.expectError {
		t.Fatalf("expected error: %v, got: %v", tc.expectError, err)
	}

	if !tc.expectError && !equal(result, tc.expectedResult) {
		t.Fatalf("expected: %v, got: %v", tc.expectedResult, result)
	}
}

func equal(a, b interface{}) bool {
	return reflect.DeepEqual(a, b)
}

func BenchmarkSqltAll(b *testing.B) {
	t := sqlt.New("first").Dollar().MustParse(`
		SELECT {{ ScanInt64 Dest.Int64 "int64," -}} {{ ScanString Dest.String "string" }} FROM results WHERE test = {{ .String }}
	`)

	query := sqlt.TypedQuery[Result, Param](t)

	benchmarkAll(b, func(db *sql.DB, params Param) ([]Result, error) {
		return query.All(context.Background(), db, params)
	})
}

func BenchmarkSquirrelAll(b *testing.B) {
	sb := squirrel.Select("int64", "string").From("results").PlaceholderFormat(squirrel.Dollar)

	benchmarkAll(b, func(db *sql.DB, params Param) ([]Result, error) {
		query := sb.Where("test = ?", params.String)

		var (
			arr []Result
			res Result
		)

		rows, err := query.RunWith(db).QueryContext(context.Background())
		if err != nil {
			return nil, err
		}

		defer rows.Close()

		for rows.Next() {
			if err = rows.Scan(&res.Int64, &res.String); err != nil {
				return nil, err
			}

			arr = append(arr, res)
		}

		if err = rows.Err(); err != nil {
			return arr, err
		}

		if err = rows.Close(); err != nil {
			return arr, err
		}

		return arr, err
	})
}

func benchmarkAll(b *testing.B, do func(db *sql.DB, param Param) ([]Result, error)) {
	db, mock, err := sqlmock.New()
	if err != nil {
		b.Fatalf("failed to open sqlmock database: %s", err)
	}

	defer db.Close()

	b.ResetTimer()

	for range b.N {
		mock.ExpectQuery(`SELECT int64, string FROM results WHERE test = \$1`).WithArgs("value").
			WillReturnRows(
				sqlmock.NewRows([]string{"int64", "string"}).
					AddRow(100, "hundred").
					AddRow(200, "two-hundred").
					AddRow(300, "three-hundred"),
			)

		res, err := do(db, Param{String: "value"})
		if err != nil {
			b.Fatal(err)
		}

		if !equal(res, []Result{{Int64: 100, String: "hundred"}, {Int64: 200, String: "two-hundred"}, {Int64: 300, String: "three-hundred"}}) {
			b.Fatal(res)
		}
	}
}
