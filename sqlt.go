package sqlt

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"iter"
	"reflect"
	"strconv"
	"sync"
	"text/template"
	"text/template/parse"
	"time"
	"unsafe"
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
	SQL  string
	Dest any
	Map  func() error
}

type Slice[T any] []T

func (s Slice[T]) MarshalJSON() ([]byte, error) {
	return json.Marshal([]T(s))
}

func (s *Slice[T]) UnmarshalJSON(data []byte) error {
	var list []T

	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}

	*s = list

	return nil
}

var ErrNilDest = errors.New("sqlt: dest is nil")

// Scan creates a Scanner for scanning results into the specified destination.
func Scan[T any](dest *T, str string) (Scanner, error) {
	if dest == nil || reflect.ValueOf(dest).IsNil() {
		return Scanner{}, fmt.Errorf("%w at %s", ErrNilDest, str)
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
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, fmt.Errorf("%w at %s", ErrNilDest, str)
				}

				return Scanner{
					SQL:  str,
					Dest: dest,
				}, nil
			},
			"ScanJSON": func(dest json.Unmarshaler, str string) (Scanner, error) {
				if dest == nil || reflect.ValueOf(dest).IsNil() {
					return Scanner{}, fmt.Errorf("%w at %s", ErrNilDest, str)
				}

				var data []byte

				return Scanner{
					SQL:  str,
					Dest: &data,
					Map: func() error {
						if err := dest.UnmarshalJSON(data); err != nil {
							return fmt.Errorf("%w at %s", err, str)
						}

						return nil
					},
				}, nil
			},
			"ScanString":   Scan[string],
			"ScanBytes":    Scan[[]byte],
			"ScanInt":      Scan[int],
			"ScanInt8":     Scan[int8],
			"ScanInt16":    Scan[int16],
			"ScanInt32":    Scan[int32],
			"ScanInt64":    Scan[int64],
			"ScanUint":     Scan[uint],
			"ScanUint8":    Scan[uint8],
			"ScanUint16":   Scan[uint16],
			"ScanUint32":   Scan[uint32],
			"ScanUint64":   Scan[uint64],
			"ScanBool":     Scan[bool],
			"ScanFloat32":  Scan[float32],
			"ScanFloat64":  Scan[float64],
			"ScanTime":     Scan[time.Time],
			"ScanDuration": Scan[time.Duration],
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
	beforeRun   func(runner *Runner)
	afterRun    func(err error, runner *Runner) error
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
func (t *Template) BeforeRun(handle func(runner *Runner)) *Template {
	t.beforeRun = handle

	return t
}

// AfterRun sets a function to be called after running the Template.
func (t *Template) AfterRun(handle func(err error, runner *Runner) error) *Template {
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

func (r *Runner) Execute(param any) error {
	return r.Text.Execute(r.SQL, param)
}

type SQL struct {
	buf []byte
}

func (s *SQL) Write(data []byte) (int, error) {
	start, end := 0, 0
	bufLen := len(s.buf)

	for start < len(data) {
		// Skip leading whitespace
		for start < len(data) && (data[start] == ' ' || data[start] == '\n' || data[start] == '\r' || data[start] == '\t') {
			start++
		}

		// Find the end of the next word
		end = start
		for end < len(data) && !(data[end] == ' ' || data[end] == '\n' || data[end] == '\r' || data[end] == '\t') {
			end++
		}

		// Append word to buffer
		if start < end {
			wordLen := end - start

			// If the buffer has existing content, append a space before the new word
			if bufLen > 0 {
				s.buf = append(s.buf, ' ')
				bufLen++
			}

			s.buf = append(s.buf, data[start:end]...)
			bufLen += wordLen
		}

		// Move to the next word
		start = end
	}

	return len(data), nil
}

func (s *SQL) String() string {
	return *(*string)(unsafe.Pointer(&s.buf))
}

func (s *SQL) Len() int {
	return len(s.buf)
}

func (s *SQL) Reset() {
	s.buf = s.buf[:0]
}

func (t *Template) GetRunner(ctx context.Context) (*Runner, error) {
	if t.pool == nil {
		t.pool = &sync.Pool{
			New: func() any {
				text, err := t.text.Clone()
				if err != nil {
					return err
				}

				var r = &Runner{
					Text: escape(text),
					SQL: &SQL{
						buf: make([]byte, 0, t.size),
					},
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
			t.beforeRun(r)
		}

		return r, nil
	case error:
		return nil, r
	}

	return nil, errors.New("invalid runner")
}

func (t *Template) PutRunner(err error, r *Runner) error {
	if t.afterRun != nil {
		err = t.afterRun(err, r)
	}

	if size := r.SQL.Len(); size > t.size {
		t.size = size
	}

	r.SQL.Reset()
	r.Args = r.Args[:0]
	r.Dest = r.Dest[:0]
	r.Map = r.Map[:0]

	t.pool.Put(r)

	return err
}

// Exec executes a SQL command using the Template and the given context and database.
func (t *Template) Exec(ctx context.Context, db DB, param any) (sql.Result, error) {
	var result sql.Result

	runner, err := t.GetRunner(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = t.PutRunner(err, runner)
	}()

	if err = runner.Execute(param); err != nil {
		return nil, err
	}

	result, err = db.ExecContext(ctx, runner.SQL.String(), runner.Args...)
	if err != nil {
		return nil, err
	}

	return result, err
}

// Query runs a SQL query using the Template and the given context and database.
func (t *Template) Query(ctx context.Context, db DB, param any) (*sql.Rows, error) {
	var rows *sql.Rows

	runner, err := t.GetRunner(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = t.PutRunner(err, runner)
	}()

	if err = runner.Execute(param); err != nil {
		return nil, err
	}

	rows, err = db.QueryContext(ctx, runner.SQL.String(), runner.Args...)
	if err != nil {
		return nil, err
	}

	return rows, err
}

// QueryRow runs a SQL query that is expected to return a single row using the Template and the given context and database.
func (t *Template) QueryRow(ctx context.Context, db DB, param any) (*sql.Row, error) {
	runner, err := t.GetRunner(ctx)
	if err != nil {
		return nil, err
	}

	defer func() {
		err = t.PutRunner(err, runner)
	}()

	if err = runner.Execute(param); err != nil {
		return nil, err
	}

	row := db.QueryRowContext(ctx, runner.SQL.String(), runner.Args...)

	return row, err
}

func Param[Param any](tpl *Template) *ParamExecutor[Param] {
	return &ParamExecutor[Param]{
		tpl: tpl,
	}
}

type ParamExecutor[Param any] struct {
	tpl *Template
}

func (e *ParamExecutor[Param]) Query(ctx context.Context, db DB, param Param) (*sql.Rows, error) {
	return e.tpl.Query(ctx, db, param)
}

func (e *ParamExecutor[Param]) QueryRow(ctx context.Context, db DB, param Param) (*sql.Row, error) {
	return e.tpl.QueryRow(ctx, db, param)
}

func (e *ParamExecutor[Param]) Exec(ctx context.Context, db DB, param Param) (sql.Result, error) {
	return e.tpl.Exec(ctx, db, param)
}

func (e *ParamExecutor[Param]) RowsAffected(ctx context.Context, db DB, param Param) (int64, error) {
	result, err := e.tpl.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return result.RowsAffected()
}

func (e *ParamExecutor[Param]) LastInsertId(ctx context.Context, db DB, param Param) (int64, error) {
	result, err := e.tpl.Exec(ctx, db, param)
	if err != nil {
		return 0, err
	}

	return result.LastInsertId()
}

func DestParam[Dest, Param any](t *Template) *DestParamExecutor[Dest, Param] {
	return &DestParamExecutor[Dest, Param]{
		tpl: t,
	}
}

type DestParamExecutor[Dest, Param any] struct {
	tpl      *Template
	clone    func(Dest) (Dest, error)
	validate func(Dest) error
}

func (q *DestParamExecutor[Dest, Param]) Clone(c func(Dest) (Dest, error)) *DestParamExecutor[Dest, Param] {
	q.clone = c

	return q
}

func (q *DestParamExecutor[Dest, Param]) Validate(v func(Dest) error) *DestParamExecutor[Dest, Param] {
	q.validate = v

	return q
}

func (q *DestParamExecutor[Dest, Param]) Query(ctx context.Context, db DB, param any) func(func(Dest, error) bool) {
	var dest Dest

	return func(yield func(Dest, error) bool) {
		runner, err := q.tpl.GetRunner(ctx)
		if err != nil {
			yield(dest, err)

			return
		}

		runner.Value = &dest

		if err = runner.Execute(param); err != nil {
			yield(dest, err)

			return
		}

		if len(runner.Dest) == 0 {
			runner.Dest = append(runner.Dest, &dest)
		}

		rows, err := db.QueryContext(ctx, runner.SQL.String(), runner.Args...)
		if err != nil {
			yield(dest, err)

			return
		}

		defer func() {
			err = q.tpl.PutRunner(errors.Join(err, rows.Close()), runner)
		}()

		for rows.Next() {
			if err = rows.Scan(runner.Dest...); err != nil {
				yield(dest, err)

				return
			}

			for _, m := range runner.Map {
				if m == nil {
					continue
				}

				if err = m(); err != nil {
					yield(dest, err)

					return
				}
			}

			if q.clone != nil {
				dest, err = q.clone(dest)
				if err != nil {
					yield(dest, err)

					return
				}
			}

			if q.validate != nil {
				err = q.validate(dest)
				if err != nil {
					yield(dest, err)

					return
				}
			}

			if !yield(dest, nil) {
				return
			}
		}

		if err = rows.Err(); err != nil {
			yield(dest, err)

			return
		}

		if err = rows.Close(); err != nil {
			yield(dest, err)

			return
		}
	}
}

// All retrieves all rows of the query result into a slice. It uses the Template to
// generate the SQL query, executes it against the given database, and collects each
// resulting row into a slice. If any error occurs during the process, it is returned.
// Note: `Dest` must not be a pointer to a struct.
func (q *DestParamExecutor[Dest, Param]) All(ctx context.Context, db DB, param Param) ([]Dest, error) {
	var values = []Dest{}

	for dest, err := range q.Query(ctx, db, param) {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, nil
		}

		values = append(values, dest)
	}

	return values, nil
}

// ErrTooManyRows is an error that is returned when a query expected to return a single row
// returns more than one row. This error helps in ensuring that functions which are designed
// to  a single row can handle cases where the query result contains multiple rows.
var ErrTooManyRows = fmt.Errorf("sqlt: too many rows")

// One retrieves exactly one row of the query result and returns an error if more
// than one row is found. It uses the Template to generate the SQL query, executes it
// against the given database, and ensures only one resulting row is returned. If no
// rows are found or more than one row is found, it returns an error.
// Note: `Dest` must not be a pointer to a struct.
func (q *DestParamExecutor[Dest, Param]) One(ctx context.Context, db DB, param Param) (Dest, error) {
	next, stop := iter.Pull2(q.Query(ctx, db, param))

	defer stop()

	dest, err, ok := next()
	if err != nil {
		return dest, err
	}

	if !ok {
		return dest, sql.ErrNoRows
	}

	_, err, ok = next()
	if err != nil {
		return dest, err
	}

	if ok {
		return dest, ErrTooManyRows
	}

	return dest, nil
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
