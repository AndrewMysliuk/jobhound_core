package pgsql

import (
	"context"
	"fmt"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// GormGetter returns the shared *gorm.DB. Callers must use getter().WithContext(ctx) on every query path.
type GormGetter func() *gorm.DB

// NewGetter wraps a non-nil *gorm.DB for injection into storage constructors (tests may pass a shared test DB).
func NewGetter(gdb *gorm.DB) GormGetter {
	return func() *gorm.DB { return gdb }
}

// OpenFromEnv opens PostgreSQL via GORM using config.LoadDatabaseFromEnv, applies pool settings, and pings with ctx.
func OpenFromEnv(ctx context.Context) (*gorm.DB, error) {
	cfg := config.LoadDatabaseFromEnv()
	if cfg.URL == "" {
		return nil, fmt.Errorf("%s is not set", config.EnvDatabaseURL)
	}
	return Open(ctx, cfg)
}

// Open opens PostgreSQL via GORM using the given database config, applies pool bounds, and pings with ctx.
func Open(ctx context.Context, dbcfg config.Database) (*gorm.DB, error) {
	gdb, err := gorm.Open(postgres.Open(dbcfg.URL), &gorm.Config{
		Logger:         logger.Default.LogMode(logger.Error),
		TranslateError: true,
	})
	if err != nil {
		return nil, err
	}
	if err := configurePool(ctx, gdb, dbcfg); err != nil {
		return nil, err
	}
	return gdb, nil
}

func configurePool(ctx context.Context, gdb *gorm.DB, dbcfg config.Database) error {
	sqlDB, err := gdb.DB()
	if err != nil {
		return err
	}
	maxOpen, maxIdle, lifetimeSec := dbcfg.NormalizePool()
	sqlDB.SetMaxOpenConns(maxOpen)
	sqlDB.SetMaxIdleConns(maxIdle)
	sqlDB.SetConnMaxLifetime(time.Duration(lifetimeSec) * time.Second)
	return sqlDB.PingContext(ctx)
}
