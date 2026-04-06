//go:build integration

package ingest

import (
	"context"
	"os"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/andrewmysliuk/jobhound_core/internal/config"
	"github.com/google/uuid"
	"github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/require"
)

// TestRedisCoordinator_liveRedis_integration exercises lock/cooldown against a real Redis
// when JOBHOUND_REDIS_URL is set (e.g. docker compose). Skips otherwise.
func TestRedisCoordinator_liveRedis_integration(t *testing.T) {
	url := strings.TrimSpace(os.Getenv(config.EnvRedisURL))
	if url == "" {
		t.Skip("set JOBHOUND_REDIS_URL for live Redis ingest coordination test")
	}

	opt, err := redis.ParseURL(url)
	require.NoError(t, err)
	rdb := redis.NewClient(opt)
	t.Cleanup(func() { _ = rdb.Close() })

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	require.NoError(t, rdb.Ping(ctx).Err())

	src := "integration-ingest-" + strconv.FormatInt(time.Now().UnixNano(), 10)
	slotID := uuid.New()
	norm := NormalizeSourceID(src)
	lockK := lockKey(slotID, norm)
	cdK := cooldownKey(slotID, norm)
	t.Cleanup(func() {
		_ = rdb.Del(context.Background(), lockK, cdK).Err()
	})

	c := NewRedisCoordinatorWithTTL(rdb, 30, 45)
	rel, err := c.Begin(ctx, slotID, src, false)
	require.NoError(t, err)
	require.NotNil(t, rel)

	_, err = c.Begin(ctx, slotID, src, false)
	require.ErrorIs(t, err, ErrLockHeld)

	require.NoError(t, rel(ctx))

	require.NoError(t, c.RecordSuccessfulIngest(ctx, slotID, src))
	_, err = c.Begin(ctx, slotID, src, false)
	require.ErrorIs(t, err, ErrCooldownActive)

	rel2, err := c.Begin(ctx, slotID, src, true)
	require.NoError(t, err)
	require.NoError(t, rel2(ctx))
}
