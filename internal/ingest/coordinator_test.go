package ingest

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/alicebob/miniredis/v2"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// Fixed slot for unit tests (lock/cooldown keys include slot_id).
var coordTestSlot = uuid.MustParse("aaaaaaaa-aaaa-4aaa-8aaa-aaaaaaaaaaaa")

var coordTestSlotB = uuid.MustParse("bbbbbbbb-bbbb-4bbb-bbbb-bbbbbbbbbbbb")

func TestBegin_lockAndRelease(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	rel, err := c.Begin(ctx, coordTestSlot, "greenhouse", false)
	require.NoError(t, err)
	require.NotNil(t, rel)

	_, err = c.Begin(ctx, coordTestSlot, "greenhouse", false)
	require.ErrorIs(t, err, ErrLockHeld)

	require.NoError(t, rel(ctx))

	rel2, err := c.Begin(ctx, coordTestSlot, "greenhouse", false)
	require.NoError(t, err)
	require.NoError(t, rel2(ctx))
}

func TestBegin_cooldownBlocksUnlessExplicitRefresh(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	require.NoError(t, c.RecordSuccessfulIngest(ctx, coordTestSlot, "src-a"))

	_, err := c.Begin(ctx, coordTestSlot, "src-a", false)
	require.ErrorIs(t, err, ErrCooldownActive)

	rel, err := c.Begin(ctx, coordTestSlot, "src-a", true)
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

	_, err := c.Begin(context.Background(), coordTestSlot, "x", false)
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

	require.NoError(t, c.RecordSuccessfulIngest(ctx, coordTestSlot, "k"))

	ttl := mr.TTL(cooldownKey(coordTestSlot, NormalizeSourceID("k")))
	require.GreaterOrEqual(t, ttl, time.Duration(IngestCooldownTTLSeconds)*time.Second-time.Second)
	require.LessOrEqual(t, ttl, time.Duration(IngestCooldownTTLSeconds)*time.Second)
}

func TestBegin_lockTTL(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	rel, err := c.Begin(ctx, coordTestSlot, "job", false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rel(ctx) })

	ttl := mr.TTL(lockKey(coordTestSlot, NormalizeSourceID("job")))
	require.GreaterOrEqual(t, ttl, time.Duration(IngestLockTTLSeconds)*time.Second-time.Second)
	require.LessOrEqual(t, ttl, time.Duration(IngestLockTTLSeconds)*time.Second)
}

func TestBegin_lockTTL_custom(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinatorWithTTL(rdb, 42, 99)
	ctx := context.Background()

	rel, err := c.Begin(ctx, coordTestSlot, "custom-ttl", false)
	require.NoError(t, err)
	t.Cleanup(func() { _ = rel(ctx) })

	ttl := mr.TTL(lockKey(coordTestSlot, NormalizeSourceID("custom-ttl")))
	require.GreaterOrEqual(t, ttl, 41*time.Second)
	require.LessOrEqual(t, ttl, 42*time.Second)

	require.NoError(t, rel(ctx))
	require.NoError(t, c.RecordSuccessfulIngest(ctx, coordTestSlot, "custom-ttl"))
	cttl := mr.TTL(cooldownKey(coordTestSlot, NormalizeSourceID("custom-ttl")))
	require.GreaterOrEqual(t, cttl, 98*time.Second)
	require.LessOrEqual(t, cttl, 99*time.Second)
}

func TestBegin_emptySourceID(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	c := NewRedisCoordinator(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	_, err := c.Begin(context.Background(), coordTestSlot, "   ", false)
	require.ErrorIs(t, err, ErrEmptySourceID)
}

func TestBegin_differentSlotsSameSourceDoNotBlock(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })

	rdb := redis.NewClient(&redis.Options{Addr: mr.Addr()})
	c := NewRedisCoordinator(rdb)
	ctx := context.Background()

	relA, err := c.Begin(ctx, coordTestSlot, "shared-src", false)
	require.NoError(t, err)
	relB, err := c.Begin(ctx, coordTestSlotB, "shared-src", false)
	require.NoError(t, err)
	require.NoError(t, relA(ctx))
	require.NoError(t, relB(ctx))
}

func TestBegin_nilSlotID(t *testing.T) {
	mr := miniredis.RunT(t)
	t.Cleanup(func() { mr.Close() })
	c := NewRedisCoordinator(redis.NewClient(&redis.Options{Addr: mr.Addr()}))

	_, err := c.Begin(context.Background(), uuid.Nil, "x", false)
	require.ErrorIs(t, err, ErrNilSlotID)
}
