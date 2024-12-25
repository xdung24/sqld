package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
	sqlite3 "github.com/mattn/go-sqlite3"
)

// RawQuery wraps the request body of a raw sqld request
type RawQuery struct {
	SqlQuery string `json:"sql"`
}

// SqldError provides additional information on errors encountered
type SqldError struct {
	Code int
	Err  error
}

// Error is implemented to ensure SqldError conforms to the error
// interface
func (s *SqldError) Error() string {
	return s.Err.Error()
}

// NewError is a SqldError factory
func NewError(err error, code int) *SqldError {
	if err == nil {
		err = errors.New("")
	}
	return &SqldError{
		Code: code,
		Err:  err,
	}
}

// BadRequest builds a SqldError that represents a bad request
func BadRequest(err error) *SqldError {
	return NewError(err, http.StatusBadRequest)
}

// NotFound builds a SqldError that represents a not found error
func NotFound(err error) *SqldError {
	return NewError(err, http.StatusNotFound)
}

// InternalError builds a SqldError that represents an internal error
func InternalError(err error) *SqldError {
	return NewError(err, http.StatusInternalServerError)
}

func InitDB(config Config) (*sqlx.DB, squirrel.StatementBuilderType, error) {

	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	switch config.Dbtype {
	case "mysql":
		return InitMySQL(sqlx.Connect, config.Dbtype, config.Dsn)
	case "postgres":
		return InitPostgres(sqlx.Connect, config.Dbtype, config.Dsn)
	case "sqlite3":
		return InitSQLite(sqlx.Connect, config.Dbtype, config.Dsn)
	}
	return nil, sq, errors.New("Unsupported database type " + config.Dbtype)
}

func CloseDB() error {
	if db != nil {
		return db.Close()
	}
	return nil
}

func parseRequest(r *http.Request) (string, map[string][]string, string) {
	paths := strings.Split(strings.TrimPrefix(r.URL.Path, config.Url), "/")
	table := paths[0]
	id := ""
	if len(paths) > 1 {
		id = paths[1]
	}
	return table, r.URL.Query(), id
}

func buildSelectQuery(r *http.Request) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Select("*").From(table)

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		case "__offset__":
			offset, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Offset(uint64(offset))
			}
		case "__order_by__":
			query = query.OrderBy(val...)
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func buildUpdateQuery(r *http.Request, values map[string]interface{}) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Update("").Table(table)

	for key, val := range values {
		query = query.SetMap(squirrel.Eq{key: val})
	}

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func buildDeleteQuery(r *http.Request) (string, []interface{}, error) {
	table, args, id := parseRequest(r)
	query := sq.Delete("").From(table)

	if id != "" {
		query = query.Where(squirrel.Eq{"id": id})
	}

	for key, val := range args {
		switch key {
		case "__limit__":
			limit, err := strconv.Atoi(val[0])
			if err == nil {
				query = query.Limit(uint64(limit))
			}
		default:
			query = query.Where(squirrel.Eq{key: val})
		}
	}

	return query.ToSql()
}

func readQuery(sql string, args []interface{}) ([]map[string]interface{}, error) {
	rows, err := db.Query(sql, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	count := len(columns)
	var tableData []map[string]interface{}
	values := make([]interface{}, count)
	valuePtrs := make([]interface{}, count)

	for rows.Next() {
		for i := 0; i < count; i++ {
			valuePtrs[i] = &values[i]
		}
		err = rows.Scan(valuePtrs...)
		if err != nil {
			return nil, err
		}
		rowData := make(map[string]interface{})
		for i, col := range columns {
			val := values[i]
			b, ok := val.([]byte)

			var v interface{}
			if ok {
				v = string(b)
			} else {
				v = val
			}
			rowData[col] = v
		}
		tableData = append(tableData, rowData)
	}

	err = rows.Err()
	if err != nil {
		return nil, err
	}
	return tableData, nil
}

// read handles the GET request.
func read(r *http.Request) (interface{}, *SqldError) {
	sql, args, err := buildSelectQuery(r)
	if err != nil {
		return nil, BadRequest(err)
	}

	tableData, err := readQuery(sql, args)
	if err != nil {
		return nil, BadRequest(err)
	}
	return tableData, nil
}

// createSingle handles the POST method when only a single model
// is provided in the request body.
func createSingle(table string, item map[string]interface{}) (map[string]interface{}, error) {
	columns := make([]string, len(item))
	values := make([]interface{}, len(item))

	i := 0
	for c, val := range item {
		columns[i] = c
		values[i] = val
		i++
	}

	query := sq.Insert(table).
		Columns(columns...).
		Values(values...)

	sql, args, err := query.ToSql()
	if err != nil {
		return nil, err
	}

	res, err := db.Exec(sql, args...)
	if err != nil {
		return nil, err
	}
	id, err := res.LastInsertId()
	if err != nil {
		return nil, err
	}
	item["id"] = id
	return item, nil
}

// create handles the POST method.
func create(r *http.Request) (interface{}, *SqldError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, BadRequest(err)
	}

	table, _, _ := parseRequest(r)

	item, ok := data.(map[string]interface{})
	if ok {
		saved, err := createSingle(table, item)
		if err != nil {
			return nil, BadRequest(err)
		}
		return saved, nil
	}

	return nil, BadRequest(nil)
}

// update handles the PUT method.
func update(r *http.Request) (interface{}, *SqldError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var data map[string]interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, BadRequest(err)
	}

	sql, args, err := buildUpdateQuery(r, data)

	if err != nil {
		return nil, BadRequest(err)
	}

	return execQuery(sql, args)
}

// del handles the DELETE method.
func del(r *http.Request) (interface{}, *SqldError) {
	sql, args, err := buildDeleteQuery(r)

	if err != nil {
		return nil, BadRequest(err)
	}

	return execQuery(sql, args)
}

// execQuery will perform a sql query, return the appropriate error code
// given error states or return an http 204 NO CONTENT on success.
func execQuery(sql string, args []interface{}) (interface{}, *SqldError) {
	res, err := db.Exec(sql, args...)
	if err != nil {
		return nil, BadRequest(err)
	}

	rows, err := res.RowsAffected()
	if err != nil {
		return nil, BadRequest(err)
	}

	if res != nil && rows == 0 {
		return nil, BadRequest(err)
	}

	return nil, nil
}

// detectQueryType determines if the SQL query is a read or write operation
func detectQueryType(query string) string {
	var firstWord = strings.Split(query, " ")[0]
	var action = strings.TrimSpace(strings.ToUpper(firstWord))
	if action == "SELECT" ||
		action == "SHOW" ||
		action == "DESCRIBE" ||
		action == "EXPLAIN" ||
		action == "DESC" ||
		action == "PRAGMA" {
		return "read"
	}
	if action == "INSERT" ||
		action == "UPDATE" ||
		action == "DELETE" ||
		action == "CREATE" ||
		action == "DROP" ||
		action == "ALTER" {
		return "write"
	}
	return "unknown"
}

// Execute a raw query
func raw(r *http.Request) (interface{}, *SqldError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var query RawQuery
	err = json.Unmarshal(body, &query)
	if err != nil {
		return nil, BadRequest(err)
	}

	var noArgs []interface{}
	var queryType = detectQueryType(query.SqlQuery)
	if queryType == "read" {
		tableData, err := readQuery(query.SqlQuery, noArgs)
		if err != nil {
			return nil, BadRequest(err)
		}
		return tableData, nil
	}
	if queryType == "write" {
		res, err := db.Exec(query.SqlQuery, noArgs...)
		if err != nil {
			return nil, BadRequest(err)
		}
		lastID, _ := res.LastInsertId()
		rAffect, _ := res.RowsAffected()
		return map[string]interface{}{
			"last_insert_id": lastID,
			"rows_affected":  rAffect,
		}, nil
	}
	return nil, BadRequest(nil)
}

func logRequest(r *http.Request, status int, start time.Time) {
	elapsed := time.Since(start)
	var elapsedStr string
	if elapsed < time.Millisecond {
		elapsedStr = fmt.Sprintf("%d µs", elapsed.Microseconds())
	} else if elapsed < time.Second {
		elapsedStr = fmt.Sprintf("%d ms", elapsed.Milliseconds())
	} else {
		elapsedStr = fmt.Sprintf("%.2f s", elapsed.Seconds())
	}
	log.Printf(
		"%d %s %s %s",
		status,
		r.Method,
		r.URL.String(),
		elapsedStr,
	)
}

// handleQuery routes the given request to the proper handler
// given the request method. If the request method matches
// no available handlers, it responds with a method not found
// status.
func HandleQuery(w http.ResponseWriter, r *http.Request) {
	var err *SqldError
	var data interface{}
	status := http.StatusOK
	start := time.Now()

	if r.URL.Path == "/" {
		if config.AllowRaw && r.Method == "POST" {
			data, err = raw(r)
		} else {
			err = BadRequest(nil)
		}
	} else {
		switch r.Method {
		case "GET":
			data, err = read(r)
		case "POST":
			data, err = create(r)
		case "PUT":
			data, err = update(r)
		case "DELETE":
			data, err = del(r)
		default:
			err = &SqldError{http.StatusMethodNotAllowed, errors.New("MethodNotAllowed")}
		}
	}

	// If an error occurred, write the error to the response
	if err != nil {
		http.Error(w, err.Error(), err.Code)
		logRequest(r, err.Code, start)
		return
	}

	// If no data was returned, write a 204 NO CONTENT response
	if data == nil {
		status := http.StatusNoContent
		w.WriteHeader(status)
		logRequest(r, status, start)
		return
	}

	// Write the data to the response
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
	logRequest(r, http.StatusOK, start)
}

// backup copies the contents of the source database to the destination database using the SQLite backup API.
func backup(destDb, srcDb *sqlx.DB) error {
	destConn, err := destDb.Conn(context.Background())
	if err != nil {
		return err
	}
	defer destConn.Close()

	srcConn, err := srcDb.Conn(context.Background())
	if err != nil {
		return err
	}
	defer srcConn.Close()

	return destConn.Raw(func(destConn interface{}) error {
		return srcConn.Raw(func(srcConn interface{}) error {
			destSQLiteConn, ok := destConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("can't convert destination connection to SQLiteConn")
			}

			srcSQLiteConn, ok := srcConn.(*sqlite3.SQLiteConn)
			if !ok {
				return fmt.Errorf("can't convert source connection to SQLiteConn")
			}

			b, err := destSQLiteConn.Backup("main", srcSQLiteConn, "main")
			if err != nil {
				return fmt.Errorf("error initializing SQLite backup: %w", err)
			}

			done, err := b.Step(-1)
			if !done {
				return fmt.Errorf("step of -1, but not done")
			}
			if err != nil {
				return fmt.Errorf("error in stepping backup: %w", err)
			}

			err = b.Finish()
			if err != nil {
				return fmt.Errorf("error finishing backup: %w", err)
			}

			return err
		})
	})
}
