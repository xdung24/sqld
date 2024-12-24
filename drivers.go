package main

import (
	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"

	// Import the sql drivers
	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
	_ "github.com/mattn/go-sqlite3"
)

// SQLConnector provides a type alias for a db initialize function
type SQLConnector func(driverName, dataSourceName string) (*sqlx.DB, error)

// InitMySQL sets up squirrel and creates a MySQL connection
func InitMySQL(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}

// InitPostgres sets up squirrel and creates a Postgres connection
func InitPostgres(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Dollar)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}

// InitSQLite sets up squirrel and creates a SQLite connection
func InitSQLite(connect SQLConnector, dbtype, dsn string) (*sqlx.DB, squirrel.StatementBuilderType, error) {
	sq := squirrel.StatementBuilder.PlaceholderFormat(squirrel.Question)
	db, err := connect(dbtype, dsn)
	return db, sq, err
}
