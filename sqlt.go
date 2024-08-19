package sqlt

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io/fs"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
)

// DB defines the interface for database operations.
// It includes methods for querying and executing SQL commands with context support.
type DB interface {
	QueryContext(ctx context.Context, str string, args ...any) (*sql.Rows, error)
	QueryRowContext(ctx context.Context, str string, args ...any) *sql.Row
	ExecContext(ctx context.Context, str string, args ...any) (sql.Result, error)
}

// InTx executes a function within a database transaction.
// It begins a transaction, rolling back if the function returns an error or panics, and committing otherwise.
func InTx(ctx context.Context, opts *sql.TxOptions, db *sql.DB, do func(db DB) error) (err error) {
	var tx *sql.Tx

	tx, err = db.BeginTx(ctx, opts)
	if err != nil {
		return err
	}

	defer func() {
		if p := recover(); p != nil {
			tx.Rollback()
			panic(p)
		} else if err != nil {
			tx.Rollback()
		} else {
			err = tx.Commit()
		}
	}()

	return do(tx)
}

// Raw represents a raw SQL string that should not be escaped.
type Raw string

// Scanner represents a structure for mapping SQL query results into a destination variable.
type Scanner struct {
	Dest any
	Map  func() error
	SQL  string
}

// ScanError indicates an error during scanning caused by a nil destination value.
type ScanError struct {
	SQL string
}

// Error returns the error message for a ScanError.
func (e ScanError) Error() string {
	return fmt.Sprintf("Dest value at '%s' is <nil>", e.SQL)
}

// ScanJSON creates a Scanner for scanning JSON results into the specified destination.
func ScanJSON[T any](dest *T, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, ScanError{SQL: str}
	}

	var data []byte

	return Scanner{
		SQL:  str,
		Dest: &data,
		Map: func() error {
			var t T

			if err := json.Unmarshal(data, &t); err != nil {
				return err
			}

			*dest = t

			return nil
		},
	}, nil
}

// Scan creates a Scanner for scanning results into the specified destination.
func Scan[T any](dest *T, str string) (Scanner, error) {
	if dest == nil {
		return Scanner{}, ScanError{SQL: str}
	}

	return Scanner{
		SQL:  str,
		Dest: dest,
	}, nil
}

// Must returns the template if no error, otherwise it panics.
func Must(t *Template, err error) *Template {
	if err != nil {
		panic(err)
	}

	return t
}

// New creates a new SQL template with the specified name.
func New(name string) *Template {
	t := &Template{
		text: template.New(name).Funcs(template.FuncMap{
			"Dest": func() any {
				return nil
			},
			"Raw": func(str string) Raw {
				return Raw(str)
			},
			"Scan": func(dest sql.Scanner, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, ScanError{SQL: str}
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil {
					return Scanner{}, ScanError{SQL: str}
				}

				var data []byte

				return Scanner{
					SQL:  str,
					Dest: &data,
					Map: func() error {
						return json.Unmarshal(data, dest)
					},
				}, nil
			},
			"ScanString":  Scan[string],
			"ScanBytes":   Scan[[]byte],
			"ScanInt":     Scan[int],
			"ScanInt8":    Scan[int8],
			"ScanInt16":   Scan[int16],
			"ScanInt32":   Scan[int32],
			"ScanInt64":   Scan[int64],
			"ScanUint":    Scan[uint],
			"ScanUint8":   Scan[uint8],
			"ScanUint16":  Scan[uint16],
			"ScanUint32":  Scan[uint32],
			"ScanUint64":  Scan[uint64],
			"ScanBool":    Scan[bool],
			"ScanFloat32": Scan[float32],
			"ScanFloat64": Scan[float64],
			"ScanTime":    Scan[time.Time],
		}),
		placeholder: "?",
	}

	return t
}

// Template provides an enhanced SQL template system, extending text/template with additional features
// for generating and executing SQL queries. It supports custom placeholders, positional parameters,
// and pre- and post-execution hooks. Template instances can parse SQL strings, files, and patterns
// from various sources, and manage execution contexts using pooled Runner instances for efficiency.
type Template struct {
	text        *template.Template
	beforeRun   func(op Operation, runner *Runner)
	afterRun    func(err error, op Operation, runner *Runner) error
	pool        *sync.Pool
	placeholder string
	size        int
	positional  bool
}

// New initializes a new SQL template with the given name, inheriting the current template's settings.
func (t *Template) New(name string) *Template {
	return &Template{
		text:        t.text.New(name),
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}
}

// Name returns the name of the underlying template.
func (t *Template) Name() string {
	return t.text.Name()
}

// Placeholder configures the template to use a specified placeholder string and positional parameter mode.
func (t *Template) Placeholder(placeholder string, positional bool) *Template {
	t.placeholder = placeholder
	t.positional = positional

	return t
}

// Question sets the placeholder to a question mark (?) for the Template.
func (t *Template) Question() *Template {
	return t.Placeholder("?", false)
}

// Dollar sets the placeholder to a dollar sign ($) for the Template.
func (t *Template) Dollar() *Template {
	return t.Placeholder("$", true)
}

// Colon sets the placeholder to a colon (:) for the Template.
func (t *Template) Colon() *Template {
	return t.Placeholder(":", true)
}

// AtP sets the placeholder to @p for the Template.
func (t *Template) AtP() *Template {
	return t.Placeholder("@p", true)
}

// BeforeRun sets a function to be called before running the Template.
func (t *Template) BeforeRun(handle func(op Operation, runner *Runner)) *Template {
	t.beforeRun = handle

	return t
}

// AfterRun sets a function to be called after running the Template.
func (t *Template) AfterRun(handle func(err error, op Operation, runner *Runner) error) *Template {
	t.afterRun = handle

	return t
}

// Option sets options for the Template.
func (t *Template) Option(opt ...string) *Template {
	t.text.Option(opt...)

	return t
}

// Parse parses a SQL template from the provided string and returns the Template.
func (t *Template) Parse(str string) (*Template, error) {
	text, err := t.text.Parse(str)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

// MustParse parses the string into the Template and panics if an error occurs.
func (t *Template) MustParse(str string) *Template {
	return Must(t.Parse(str))
}

// ParseFS parses files from a filesystem into the Template.
func (t *Template) ParseFS(fsys fs.FS, patterns ...string) (*Template, error) {
	text, err := t.text.ParseFS(fsys, patterns...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

// MustParseFS ensures that files are parsed from a filesystem into the Template successfully or panics if there's an error.
func (t *Template) MustParseFS(fsys fs.FS, patterns ...string) *Template {
	return Must(t.ParseFS(fsys, patterns...))
}

// ParseFiles parses the specified files into the Template.
func (t *Template) ParseFiles(filenames ...string) (*Template, error) {
	text, err := t.text.ParseFiles(filenames...)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

// MustParseFiles ensures that the specified files are parsed into the Template successfully or panics if there's an error.
func (t *Template) MustParseFiles(filenames ...string) *Template {
	return Must(t.ParseFiles(filenames...))
}

// ParseGlob parses the specified glob pattern into the Template.
func (t *Template) ParseGlob(pattern string) (*Template, error) {
	text, err := t.text.ParseGlob(pattern)
	if err != nil {
		return nil, err
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

// MustParseGlob ensures that the glob pattern is parsed into the Template successfully or panics if there's an error.
func (t *Template) MustParseGlob(pattern string) *Template {
	return Must(t.ParseGlob(pattern))
}

// Funcs registers custom functions to the template, extending its capabilities.
func (t *Template) Funcs(fm template.FuncMap) *Template {
	t.text.Funcs(fm)

	return t
}

// Lookup retrieves a template by its name, returning an error if the template is not found.
func (t *Template) Lookup(name string) (*Template, error) {
	text := t.text.Lookup(name)
	if text == nil {
		return nil, fmt.Errorf("template name '%s' not found", name)
	}

	return &Template{
		text:        text,
		placeholder: t.placeholder,
		positional:  t.positional,
		beforeRun:   t.beforeRun,
		afterRun:    t.afterRun,
		pool:        t.pool,
	}, nil
}

// MustLookup ensures that a template with the specified name is found successfully or panics if there's an error.
func (t *Template) MustLookup(name string) *Template {
	return Must(t.Lookup(name))
}

// Runner is responsible for executing a SQL template. It holds the context,
// template, SQL buffer, arguments, destination values, and mapping functions
// necessary for processing and executing the SQL query.
type Runner struct {
	Context context.Context
	Text    *template.Template
	SQL     *SQL
	Value   any
	Args    []any
	Dest    []any
	Map     []func() error
}

type SQL struct {
	data      []byte
	nextSpace bool
}

func (s *SQL) Write(data []byte) (int, error) {
	for _, b := range data {
		if b == ' ' || b == '\t' || b == '\n' || b == '\r' {
			if !s.nextSpace {
				continue
			}
			s.data = append(s.data, ' ')
			s.nextSpace = false
		} else {
			s.data = append(s.data, b)
			s.nextSpace = true
		}
	}

	return len(data), nil
}

func (s *SQL) String() string {
	return string(s.data)
}

func (s *SQL) Len() int {
	return len(s.data)
}

func (s *SQL) Reset() {
	s.data = s.data[:0]
	s.nextSpace = false
}

// Run executes the SQL template with the provided context and parameters.
// It manages Runner instances from a pool for efficient resource reuse, and processes
// the template, mapping arguments and destinations. Optional hooks can be set for pre
// and post-execution. The Runner resets itself for reuse after execution.
func (t *Template) Run(ctx context.Context, op Operation, use func(runner *Runner) error) error {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner{
					Text: escape(text),
					SQL:  &SQL{data: make([]byte, 0, t.size)},
				}

				r.Text.Funcs(template.FuncMap{
					"Dest": func() any {
						return r.Value
					},
					ident: func(arg any) string {
						switch a := arg.(type) {
						case Scanner:
							r.Dest = append(r.Dest, a.Dest)
							r.Map = append(r.Map, a.Map)

							return a.SQL
						case Raw:
							return string(a)
						default:
							r.Args = append(r.Args, arg)

							if t.positional {
								return t.placeholder + strconv.Itoa(len(r.Args))
							}

							return t.placeholder
						}
					},
				})

				return r
			},
		}
	}

	switch r := t.pool.Get().(type) {
	case *Runner:
		r.Context = ctx

		if t.beforeRun != nil {
			t.beforeRun(op, r)
		}

		err := use(r)

		if t.afterRun != nil {
			err = t.afterRun(err, op, r)
		}

		go func() {
			if size := r.SQL.Len(); size > t.size {
				t.size = size
			}

			r.SQL.Reset()
			r.Args = r.Args[:0]
			r.Dest = r.Dest[:0]
			r.Map = r.Map[:0]

			t.pool.Put(r)
		}()

		return err
	case error:
		return r
	default:
		panic(r)
	}
}

type Operation string

const (
	ExecOperation       Operation = "Exec"
	QueryRowOperation   Operation = "QueryRow"
	QueryOperation      Operation = "Query"
	FetchAllOperation   Operation = "FetchAll"
	FetchOneOperation   Operation = "FetchOne"
	FetchFirstOperation Operation = "FetchFirst"
)

// Exec executes a SQL command using the Template and the given context and database.
func (t *Template) Exec(ctx context.Context, db DB, params any) (sql.Result, error) {
	var (
		result sql.Result
		err    error
	)

	err = t.Run(ctx, ExecOperation, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		result, err = db.ExecContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		return nil
	})

	return result, err
}

// Query runs a SQL query using the Template and the given context and database.
func (t *Template) Query(ctx context.Context, db DB, params any) (*sql.Rows, error) {
	var (
		rows *sql.Rows
		err  error
	)

	err = t.Run(ctx, QueryOperation, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		return nil
	})

	return rows, err
}

// QueryRow runs a SQL query that is expected to return a single row using the Template and the given context and database.
func (t *Template) QueryRow(ctx context.Context, db DB, params any) (*sql.Row, error) {
	var (
		row *sql.Row
		err error
	)

	err = t.Run(ctx, QueryRowOperation, func(r *Runner) error {
		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		row = db.QueryRowContext(ctx, r.SQL.String(), r.Args...)

		return nil
	})

	return row, err
}

// FetchAll retrieves all rows of the query result into a slice. It uses the Template to
// generate the SQL query, executes it against the given database, and collects each
// resulting row into a slice. If any error occurs during the process, it is returned.
// Note: `Dest` must not be a pointer to a struct.
func FetchAll[Dest any](ctx context.Context, t *Template, db DB, params any) ([]Dest, error) {
	var (
		values []Dest
		dest   Dest
		err    error
	)

	err = t.Run(ctx, FetchAllOperation, func(r *Runner) error {
		var rows *sql.Rows

		r.Value = &dest

		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		if len(r.Dest) == 0 {
			r.Dest = append(r.Dest, &dest)
		}

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		defer rows.Close()

		for rows.Next() {
			if err = rows.Scan(r.Dest...); err != nil {
				return err
			}

			for _, m := range r.Map {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					return err
				}
			}

			values = append(values, dest)
		}

		if err = rows.Err(); err != nil {
			return err
		}

		if err = rows.Close(); err != nil {
			return err
		}

		return nil
	})

	return values, err
}

// ErrTooManyRows is an error that is returned when a query expected to return a single row
// returns more than one row. This error helps in ensuring that functions which are designed
// to fetch a single row can handle cases where the query result contains multiple rows.
var ErrTooManyRows = fmt.Errorf("sqlt: too many rows")

// FetchOne retrieves exactly one row of the query result and returns an error if more
// than one row is found. It uses the Template to generate the SQL query, executes it
// against the given database, and ensures only one resulting row is returned. If no
// rows are found or more than one row is found, it returns an error.
// Note: `Dest` must not be a pointer to a struct.
func FetchOne[Dest any](ctx context.Context, t *Template, db DB, params any) (Dest, error) {
	var (
		dest Dest
		err  error
	)

	err = t.Run(ctx, FetchOneOperation, func(r *Runner) error {
		var rows *sql.Rows

		r.Value = &dest

		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		if len(r.Dest) == 0 {
			r.Dest = append(r.Dest, &dest)
		}

		rows, err = db.QueryContext(ctx, r.SQL.String(), r.Args...)
		if err != nil {
			return err
		}

		defer rows.Close()

		if !rows.Next() {
			return sql.ErrNoRows
		}

		if err = rows.Scan(r.Dest...); err != nil {
			return err
		}

		for _, m := range r.Map {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return err
			}
		}

		if rows.Next() {
			return ErrTooManyRows
		}

		if err = rows.Err(); err != nil {
			return err
		}

		if err = rows.Close(); err != nil {
			return err
		}

		return nil
	})

	return dest, err
}

// FetchFirst retrieves the first row of the query result. It uses the Template to
// generate the SQL query, executes it against the given database, and returns the
// first resulting row. If any error occurs during the process, it is returned.
// Note: `Dest` must not be a pointer to a struct.
func FetchFirst[Dest any](ctx context.Context, t *Template, db DB, params any) (Dest, error) {
	var (
		dest Dest
		err  error
	)

	err = t.Run(ctx, FetchFirstOperation, func(r *Runner) error {
		r.Value = &dest

		if err = r.Text.Execute(r.SQL, params); err != nil {
			return err
		}

		if len(r.Dest) == 0 {
			r.Dest = append(r.Dest, &dest)
		}

		if err = db.QueryRowContext(ctx, r.SQL.String(), r.Args...).Scan(r.Dest...); err != nil {
			return err
		}

		for _, m := range r.Map {
			if m == nil {
				continue
			}

			if err = m(); err != nil {
				return err
			}
		}

		return nil
	})

	return dest, err
}

var ident = "__sqlt__"

// stolen from here: https://github.com/mhilton/sqltemplate/blob/main/escape.go
func escape(text *template.Template) *template.Template {
	for _, tpl := range text.Templates() {
		escapeTree(tpl.Tree)
	}

	return text
}

func escapeTree(s *parse.Tree) *parse.Tree {
	if s.Root == nil {
		return s
	}

	escapeNode(s, s.Root)

	return s
}

// escapeNode traverses and modifies a parse.Node to add necessary escaping logic for template processing.
func escapeNode(s *parse.Tree, n parse.Node) {
	switch v := n.(type) {
	case *parse.ActionNode:
		escapeNode(s, v.Pipe)
	case *parse.IfNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	case *parse.ListNode:
		if v == nil {
			return
		}

		for _, n := range v.Nodes {
			escapeNode(s, n)
		}
	case *parse.PipeNode:
		if len(v.Decl) > 0 {
			return
		}

		if len(v.Cmds) < 1 {
			return
		}

		cmd := v.Cmds[len(v.Cmds)-1]
		if len(cmd.Args) == 1 && cmd.Args[0].Type() == parse.NodeIdentifier && cmd.Args[0].(*parse.IdentifierNode).Ident == ident {
			return
		}

		v.Cmds = append(v.Cmds, &parse.CommandNode{
			NodeType: parse.NodeCommand,
			Args:     []parse.Node{parse.NewIdentifier(ident).SetTree(s).SetPos(cmd.Pos)},
		})
	case *parse.RangeNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	case *parse.WithNode:
		escapeNode(s, v.List)
		escapeNode(s, v.ElseList)
	}
}
