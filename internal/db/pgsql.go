package db

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormGetter returns the shared *gorm.DB. Callers must use getter().WithContext(ctx) on every query path.
type GormGetter func() *gorm.DB

const (
	envDatabaseURL = "JOBHOUND_DATABASE_URL"

	envMaxOpenConns    = "JOBHOUND_DB_MAX_OPEN_CONNS"
	envMaxIdleConns    = "JOBHOUND_DB_MAX_IDLE_CONNS"
	envConnMaxLifetime = "JOBHOUND_DB_CONN_MAX_LIFETIME_SEC"
)

const (
	defaultMaxOpenConns    = 25
	defaultMaxIdleConns    = 5
	defaultConnMaxLifetime = time.Hour
)

// NewGetter wraps a non-nil *gorm.DB for injection into storage constructors (tests may pass a shared test DB).
func NewGetter(gdb *gorm.DB) GormGetter {
	return func() *gorm.DB { return gdb }
}

// OpenFromEnv opens PostgreSQL via GORM using JOBHOUND_DATABASE_URL, applies pool settings, and pings with ctx.
func OpenFromEnv(ctx context.Context) (*gorm.DB, error) {
	dsn := os.Getenv(envDatabaseURL)
	if dsn == "" {
		return nil, fmt.Errorf("%s is not set", envDatabaseURL)
	}
	return Open(ctx, dsn)
}

// Open opens PostgreSQL via GORM, applies pool settings from env (optional) or defaults, and pings with ctx.
func Open(ctx context.Context, dsn string) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Error),
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	if err := configurePool(ctx, gdb); err != nil {
		return nil, err
	}
	return gdb, nil
}

func configurePool(ctx context.Context, gdb *gorm.DB) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	maxOpen := intFromEnv(envMaxOpenConns, defaultMaxOpenConns)
	maxIdle := intFromEnv(envMaxIdleConns, defaultMaxIdleConns)
	lifetimeSec := intFromEnv(envConnMaxLifetime, int(defaultConnMaxLifetime/time.Second))
	if maxOpen < 1 {
		maxOpen = defaultMaxOpenConns
	}
	if maxIdle < 0 {
		maxIdle = defaultMaxIdleConns
	}
	if lifetimeSec < 1 {
		lifetimeSec = int(defaultConnMaxLifetime / time.Second)
	}
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(lifetimeSec) * time.Second)
	return sqlDB.PingContext(ctx)
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
