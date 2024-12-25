package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/jmoiron/sqlx"
)

var config Config
var db *sqlx.DB
var sq squirrel.StatementBuilderType

// main handles some flag defaults,
// connects to the database,
// and starts the http server.
func main() {
	fmt.Println(`*** Note: This application is only for development environment. ***
*** Do not use it in production without additional security (ssl, authentication, rate limit) ***`)

	log.SetOutput(os.Stdout)

	config = HandleFlags()

	var err error

	defer CloseDB()
	db, sq, err = InitDB(config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %s\n", err)
	}
	log.Println("Connected to the database.")

	// Load db backup from file if the database is sqlite3 memcache
	if config.IsSqlite3Memcache() && fileExists(config.SqliteBackup) {
		log.Println("Restoring database...")
		backupdb, _, err := InitSQLite(sqlx.Connect, "sqlite3", config.SqliteBackup)
		if err != nil {
			log.Fatalf("Unable to connect to backup database: %s\n", err)
		}
		err = backup(db, backupdb)
		if err != nil {
			log.Fatalf("Unable to restore database: %s\n", err)
		}
		backupdb.Close()
		log.Println("Restore completed.")
	}

	http.HandleFunc(config.Url, HandleQuery)
	server := &http.Server{Addr: fmt.Sprintf(":%d", config.Port)}

	// Signal handling
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	go func() {
		sig := <-sigs
		log.Println()
		log.Println(sig)
		log.Println("Process killed, running cleanup...")

		// Gracefully shutdown the server
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(ctx); err != nil {
			log.Fatalf("Server forced to shutdown: %s", err)
		}

		// Save db to file if the database is sqlite3 memcache
		if config.IsSqlite3Memcache() {
			log.Println("Backing up database...")
			backupdb, _, err := InitSQLite(sqlx.Connect, "sqlite3", config.SqliteBackup)
			if err != nil {
				log.Fatalf("Unable to connect to backup database: %s\n", err)
			}
			err = backup(backupdb, db)
			if err != nil {
				log.Fatalf("Unable to backup database: %s\n", err)
			}
			backupdb.Close()
			log.Println("Backup completed.")
		}

		done <- true
	}()

	log.Printf("sqld listening on port %d", config.Port)
	if err := server.ListenAndServe(); err != http.ErrServerClosed {
		log.Fatalf("ListenAndServe(): %s", err)
	}

	<-done

	os.Exit(0)
}

// check if file exists, and is not a directory and can be read
func fileExists(filePath string) bool {
	fi, err := os.Stat(filePath)
	if err != nil {
		return false
	}
	if os.IsNotExist(err) {
		return false
	}
	if fi.IsDir() {
		return false
	}
	return true
}
