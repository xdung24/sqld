package main

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/pflag"
	"github.com/spf13/viper"
	"github.com/subosito/gotenv"
)

const (
	mysqlDSNTemplate       = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate    = "postgres://%s:%s@%s/%s?sslmode=disable"
	postgresSchemaTemplate = "postgres://%s:%s@%s/%s?sslmode=disable&search_path=%s"
)

type Config struct {
	AllowRaw           bool   // allow raw sql queries
	Dsn                string // database source name
	User               string // database username
	Pass               string // database password
	Host               string // database host
	Dbtype             string // database type
	Dbname             string // database name
	Schema             string // database schema (PostgreSQL only)
	Port               int    // server http port
	Url                string // url prefix
	HealthCheckUrl     string // health check url
	HealthCheckInteval int    // health check interval
	Debug              bool   // debug mode
}

// Order of precedence: command line flag > .env file > environment variable > default value
func parseConfig() Config {
	// If has -v flag, print version and exit
	if len(os.Args) > 1 && (os.Args[1] == "-v" || os.Args[1] == "--version") {
		printInfo()
		os.Exit(0)
	}

	// Load .env file first
	_ = gotenv.Load()

	// Set up viper
	v := viper.New()

	// Set defaults
	v.SetDefault("allowraw", false)
	v.SetDefault("dsn", "")
	v.SetDefault("user", "root")
	v.SetDefault("pass", "")
	v.SetDefault("host", "")
	v.SetDefault("dbtype", "sqlite3")
	v.SetDefault("dbname", "")
	v.SetDefault("schema", "public")
	v.SetDefault("port", 8080)
	v.SetDefault("url", "/")
	v.SetDefault("healthcheckurl", "")
	v.SetDefault("healthcheckinterval", 1)
	v.SetDefault("debug", false)

	// Bind environment variables
	v.AutomaticEnv()
	v.BindEnv("allowraw", "ALLOW_RAW")
	v.BindEnv("dsn", "DSN")
	v.BindEnv("user", "DB_USER")
	v.BindEnv("pass", "DB_PASS")
	v.BindEnv("host", "DB_HOST")
	v.BindEnv("dbtype", "DB_TYPE")
	v.BindEnv("dbname", "DB_NAME")
	v.BindEnv("schema", "DB_SCHEMA")
	v.BindEnv("port", "PORT")
	v.BindEnv("url", "URL")
	v.BindEnv("healthcheckurl", "HEALTH_CHECK_URL")
	v.BindEnv("healthcheckinterval", "HEALTH_CHECK_INTERVAL")
	v.BindEnv("debug", "DEBUG")

	// Define flags
	pflag.Bool("raw", v.GetBool("allowraw"), "allow raw sql queries")
	pflag.String("dsn", v.GetString("dsn"), "database source name")
	pflag.StringP("user", "u", v.GetString("user"), "database username")
	pflag.StringP("pass", "p", v.GetString("pass"), "database password")
	pflag.StringP("host", "h", v.GetString("host"), "database host")
	pflag.String("type", v.GetString("dbtype"), "database type")
	pflag.String("db", v.GetString("dbname"), "database name")
	pflag.String("schema", v.GetString("schema"), "database schema (PostgreSQL only)")
	pflag.Int("port", v.GetInt("port"), "http port")
	pflag.String("url", v.GetString("url"), "url prefix")
	pflag.String("healthCheckUrl", v.GetString("healthcheckurl"), "health check url")
	pflag.Int("healthCheckInterval", v.GetInt("healthcheckinterval"), "health check interval (minutes)")
	pflag.Bool("debug", v.GetBool("debug"), "debug mode")

	pflag.Parse()

	// Bind flags to viper
	v.BindPFlags(pflag.CommandLine)

	return Config{
		AllowRaw:           v.GetBool("raw"),
		Dsn:                v.GetString("dsn"),
		User:               v.GetString("user"),
		Pass:               v.GetString("pass"),
		Host:               v.GetString("host"),
		Dbtype:             v.GetString("type"),
		Dbname:             v.GetString("db"),
		Schema:             v.GetString("schema"),
		Port:               v.GetInt("port"),
		Url:                v.GetString("url"),
		HealthCheckUrl:     v.GetString("healthCheckUrl"),
		HealthCheckInteval: v.GetInt("healthCheckInterval"),
		Debug:              v.GetBool("debug"),
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
		switch c.Dbtype {
		case "postgres":
			c.Host = "localhost:5432"
		case "mysql":
			c.Host = "localhost:3306"
		default:
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
		if c.Schema != "" && c.Schema != "public" {
			c.Dsn = fmt.Sprintf(postgresSchemaTemplate, c.User, c.Pass, c.Host, c.Dbname, c.Schema)
		} else {
			c.Dsn = fmt.Sprintf(postgresDSNTemplate, c.User, c.Pass, c.Host, c.Dbname)
		}
	case "sqlite3":
		c.Dsn = "file::memory:"
	default:
		panic("Unsupported database type " + c.Dbtype)
	}
}

// Print the usage message and exit
func usage() {
	var usageMessage = `Usage of 'sqld':
    sqld [options]

Options:
  -u, --user           Database username (default: root)
  -p, --pass           Database password
  -h, --host           Database host (default: localhost)
  -type, --dbtype      Database type (default: sqlite3, other supported: mysql, postgres)
  -db, --dbname        Database name
  -schema, --schema    Database schema (PostgreSQL only, default: public)
  -port, --port        HTTP port (default: 8080)
  -url, --url          URL prefix (default: /)
  -dsn, --dsn          Database source name (default: file::memory:)
  -raw, --allowRaw     Allow raw SQL queries (default: false)
  -healthCheckUrl      Health check URL (default: http://localhost:8080/health)
  -healthCheckInterval Health check interval in minutes (default: 1)
  -debug               Debug mode (default: false)
  -v                   Print version and exit
  
Example:
  sqld -u root -p password -db mydatabase -h localhost:3306 -type mysql -port 8080 -url /api
  sqld -u postgres -p password -db mydb -schema myschema -h localhost:5432 -type postgres -port 8080
`
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	pflag.PrintDefaults()
	os.Exit(1)
}

// HandleFlags parses the command line flags and returns the Config
func HandleFlags() Config {
	pflag.Usage = usage
	config := parseConfig()
	config.fixUrl()
	config.buildDSN()
	return config
}

func (config *Config) print() {
	fmt.Println("Config:")
	fmt.Println("AllowRaw:", config.AllowRaw)
	fmt.Println("Dsn:", config.Dsn)
	fmt.Println("User:", config.User)
	fmt.Println("Pass:", config.Pass)
	fmt.Println("Host:", config.Host)
	fmt.Println("Dbtype:", config.Dbtype)
	fmt.Println("Dbname:", config.Dbname)
	fmt.Println("Schema:", config.Schema)
	fmt.Println("Port:", config.Port)
	fmt.Println("Url:", config.Url)
	fmt.Println("HealthCheckUrl:", config.HealthCheckUrl)
	fmt.Println("HealthCheckInteval:", config.HealthCheckInteval)
}

// IsBaseUrl returns true if the url is the same as the base url or
// the base url is a prefix of the url
func (c *Config) IsBaseUrl(url string) bool {
	return url == c.Url || url+"/" == c.Url
}

// GetTableName returns the fully qualified table name for the database type
// For PostgreSQL, it includes schema prefix if not using public schema
func (c *Config) GetTableName(tableName string) string {
	if c.Dbtype == "postgres" && c.Schema != "" && c.Schema != "public" {
		return fmt.Sprintf("%s.%s", c.Schema, tableName)
	}
	return tableName
}
