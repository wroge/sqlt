package sqlt

import (
	"context"
	stdsql "database/sql"
	"fmt"
	"strings"
)

type DB interface {
	QueryContext(ctx context.Context, sql string, args ...any) (*stdsql.Rows, error)
	QueryRowContext(ctx context.Context, sql string, args ...any) *stdsql.Row
	ExecContext(ctx context.Context, sql string, args ...any) (stdsql.Result, error)
}

func InTx(ctx context.Context, opts *stdsql.TxOptions, db *stdsql.DB, do func(db DB) error) error {
	var (
		tx  *stdsql.Tx
		err error
	)

	tx, err = db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return do(tx)
}

func Exec(ctx context.Context, db DB, t *Template, params any) (stdsql.Result, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	result, err := db.ExecContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s; args: %v; err: %w", strings.Join(strings.Fields(runner.SQL), " "), runner.Args, err)
	}

	return result, nil
}

func Query(ctx context.Context, db DB, t *Template, params any) (*stdsql.Rows, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	rows, err := db.QueryContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s; args: %v; err: %w", strings.Join(strings.Fields(runner.SQL), " "), runner.Args, err)
	}

	return rows, nil
}

func QueryRow(ctx context.Context, db DB, t *Template, params any) (*stdsql.Row, error) {
	runner, err := t.Run(params, nil)
	if err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, runner.SQL, runner.Args...)
	if err = row.Err(); err != nil {
		return nil, fmt.Errorf("sql: %s; args: %v; err: %w", strings.Join(strings.Fields(runner.SQL), " "), runner.Args, err)
	}

	return row, nil
}

func QueryAll[Dest any](ctx context.Context, db DB, t *Template, params any) ([]Dest, error) {
	var (
		values []Dest
		value  Dest
		err    error
	)

	runner, err := t.Run(params, &value)
	if err != nil {
		return nil, err
	}

	if len(runner.Dest) == 0 {
		runner.Dest = []any{&value}
	}

	rows, err := db.QueryContext(ctx, runner.SQL, runner.Args...)
	if err != nil {
		return nil, fmt.Errorf("sql: %s; args: %v; err: %w", strings.Join(strings.Fields(runner.SQL), " "), runner.Args, err)
	}

	defer rows.Close()

	for rows.Next() {
		if err = rows.Scan(runner.Dest...); err != nil {
			return nil, err
		}

		if runner.Map != nil {
			if err = runner.Map(); err != nil {
				return nil, err
			}
		}

		values = append(values, value)
	}

	if err = rows.Err(); err != nil {
		return nil, err
	}

	if err = rows.Close(); err != nil {
		return nil, err
	}

	return values, nil
}

func QueryFirst[Dest any](ctx context.Context, db DB, t *Template, params any) (Dest, error) {
	var value Dest

	runner, err := t.Run(params, &value)
	if err != nil {
		return value, err
	}

	if len(runner.Dest) == 0 {
		runner.Dest = []any{&value}
	}

	row := db.QueryRowContext(ctx, runner.SQL, runner.Args...)
	if err = row.Err(); err != nil {
		return value, fmt.Errorf("sql: %s; args: %v; err: %w", strings.Join(strings.Fields(runner.SQL), " "), runner.Args, err)
	}

	if err := row.Scan(runner.Dest...); err != nil {
		return value, err
	}

	if runner.Map != nil {
		if err := runner.Map(); err != nil {
			return value, err
		}
	}

	return value, nil
}
