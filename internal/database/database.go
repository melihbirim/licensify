package database

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"time"

	_ "github.com/lib/pq"
	_ "modernc.org/sqlite"
)

// DB wraps sql.DB and provides database operations
type DB struct {
	*sql.DB
	isPostgres bool
}

// Config holds database configuration
type Config struct {
	// SQLite options
	DBPath string

	// PostgreSQL options
	DatabaseURL string

	// Connection pool settings (PostgreSQL)
	MaxOpenConns    int
	MaxIdleConns    int
	ConnMaxLifetime time.Duration
	ConnMaxIdleTime time.Duration
}

// New creates a new database connection
func New(cfg Config) (*DB, error) {
	var sqlDB *sql.DB
	var err error
	var isPostgres bool
	var driverName, dataSource string

	// Detect database type
	if cfg.DatabaseURL != "" {
		// PostgreSQL
		driverName = "postgres"
		dataSource = cfg.DatabaseURL
		isPostgres = true
		log.Printf("📊 Using PostgreSQL database")
	} else {
		// SQLite
		driverName = "sqlite"
		dataSource = cfg.DBPath
		isPostgres = false
		log.Printf("📊 Using SQLite database: %s", cfg.DBPath)
	}

	sqlDB, err = sql.Open(driverName, dataSource)
	if err != nil {
		return nil, fmt.Errorf("failed to open database: %w", err)
	}

	// Test connection
	if err = sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("failed to ping database: %w", err)
	}

	db := &DB{
		DB:         sqlDB,
		isPostgres: isPostgres,
	}

	// Configure connection pool for PostgreSQL
	if isPostgres {
		maxOpen := cfg.MaxOpenConns
		if maxOpen == 0 {
			maxOpen = 25
		}
		maxIdle := cfg.MaxIdleConns
		if maxIdle == 0 {
			maxIdle = 5
		}
		connMaxLifetime := cfg.ConnMaxLifetime
		if connMaxLifetime == 0 {
			connMaxLifetime = 5 * time.Minute
		}
		connMaxIdleTime := cfg.ConnMaxIdleTime
		if connMaxIdleTime == 0 {
			connMaxIdleTime = 10 * time.Minute
		}

		sqlDB.SetMaxOpenConns(maxOpen)
		sqlDB.SetMaxIdleConns(maxIdle)
		sqlDB.SetConnMaxLifetime(connMaxLifetime)
		sqlDB.SetConnMaxIdleTime(connMaxIdleTime)
		log.Printf("📊 PostgreSQL connection pool configured (max_open=%d, max_idle=%d)", maxOpen, maxIdle)
	} else {
		// Enable WAL mode for SQLite for better concurrency
		pragmas := []string{
			"PRAGMA journal_mode=WAL;",   // Write-Ahead Logging for better concurrency
			"PRAGMA synchronous=NORMAL;", // Balance between safety and performance
			"PRAGMA foreign_keys=ON;",    // Enforce foreign key constraints
			"PRAGMA busy_timeout=5000;",  // Wait up to 5s if database is locked
			"PRAGMA cache_size=-64000;",  // 64MB cache
		}
		for _, pragma := range pragmas {
			if _, err := sqlDB.Exec(pragma); err != nil {
				log.Printf("⚠️  Failed to set SQLite pragma: %v", err)
			}
		}
		log.Printf("📊 SQLite WAL mode enabled for better concurrency")
	}

	return db, nil
}

// InitSchema loads and executes the database schema
func (db *DB) InitSchema() error {
	var schemaPath string
	if db.isPostgres {
		schemaPath = "sql/postgres/init.sql"
	} else {
		schemaPath = "sql/sqlite/init.sql"
	}

	schema, err := os.ReadFile(schemaPath)
	if err != nil {
		return fmt.Errorf("failed to read schema file %s: %w", schemaPath, err)
	}

	_, err = db.Exec(string(schema))
	if err != nil {
		return fmt.Errorf("failed to create schema: %w", err)
	}

	return nil
}

// Placeholder returns the correct SQL placeholder for the database type
// n is the parameter number (1-indexed)
func (db *DB) Placeholder(n int) string {
	if db.isPostgres {
		return fmt.Sprintf("$%d", n)
	}
	return "?"
}

// IsPostgres returns true if the database is PostgreSQL
func (db *DB) IsPostgres() bool {
	return db.isPostgres
}
