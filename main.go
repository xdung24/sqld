package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/jmoiron/sqlx"
)

var config Config

// main handles some flag defaults,
// connects to the database,
// and starts the http server.
func main() {
	fmt.Println(`*** Note: This application is only for development environment. ***
*** Do not use it in production without additional security (ssl, authentication, rate limit) ***`)

	log.SetOutput(os.Stdout)

	config = HandleFlags()

	var err error
	defer closeDB()
	db, sq, err = InitDB(config, sqlx.Connect)
	if err != nil {
		log.Fatalf("Unable to connect to database: %s\n", err)
	}

	http.HandleFunc(config.Url, handleQuery)
	log.Printf("sqld listening on port %d", config.Port)
	log.Print(http.ListenAndServe(fmt.Sprintf(":%d", config.Port), nil))
}
