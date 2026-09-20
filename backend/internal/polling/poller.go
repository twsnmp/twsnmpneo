package polling

import (
	"context"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// State constants for polling outcomes
const (
	StateNormal = "normal"
	StateWarn   = "warn"
	StateLow    = "low"
	StateHigh   = "high"
	StateRepair = "repair"
	StateUnknown = "unknown"
)

// Result represents the outcome of a single polling execution.
type Result struct {
	State   string                 `json:"state"`
	RTT     time.Duration          `json:"rtt"`
	Message string                 `json:"message"`
	Fields  map[string]interface{} `json:"fields"`
}

// Poller defines the interface for individual check implementations.
type Poller interface {
	Poll(ctx context.Context, p *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error)
}
