package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"reflect"
	"strconv"
	"strings"
	"time"

	"github.com/Masterminds/squirrel"
	_ "github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	_ "github.com/lib/pq"
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

// Response is a generic response struct
type Response struct {
	Data  *interface{} `json:"data,omitempty"`
	Error *string      `json:"error,omitempty"`
}

// ExecResult is a generic response struct for exec queries
type ExecResult struct {
	RowsAffected int64 `json:"rows_affected"`
}

// EmptyArray is an empty array of maps
var EmptyArray = []map[string]string{}

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
		quotedKey := fmt.Sprintf("\"%s\"", key)
		if config.Dbtype == "mysql" {
			quotedKey = fmt.Sprintf("`%s`", key)
		}
		query = query.SetMap(squirrel.Eq{quotedKey: val})
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
func createSingle(table string, item map[string]interface{}) (interface{}, error) {
	columns := make([]string, len(item))
	values := make([]interface{}, len(item))

	i := 0
	for c, val := range item {
		if config.Dbtype == "mysql" {
			columns[i] = fmt.Sprintf("`%s`", c)
		} else {
			columns[i] = fmt.Sprintf("\"%s\"", c)
		}
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
	rowsAffected, _ := res.RowsAffected()
	return ExecResult{RowsAffected: rowsAffected}, nil
}

// create handles the POST method.
func create(r *http.Request) (interface{}, *SqldError) {
	// Read the request body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	// Parse the request body
	var data interface{}
	if err := json.Unmarshal(body, &data); err != nil {
		return nil, BadRequest(err)
	}

	// Map item data into interface and create a single item
	item, ok := data.(map[string]interface{})
	if ok {
		// Get the table name from the request path
		table, _, _ := parseRequest(r)
		saved, err := createSingle(table, item)
		if err != nil {
			return nil, BadRequest(err)
		}
		return saved, nil
	}

	return nil, BadRequest(errors.New("invalid request"))
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

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return nil, BadRequest(err)
	}

	return ExecResult{RowsAffected: rowsAffected}, nil
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
// Suport content type application/json and text/plain, application/json is default
// Type of action will be detected by the first word of the query
func raw(r *http.Request) (interface{}, *SqldError) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		return nil, BadRequest(err)
	}
	defer r.Body.Close()

	var query RawQuery

	// Read the sql query from the request body
	var contentType = r.Header.Get("Content-Type")
	if contentType == "text/plain" {
		query.SqlQuery = string(body)
		if query.SqlQuery == "" {
			return nil, BadRequest(errors.New("invalid raw query request"))
		}
	} else {
		err = json.Unmarshal(body, &query)
		if err != nil {
			return nil, BadRequest(err)
		}
	}

	// Execute the query
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
		rAffect, _ := res.RowsAffected()
		totalWrites++
		return ExecResult{RowsAffected: rAffect}, nil
	}

	// If the query type is unknown, return a bad request
	return nil, BadRequest(errors.New("unknown query type"))
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

func quoteMinimal(field string) string {
	if strings.ContainsAny(field, ",\"\n") {
		return strconv.Quote(field)
	}
	return field
}

// writeResponseCsv writes the response to the client in csv format
func writeResponseCsv(w http.ResponseWriter, data interface{}, err *SqldError) int {
	// If an error occurred, write the error to the response
	if err != nil {
		http.Error(w, err.Error(), err.Code)
		return err.Code
	}

	// If no data was returned, write a 200 OK response
	if data == nil {
		w.WriteHeader(http.StatusOK)
		return http.StatusOK
	}

	// write csv response
	w.Header().Set("Content-Type", "text/csv")
	rv := reflect.ValueOf(data)

	if rv.Kind() == reflect.Struct {
		w.WriteHeader(http.StatusOK)
		if result, ok := data.(ExecResult); ok {
			w.Write([]byte("rows_affected\n"))
			w.Write([]byte(fmt.Sprintf("%v\n", result.RowsAffected)))
		}
		return http.StatusOK
	}

	if rv.IsNil() {
		w.WriteHeader(http.StatusOK)
		return http.StatusOK
	}

	// if data is basic type, return 200 OK
	if rv.Kind() == reflect.String || rv.Kind() == reflect.Int || rv.Kind() == reflect.Float64 {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte(fmt.Sprintf("%v", data)))
		return http.StatusOK
	}

	// if data is a map, return 200 OK
	if rv.Kind() == reflect.Map {
		// get the headers
		w.WriteHeader(http.StatusOK)
		var headers []string
		for key := range data.(map[string]interface{}) {
			headers = append(headers, key)
		}
		w.Write([]byte(strings.Join(headers, ",") + "\n"))
		// get the values
		var row []string
		for _, header := range headers {
			val := data.(map[string]interface{})[header]
			valStr := fmt.Sprintf("%v", val)
			if val == nil {
				valStr = "null"
			}
			row = append(row, quoteMinimal(valStr))
		}
		w.Write([]byte(strings.Join(row, ",") + "\n"))
		return http.StatusOK
	}

	// if data is a slice or array, return 200 OK
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		// if data is empty, return 200 OK
		if rv.Len() == 0 {
			w.WriteHeader(http.StatusOK)
			return http.StatusOK
		}

		// get the first element of the slice to get the headers
		w.WriteHeader(http.StatusOK)
		var headers []string
		for key := range rv.Index(0).Interface().(map[string]interface{}) {
			headers = append(headers, key)
		}
		w.Write([]byte(strings.Join(headers, ",") + "\n"))

		// write the data
		for _, item := range data.([]map[string]interface{}) {
			var row []string
			for _, header := range headers {
				val := item[header]
				valStr := fmt.Sprintf("%v", val)
				if val == nil {
					valStr = "null"
				}
				row = append(row, quoteMinimal(valStr))
			}
			w.Write([]byte(strings.Join(row, ",") + "\n"))
		}

		return http.StatusOK
	}

	return http.StatusInternalServerError
}

// writeResponse writes the response to the client,
// accept 2 response types, text(csv) or json
// if request does not send accept header, default response is json
func writeResponse(w http.ResponseWriter, r *http.Request, data interface{}, err *SqldError) int {
	var acceptHeader = r.Header.Get("Accept")

	// accept csv
	if acceptHeader == "text/csv" {
		return writeResponseCsv(w, data, err)
	}

	// default response is json
	w.Header().Set("Content-Type", "application/json")

	// If an error occurred, write the error to the response
	if err != nil {
		errStr := err.Error()
		w.WriteHeader(err.Code)
		json.NewEncoder(w).Encode(Response{
			Error: &errStr,
		})
		return err.Code
	}

	// To ensure the response is always an array
	rv := reflect.ValueOf(data)
	if data == nil || (rv.Kind() != reflect.Struct && rv.IsNil()) {
		data = EmptyArray
	}

	// Write data to the response
	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(Response{
		Data: &data,
	})
	return http.StatusOK
}

// handleQuery routes the given request to the proper handler
// given the request method. If the request method matches
// no available handlers, it responds with a method not found
// status.
func HandleQuery(w http.ResponseWriter, r *http.Request) {
	var err *SqldError
	var data interface{}
	start := time.Now()

	if r.URL.Path == "/" {
		if config.AllowRaw && r.Method == "POST" {
			data, err = raw(r)
		} else {
			err = BadRequest(errors.New("invalid raw query request"))
		}
	} else {
		switch r.Method {
		case "GET":
			data, err = read(r)
		case "POST":
			data, err = create(r)
			if err != nil {
				totalWrites++
			}
		case "PUT":
			data, err = update(r)
			if err != nil {
				totalWrites++
			}
		case "DELETE":
			data, err = del(r)
			if err != nil {
				totalWrites++
			}
		default:
			err = &SqldError{http.StatusMethodNotAllowed, errors.New("MethodNotAllowed")}
		}
	}

	// Write the data to the response
	status := writeResponse(w, r, data, err)
	logRequest(r, status, start)
}

func HandleHealthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
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
