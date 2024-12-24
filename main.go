package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jmoiron/sqlx"
)

// main handles some flag defaults, connects to the database,
// and starts the http server.
func main() {
	log.SetOutput(os.Stdout)
	handleFlags()

	var err error
	defer closeDB()
	db, sq, err = initDB(sqlx.Connect)
	if err != nil {
		log.Fatalf("Unable to connect to database: %s\n", err)
	}

	http.HandleFunc(*url, handleQuery)
	log.Printf("sqld listening on port %d", *port)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%d", *port), nil))
}
