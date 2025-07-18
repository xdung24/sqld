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
	"github.com/joho/godotenv"
)

var config Config
var db *sqlx.DB
var sq squirrel.StatementBuilderType
var totalWrites int

// main handles some flag defaults,
// connects to the database,
// and starts the http server.
func main() {
	fmt.Println(`*** Note: This application is only for development environment. ***
*** Do not use it in production without additional security (ssl, authentication, rate limit) ***`)

	log.SetOutput(os.Stdout)

	// Overload environment variables from .env file
	if fileExists(".env") {
		log.Println("Loading .env file...")
		if err := godotenv.Overload(".env"); err != nil {
			log.Println("Error loading .env file")
		}
	} else {
		log.Println(".env file not found, using environment variables")
	}

	config = HandleFlags()
	if config.Debug {
		printInfo()
		config.print()
	}

	var err error

	defer CloseDB()
	db, sq, err = InitDB(config)
	if err != nil {
		log.Fatalf("Unable to connect to database: %s\n", err)
	}
	log.Println("Connected to the database.")

	// Load db backup from file if the database is sqlite3 memcache
	if config.CanBackup() && fileExists(config.SqliteBackup) {
		log.Println("Restoring database...")
		backupdb, _, err := InitSQLite(sqlx.Connect, "sqlite3", config.SqliteBackup)
		if err != nil {
			log.Fatalf("Unable to connect to backup database: %s\n", err)
		}
		err = backup(db, backupdb)
		if err != nil {
			log.Fatalf("Unable to restore database: %s\n", err)
		}
		err = backupdb.Close()
		if err != nil {
			log.Fatalf("Unable to close backup database: %s\n", err)
		}
		log.Println("Restore completed.")
	}

	// Create http server
	http.HandleFunc(config.Url, HandleQuery)
	server := &http.Server{Addr: fmt.Sprintf(":%d", config.Port)}

	// Signal handling
	sigs := make(chan os.Signal, 1)
	done := make(chan bool, 1)
	signal.Notify(sigs, syscall.SIGINT, syscall.SIGTERM)

	// Handle signals
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
		backupSqlite(config)

		done <- true
	}()

	// Run the timer to check if the database connection is still alive
	go selfDbCheck(time.Duration(config.HealthCheckInteval) * time.Minute)

	// Run timer to self health check
	go selfHealthCheck(time.Duration(config.HealthCheckInteval)*time.Minute, config)

	// Run the timer to save db to file if the database is sqlite3 memcache every 5 minutes
	go autoBackup(time.Duration(config.BackupInterval)*time.Minute, config)

	// Start the server
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

// Save db to file if the database is sqlite3 memcache
func backupSqlite(config Config) {
	if config.CanBackup() {
		log.Println("Backing up database...")
		backupdb, _, err := InitSQLite(sqlx.Connect, "sqlite3", config.SqliteBackup)
		if err != nil {
			log.Fatalf("Unable to connect to backup database: %s\n", err)
		}
		err = backup(backupdb, db)
		if err != nil {
			log.Fatalf("Unable to backup database: %s\n", err)
		}
		err = backupdb.Close()
		if err != nil {
			log.Fatalf("Unable to close backup database: %s\n", err)
		}
		backupdb = nil
		log.Println("Backup completed.")
	}
}

// AutoBackup periodically takes a snapshot and saves it to disk
func autoBackup(interval time.Duration, config Config) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Autobackup the database every 5 minutes if there are write operations
	for {
		select {
		case <-ticker.C:
			if totalWrites > 0 {
				totalWrites = 0
				backupSqlite(config)
			}
		case <-context.Background().Done():
			return
		}
	}
}

// Close app if lose connection to db
func selfDbCheck(interval time.Duration) {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Check if the database connection is still alive
	for {
		select {
		case <-ticker.C:
			if err := db.Ping(); err != nil {
				log.Println("Database connection lost, backing up database...")
				os.Exit(1)
			}
		case <-context.Background().Done():
			return
		}
	}
}

// selfHealthCheck periodically checks if the server is still alive
func selfHealthCheck(duration time.Duration, config Config) {
	if config.HealthCheckUrl == "" {
		return
	}

	ticker := time.NewTicker(duration)
	defer ticker.Stop()

	// Check if the server is still alive
	for {
		select {
		case <-ticker.C:
			if _, err := http.Get(config.HealthCheckUrl); err != nil {
				// Backoff for 5 seconds before retrying
				time.Sleep(5 * time.Second)
				if _, err := http.Get(config.HealthCheckUrl); err != nil {
					// Backup the database before exiting
					log.Println("Self health check failed, backing up database...")
					backupSqlite(config)
					os.Exit(1)
				}
			}
		case <-context.Background().Done():
			return
		}
	}
}
