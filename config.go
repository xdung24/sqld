package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const (
	mysqlDSNTemplate    = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate = "postgres://%s:%s@%s/%s?sslmode=disable"
)

type Config struct {
	AllowRaw bool   // allow raw sql queries
	Dsn      string // database source name
	User     string // database username
	Pass     string // database password
	Host     string // database host
	Dbtype   string // database type
	Dbname   string // database name
	Port     int    // server http port
	Url      string // url prefix
}

func parseConfig() Config {
	var allowRaw = flag.Bool("raw", false, "allow raw sql queries")
	var dsn = flag.String("dsn", "", "database source name")
	var user = flag.String("u", "root", "database username")
	var pass = flag.String("p", "", "database password")
	var host = flag.String("h", "", "database host")
	var dbtype = flag.String("type", "sqlite3", "database type")
	var dbname = flag.String("db", "", "database name")
	var port = flag.Int("port", 8080, "http port")
	var url = flag.String("url", "/", "url prefix")
	flag.Parse()

	return Config{
		AllowRaw: *allowRaw,
		Dsn:      *dsn,
		User:     *user,
		Pass:     *pass,
		Host:     *host,
		Dbtype:   *dbtype,
		Dbname:   *dbname,
		Port:     *port,
		Url:      *url,
	}
}

// Add slash to the end of the url and add slash to the beginning of the url
func (c *Config) fixUrl() {
	if !strings.HasSuffix(c.Url, "/") {
		c.Url += "/"
	}

	if !strings.HasPrefix(c.Url, "/") {
		c.Url = "/" + c.Url
	}
}

func (c *Config) buildDSN() {
	if c.Dsn != "" {
		return
	}

	if c.Host == "" {
		if c.Dbtype == "postgres" {
			c.Host = "localhost:5432"
		} else if c.Dbtype == "mysql" {
			c.Host = "localhost:3306"
		} else {
			c.Host = "localhost"
		}
	}

	if c.User == "" {
		c.User = "root"
	}

	switch c.Dbtype {
	case "mysql":
		c.Dsn = fmt.Sprintf(mysqlDSNTemplate, c.User, c.Pass, c.Host, c.Dbname)
	case "postgres":
		c.Dsn = fmt.Sprintf(postgresDSNTemplate, c.User, c.Pass, c.Host, c.Dbname)
	case "sqlite3":
		c.Dsn = "file::memory:?cache=shared"
	default:
		panic("Unsupported database type " + c.Dbtype)
	}
}

// Print the usage message and exit
func usage() {
	var usageMessage = `Usage of 'sqld':
    sqld [options]

Options:
  -u, --user       Database username (default: root)
  -p, --pass       Database password
  -h, --host       Database host (default: localhost)
  -type, --dbtype  Database type (default: sqlite3)
  -db, --dbname    Database name
  -port, --port    HTTP port (default: 8080)
  -url, --url      URL prefix (default: /)
  -dsn, --dsn      Database source name
  -raw, --allowRaw Allow raw SQL queries (default: false)

Example:
  sqld -u root -p password -db mydatabase -h localhost:3306 -type mysql -port 8080 -url /api
`
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(2)
}

// HandleFlags parses the command line flags and returns the Config
func HandleFlags() Config {
	flag.Usage = usage
	config := parseConfig()
	config.fixUrl()
	config.buildDSN()
	return config
}
