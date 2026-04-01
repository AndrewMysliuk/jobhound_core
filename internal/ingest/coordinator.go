package ingest

import (
	"context"
	"time"

	"github.com/redis/go-redis/v9"
)

// RedisCoordinator applies ingest lock and cooldown using Redis (fail closed on errors).
type RedisCoordinator struct {
	rdb         *redis.Client
	lockTTL     time.Duration
	cooldownTTL time.Duration
}

// NewRedisCoordinator wraps an existing go-redis client (caller owns lifecycle).
// Lock and cooldown TTLs are the package defaults (see contract.go constants).
func NewRedisCoordinator(rdb *redis.Client) *RedisCoordinator {
	return NewRedisCoordinatorWithTTL(rdb, 0, 0)
}

// NewRedisCoordinatorWithTTL is like NewRedisCoordinator but uses lockTTLSec and
// cooldownTTLSec when positive; otherwise package defaults apply.
func NewRedisCoordinatorWithTTL(rdb *redis.Client, lockTTLSec, cooldownTTLSec int) *RedisCoordinator {
	lock := time.Duration(IngestLockTTLSeconds) * time.Second
	if lockTTLSec > 0 {
		lock = time.Duration(lockTTLSec) * time.Second
	}
	cd := time.Duration(IngestCooldownTTLSeconds) * time.Second
	if cooldownTTLSec > 0 {
		cd = time.Duration(cooldownTTLSec) * time.Second
	}
	return &RedisCoordinator{rdb: rdb, lockTTL: lock, cooldownTTL: cd}
}

func lockKey(normalizedSourceID string) string {
	return "ingest:lock:" + normalizedSourceID
}

func cooldownKey(normalizedSourceID string) string {
	return "ingest:cooldown:" + normalizedSourceID
}

// Begin acquires the ingest lock for sourceID. When explicitRefresh is false, an existing
// cooldown key blocks starting (fail closed). When explicitRefresh is true, cooldown is
// ignored but the lock is still taken. Any Redis error is returned and ingest must not proceed.
func (c *RedisCoordinator) Begin(ctx context.Context, sourceID string, explicitRefresh bool) (release func(context.Context) error, err error) {
	if c == nil || c.rdb == nil {
		return nil, ErrNilRedisClient
	}
	id := NormalizeSourceID(sourceID)
	if id == "" {
		return nil, ErrEmptySourceID
	}

	if !explicitRefresh {
		n, err := c.rdb.Exists(ctx, cooldownKey(id)).Result()
		if err != nil {
			return nil, err
		}
		if n > 0 {
			return nil, ErrCooldownActive
		}
	}

	err = c.rdb.SetArgs(ctx, lockKey(id), "1", redis.SetArgs{
		Mode: "nx",
		TTL:  c.lockTTL,
	}).Err()
	if err == redis.Nil {
		return nil, ErrLockHeld
	}
	if err != nil {
		return nil, err
	}

	release = func(ctx context.Context) error {
		return c.rdb.Del(ctx, lockKey(id)).Err()
	}
	return release, nil
}

// RecordSuccessfulIngest sets the cooldown key after a successful ingest (e.g. after Postgres commit).
func (c *RedisCoordinator) RecordSuccessfulIngest(ctx context.Context, sourceID string) error {
	if c == nil || c.rdb == nil {
		return ErrNilRedisClient
	}
	id := NormalizeSourceID(sourceID)
	if id == "" {
		return ErrEmptySourceID
	}
	return c.rdb.Set(ctx, cooldownKey(id), "1", c.cooldownTTL).Err()
}
