package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Database env keys (contract: specs/002-postgres-gorm-migrations/contracts/environment.md).
const (
	EnvDatabaseURL          = "JOBHOUND_DATABASE_URL"
	EnvMigrateDatabaseURL   = "JOBHOUND_MIGRATE_DATABASE_URL"
	EnvDBMaxOpenConns       = "JOBHOUND_DB_MAX_OPEN_CONNS"
	EnvDBMaxIdleConns       = "JOBHOUND_DB_MAX_IDLE_CONNS"
	EnvDBConnMaxLifetimeSec = "JOBHOUND_DB_CONN_MAX_LIFETIME_SEC"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = time.Hour
)

// Database holds Postgres DSNs and pool tuning from the environment.
type Database struct {
	URL                string
	MigrateURL         string // optional migrate-only DSN override
	MaxOpenConns       int
	MaxIdleConns       int
	ConnMaxLifetimeSec int
}

// LoadDatabaseFromEnv reads database-related env vars. URL may be empty (callers decide if that is an error).
func LoadDatabaseFromEnv() Database {
	return Database{
		URL:                os.Getenv(EnvDatabaseURL),
		MigrateURL:         os.Getenv(EnvMigrateDatabaseURL),
		MaxOpenConns:       intFromEnv(EnvDBMaxOpenConns, defaultMaxOpenConns),
		MaxIdleConns:       intFromEnv(EnvDBMaxIdleConns, defaultMaxIdleConns),
		ConnMaxLifetimeSec: intFromEnv(EnvDBConnMaxLifetimeSec, int(defaultConnMaxLifetime/time.Second)),
	}
}

// MigrationDSN returns migrate DSN if set, otherwise primary database URL (may be empty).
func (d Database) MigrationDSN() string {
	if d.MigrateURL != "" {
		return d.MigrateURL
	}
	return d.URL
}

// MigrateDSNFromEnv returns the DSN golang-migrate should use (migrate URL override or primary URL).
func MigrateDSNFromEnv() (string, error) {
	d := LoadDatabaseFromEnv()
	s := d.MigrationDSN()
	if s == "" {
		return "", fmt.Errorf("set %s or %s", EnvMigrateDatabaseURL, EnvDatabaseURL)
	}
	return s, nil
}

// NormalizePool applies defaults and sanity bounds for GORM/sql.DB pool settings.
func (d Database) NormalizePool() (maxOpen, maxIdle, lifetimeSec int) {
	maxOpen = d.MaxOpenConns
	maxIdle = d.MaxIdleConns
	lifetimeSec = d.ConnMaxLifetimeSec
	if maxOpen < 1 {
		maxOpen = defaultMaxOpenConns
	}
	if maxIdle < 0 {
		maxIdle = defaultMaxIdleConns
	}
	if lifetimeSec < 1 {
		lifetimeSec = int(defaultConnMaxLifetime / time.Second)
	}
	return maxOpen, maxIdle, lifetimeSec
}

func intFromEnv(name string, defaultVal int) int {
	s := os.Getenv(name)
	if s == "" {
		return defaultVal
	}
	n, err := strconv.Atoi(s)
	if err != nil {
		return defaultVal
	}
	return n
}
