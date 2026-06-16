package workflow

import (
	"fmt"
	"os"

	"go.temporal.io/sdk/client"
)

const (
	// TaskQueue is the default Temporal task queue for WeaveBrain.
	TaskQueue = "weavebrain-default"
)

// NewTemporalClient creates a new Temporal client with the given address.
// If addr is empty, it uses the TEMPORAL_ADDRESS env var or defaults to localhost:7233.
func NewTemporalClient(addr string) (client.Client, error) {
	if addr == "" {
		addr = os.Getenv("TEMPORAL_ADDRESS")
	}
	if addr == "" {
		addr = "localhost:7233"
	}

	c, err := client.Dial(client.Options{
		HostPort: addr,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to connect to Temporal at %s: %w", addr, err)
	}

	return c, nil
}
