package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"strings"
)

const (
	mysqlDSNTemplate    = "%s:%s@(%s)/%s?parseTime=true"
	postgresDSNTemplate = "postgres://%s:%s@%s/%s?sslmode=disable"
)

type Config struct {
	AllowRaw           bool   // allow raw sql queries
	Dsn                string // database source name
	User               string // database username
	Pass               string // database password
	Host               string // database host
	Dbtype             string // database type
	Dbname             string // database name
	Port               int    // server http port
	Url                string // url prefix
	SqliteBackup       string // sqlite backup file
	HealthCheckUrl     string // health check url
	HealthCheckInteval int    // health check interval
	BackupInterval     int    // backup interval
}

// Order of precedence: command line flag > environment variable > default value
func parseConfig() Config {
	var allowRaw = flag.Bool("raw", getEnvAsBool("ALLOW_RAW", false), "allow raw sql queries")
	var dsn = flag.String("dsn", getEnv("DSN", ""), "database source name")
	var user = flag.String("u", getEnv("DB_USER", "root"), "database username")
	var pass = flag.String("p", getEnv("DB_PASS", ""), "database password")
	var host = flag.String("h", getEnv("DB_HOST", ""), "database host")
	var dbtype = flag.String("type", getEnv("DB_TYPE", "sqlite3"), "database type")
	var dbname = flag.String("db", getEnv("DB_NAME", ""), "database name")
	var port = flag.Int("port", getEnvAsInt("PORT", 8080), "http port")
	var url = flag.String("url", getEnv("URL", "/"), "url prefix")
	var sqliteBackup = flag.String("sqliteBackup", getEnv("SQLITE_BACKUP", ""), "sqlite backup file")
	var healthCheckUrl = flag.String("healthCheckUrl", getEnv("HEALTH_CHECK_URL", ""), "health check url")
	var healthCheckInterval = flag.Int("healthCheckInterval", getEnvAsInt("HEALTH_CHECK_INTERVAL", 1), "health check interval (minutes)")
	var backupInterval = flag.Int("backupInterval", getEnvAsInt("BACKUP_INTERVAL", 5), "backup interval - only for sqlite memory (minutes)")

	flag.Parse()

	return Config{
		AllowRaw:           *allowRaw,
		Dsn:                *dsn,
		User:               *user,
		Pass:               *pass,
		Host:               *host,
		Dbtype:             *dbtype,
		Dbname:             *dbname,
		Port:               *port,
		Url:                *url,
		SqliteBackup:       *sqliteBackup,
		HealthCheckUrl:     *healthCheckUrl,
		HealthCheckInteval: *healthCheckInterval,
		BackupInterval:     *backupInterval,
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
  -port, --port        HTTP port (default: 8080)
  -url, --url          URL prefix (default: /)
  -dsn, --dsn          Database source name (default: file::memory:)
  -raw, --allowRaw     Allow raw SQL queries (default: false)
  -sqliteBackup        SQLite backup file when using sqlite3 memcache (default: db.sqlite)
  -healthCheckUrl      Health check URL (default: http://localhost:8080/health)
  -healthCheckInterval Health check interval in minutes (default: 1)
  -backupInterval      Backup interval in minutes (default: 5)

Example:
  sqld -u root -p password -db mydatabase -h localhost:3306 -type mysql -port 8080 -url /api
`
	fmt.Fprintln(os.Stderr, usageMessage)
	fmt.Fprintln(os.Stderr, "Flags:")
	flag.PrintDefaults()
	os.Exit(1)
}

// HandleFlags parses the command line flags and returns the Config
func HandleFlags() Config {
	flag.Usage = usage
	config := parseConfig()
	config.fixUrl()
	config.buildDSN()
	return config
}

// CanBackup returns true if the database is sqlite3 and sqlite backup file is set
func (c *Config) CanBackup() bool {
	return c.Dbtype == "sqlite3" && c.SqliteBackup != ""
}

func getEnv(key string, defaultValue string) string {
	if value, exists := os.LookupEnv(key); exists {
		return value
	}
	return defaultValue
}

func getEnvAsInt(name string, defaultValue int) int {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.Atoi(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}

func getEnvAsBool(name string, defaultValue bool) bool {
	if valueStr, exists := os.LookupEnv(name); exists {
		if value, err := strconv.ParseBool(valueStr); err == nil {
			return value
		}
	}
	return defaultValue
}
