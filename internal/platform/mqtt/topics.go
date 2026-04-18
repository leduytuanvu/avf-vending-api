package mqtt

import (
	"fmt"
	"strings"

	"github.com/google/uuid"
)

// OutboundCommandDispatchTopic is the MQTT topic the API publishes remote commands to.
// Convention mirrors inbound channels under TopicPrefix: {prefix}/{machineId}/commands/dispatch
func OutboundCommandDispatchTopic(prefix string, machineID uuid.UUID) string {
	p := strings.TrimSuffix(strings.TrimSpace(prefix), "/")
	return fmt.Sprintf("%s/%s/commands/dispatch", p, machineID)
}
