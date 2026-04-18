package temporal

import (
	"fmt"
	"strings"

	"go.temporal.io/sdk/client"
)

// DialOptions configures a Temporal frontend connection.
type DialOptions struct {
	HostPort  string
	Namespace string
}

// Dial returns a Temporal SDK client. The API process calls this from [bootstrap.BuildRuntime] when
// Temporal is enabled in configuration; worker-only processes may use it when a cmd/* worker is added.
func Dial(o DialOptions) (client.Client, error) {
	host := strings.TrimSpace(o.HostPort)
	if host == "" {
		return nil, fmt.Errorf("temporal: HostPort is required")
	}
	ns := strings.TrimSpace(o.Namespace)
	if ns == "" {
		ns = "default"
	}
	opts := client.Options{
		HostPort:  host,
		Namespace: ns,
	}
	c, err := client.Dial(opts)
	if err != nil {
		return nil, fmt.Errorf("temporal: dial %s ns=%s: %w", host, ns, err)
	}
	return c, nil
}
