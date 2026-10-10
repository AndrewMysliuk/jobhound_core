package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

func TestBegin_lockAndRelease(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	rel, err := c.Begin(ctx, "greenhouse", "", false)
	require.NoError(t, err)
	require.NotNil(t, rel)
	require.True(t, mr.Exists("ingest:lock:greenhouse:catalog"))

	_, err = c.Begin(ctx, "greenhouse", "", false)
	require.ErrorIs(t, err, ErrLockHeld)

	require.NoError(t, rel(ctx))

	rel2, err := c.Begin(ctx, "greenhouse", "", false)
	require.NoError(t, err)
	require.NoError(t, rel2(ctx))
}

func TestBegin_cooldownBlocksUnlessExplicitRefresh(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	require.NoError(t, c.RecordSuccessfulIngest(ctx, "src-a", ""))
	require.True(t, mr.Exists("ingest:cooldown:src-a:catalog"))

	_, err := c.Begin(ctx, "src-a", "", false)
	require.ErrorIs(t, err, ErrCooldownActive)

	rel, err := c.Begin(ctx, "src-a", "", true)
	require.NoError(t, err)
	require.NoError(t, rel(ctx))
}

func TestBegin_failClosedRedisUnavailable(t *testing.T) {
	// No server on this port — fail closed quickly without miniredis retry noise.
	rdb := redis.NewClient(&redis.Options{
		Addr:            "127.0.0.1:63799",
		MaxRetries:      0,
		DialTimeout:     20 * time.Millisecond,
		ReadTimeout:     20 * time.Millisecond,
		WriteTimeout:    20 * time.Millisecond,
		ConnMaxIdleTime: time.Second,
	})
	t.Cleanup(func() { _ = rdb.Close() })
	c := NewRedisCoordinator(rdb)

	_, err := c.Begin(context.Background(), "x", "", false)
	require.Error(t, err)
	require.False(t, errors.Is(err, ErrLockHeld))
	require.False(t, errors.Is(err, ErrCooldownActive))
}

func TestRecordSuccessfulIngest_TTL(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	require.NoError(t, c.RecordSuccessfulIngest(ctx, "k", ""))

	ttl := mr.TTL(cooldownKey(NormalizeSourceID("k"), catalogQuerySegment))
	require.GreaterOrEqual(t, ttl, time.Duration(IngestCooldownTTLSeconds)*time.Second-time.Second)
	require.LessOrEqual(t, ttl, time.Duration(IngestCooldownTTLSeconds)*time.Second)
}

func TestBegin_lockTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	rel, err := c.Begin(ctx, "job", "", false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rel(ctx) })

	ttl := mr.TTL(lockKey(NormalizeSourceID("job"), catalogQuerySegment))
	require.GreaterOrEqual(t, ttl, time.Duration(IngestLockTTLSeconds)*time.Second-time.Second)
	require.LessOrEqual(t, ttl, time.Duration(IngestLockTTLSeconds)*time.Second)
}

func TestBegin_lockTTL_custom(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinatorWithTTL(rdb, 42, 99)
	ctx := context.Background()

	rel, err := c.Begin(ctx, "custom-ttl", "", false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rel(ctx) })

	ttl := mr.TTL(lockKey(NormalizeSourceID("custom-ttl"), catalogQuerySegment))
	require.GreaterOrEqual(t, ttl, 41*time.Second)
	require.LessOrEqual(t, ttl, 42*time.Second)

	require.NoError(t, rel(ctx))
	require.NoError(t, c.RecordSuccessfulIngest(ctx, "custom-ttl", ""))
	cttl := mr.TTL(cooldownKey(NormalizeSourceID("custom-ttl"), catalogQuerySegment))
	require.GreaterOrEqual(t, cttl, 98*time.Second)
	require.LessOrEqual(t, cttl, 99*time.Second)
}

func TestBegin_emptySourceID(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	c := NewRedisCoordinator(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	_, err := c.Begin(context.Background(), "   ", "", false)
	require.ErrorIs(t, err, ErrEmptySourceID)
}

func TestBegin_differentQueriesDoNotBlock(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	relA, err := c.Begin(ctx, "shared-src", "Vue", false)
	require.NoError(t, err)
	require.True(t, mr.Exists("ingest:lock:shared-src:vue"))
	relB, err := c.Begin(ctx, "shared-src", "react", false)
	require.NoError(t, err)
	require.True(t, mr.Exists("ingest:lock:shared-src:react"))
	require.NoError(t, relA(ctx))
	require.NoError(t, relB(ctx))
}

func TestQuerySegment_emptyIsCatalog(t *testing.T) {
	require.Equal(t, "catalog", querySegment("  "))
	require.Equal(t, "vue", querySegment(" Vue "))
	require.Equal(t, "ingest:lock:working_nomads:catalog", lockKey("working_nomads", querySegment("")))
	require.Equal(t, "ingest:cooldown:working_nomads:vue", cooldownKey("working_nomads", querySegment("Vue")))
}
