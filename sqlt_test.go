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
	"github.com/wroge/sqlt"
)

type testCase struct {
	tpl            *sqlt.Template
	params         any
	mockRows       *sqlmock.Rows
	expectedResult interface{}
	expectedSQL    string
	expectedArgs   []driver.Value
	expectError    bool
	testFunc       func(t *testing.T, db *sql.DB, tc testCase)
}

type Result struct {
	Int     int
	Int64   int64
	Float64 float64
	String  string
	Bool    bool
	Time    time.Time
	JSON    json.RawMessage
}

var (
	date, _ = time.Parse(time.DateTime, "2010-11-20 08:10:30")
)

func TestStuff(t *testing.T) {
	tests := []testCase{
		{
			tpl:            sqlt.New("test-1").MustParse(`SELECT {{ index . 0 }} {{ ScanInt64 Dest.Int64 "AS int64" }}, {{ index . 1 }} {{ ScanString Dest.String "AS string" }}`),
			params:         []any{100, "hundered"},
			mockRows:       sqlmock.NewRows([]string{"int64", "string"}).AddRow(100, "hundered"),
			expectedSQL:    `SELECT \? AS int64, \? AS string`,
			expectedArgs:   []driver.Value{100, ("hundered")},
			expectedResult: Result{Int64: 100, String: "hundered"},
			expectError:    false,
			testFunc:       testFetchOne[Result],
		},
		{
			tpl:            sqlt.New("test-2").MustParse(`SELECT {{ index . 0 }} {{ ScanInt64 Dest.Int64 "AS int64" }}, {{ index . 1 }} {{ ScanString Dest.String "AS string" }}`),
			params:         []any{100, "hundered"},
			mockRows:       sqlmock.NewRows([]string{"int64", "string"}).AddRow(100, "hundered"),
			expectedSQL:    `SELECT \? AS int64, \? AS string`,
			expectedArgs:   []driver.Value{100, "hundered"},
			expectedResult: Result{Int64: 100, String: "hundered"},
			expectError:    false,
			testFunc:       testFetchFirst[Result],
		},
		{
			tpl:            sqlt.New("test-3").MustParse(`SELECT {{ index . 0 }} {{ ScanTime Dest.Time "AS time" }}, {{ index . 1 }} {{ ScanJSON Dest.JSON "AS json" }}`),
			params:         []any{date, []byte(`{"hundered": 100}`)},
			mockRows:       sqlmock.NewRows([]string{"time", "json"}).AddRow(date, []byte(`{"hundered": 100}`)),
			expectedSQL:    `SELECT \? AS time, \? AS json`,
			expectedArgs:   []driver.Value{date, []byte(`{"hundered": 100}`)},
			expectedResult: []Result{{Time: date, JSON: []byte(`{"hundered": 100}`)}},
			expectError:    false,
			testFunc:       testFetchAll[Result],
		},
		{
			tpl:            sqlt.New("test-4").MustParse(`SELECT {{ index . 00 }} {{ ScanInt Dest.Int "AS int" }}`),
			params:         []any{42},
			mockRows:       sqlmock.NewRows([]string{"int"}).AddRow(42),
			expectedSQL:    `SELECT \? AS int`,
			expectedArgs:   []driver.Value{42},
			expectedResult: Result{Int: 42},
			expectError:    false,
			testFunc:       testFetchOne[Result],
		},
		{
			tpl:            sqlt.New("test-5").MustParse(`SELECT {{ index . 0 }} {{ ScanString Dest.String "AS string" }}, {{ index . 1 }} {{ ScanBool Dest.Bool "AS bool" }}`),
			params:         []any{"example", true},
			mockRows:       sqlmock.NewRows([]string{"string", "bool"}).AddRow("example", true),
			expectedSQL:    `SELECT \? AS string, \? AS bool`,
			expectedArgs:   []driver.Value{"example", true},
			expectedResult: Result{String: "example", Bool: true},
			expectError:    false,
			testFunc:       testFetchFirst[Result],
		},
		{
			tpl:            sqlt.New("test-6").MustParse(`SELECT {{ index . 0 }} {{ ScanFloat64 Dest.Float64 "AS float64" }}, {{ index . 1 }} {{ ScanJSON Dest.JSON "AS json" }}`),
			params:         []any{3.14, []byte(`{"pi": 3.14}`)},
			mockRows:       sqlmock.NewRows([]string{"float64", "json"}).AddRow(3.14, []byte(`{"pi": 3.14}`)),
			expectedSQL:    `SELECT \? AS float64, \? AS json`,
			expectedArgs:   []driver.Value{3.14, []byte(`{"pi": 3.14}`)},
			expectedResult: []Result{{Float64: 3.14, JSON: []byte(`{"pi": 3.14}`)}},
			expectError:    false,
			testFunc:       testFetchAll[Result],
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

func testFetchOne[Dest any](t *testing.T, db *sql.DB, tc testCase) {
	result, err := sqlt.FetchOne[Dest](context.Background(), tc.tpl, db, tc.params)

	if (err != nil) != tc.expectError {
		t.Fatalf("expected error: %v, got: %v", tc.expectError, err)
	}

	if !tc.expectError && !equal(result, tc.expectedResult) {
		t.Fatalf("expected: %v, got: %v", tc.expectedResult, result)
	}
}

func testFetchAll[Dest any](t *testing.T, db *sql.DB, tc testCase) {
	result, err := sqlt.FetchAll[Dest](context.Background(), tc.tpl, db, tc.params)

	if (err != nil) != tc.expectError {
		t.Fatalf("expected error: %v, got: %v", tc.expectError, err)
	}

	if !tc.expectError && !equal(result, tc.expectedResult) {
		t.Fatalf("expected: %v, got: %v", tc.expectedResult, result)
	}
}

func testFetchFirst[Dest any](t *testing.T, db *sql.DB, tc testCase) {
	result, err := sqlt.FetchFirst[Dest](context.Background(), tc.tpl, db, tc.params)
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
