package device

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/require"
)

func TestMQTTAttemptPublishInFlight_preventsDuplicateAcquire(t *testing.T) {
	t.Parallel()
	d := &MQTTCommandDispatcher{publishMaxAttempts: 3, retryBackoff: 150 * time.Millisecond}
	id := uuid.New()
	require.True(t, d.beginMQTTAttemptPublish(id))
	require.False(t, d.beginMQTTAttemptPublish(id))
	d.endMQTTAttemptPublish(id)
	require.True(t, d.beginMQTTAttemptPublish(id))
}

func TestMQTTPublishReclaimMinAge_coversPublishBudget(t *testing.T) {
	t.Parallel()
	d := &MQTTCommandDispatcher{publishMaxAttempts: 3, retryBackoff: 150 * time.Millisecond}
	age := d.mqttPublishReclaimMinAge()
	require.GreaterOrEqual(t, age, 3*mqttPublishTimeoutPerTry)
	require.Less(t, age, 20*time.Second)
}
