package temporalopts

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestDefaultActivityOptions_matchesReferenceContract(t *testing.T) {
	t.Parallel()

	ao := DefaultActivityOptions()
	require.Equal(t, 30*time.Second, ao.StartToCloseTimeout)
	require.Equal(t, time.Minute, ao.ScheduleToCloseTimeout)
	require.NotNil(t, ao.RetryPolicy)
	require.Equal(t, int32(3), ao.RetryPolicy.MaximumAttempts)
	require.Equal(t, time.Second, ao.RetryPolicy.InitialInterval)
	require.Equal(t, 2.0, ao.RetryPolicy.BackoffCoefficient)
}
