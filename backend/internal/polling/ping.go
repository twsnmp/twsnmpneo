package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

// PingPoller performs connectivity check via ICMP.
type PingPoller struct{}

func (p *PingPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address",
		}, nil
	}

	timeout := pe.Timeout
	if timeout <= 0 {
		timeout = 2
	}
	retry := pe.Retry
	if retry < 0 {
		retry = 1
	}

	res := ping.DoPing(node.IP, timeout, retry, 64, 64)
	rtt := time.Duration(res.Time)

	if res.Stat != ping.PingOK {
		errMsg := res.Stat.String()
		if res.Error != nil {
			errMsg = res.Error.Error()
		}
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ping failed: %s", errMsg),
			Fields: map[string]interface{}{
				"rtt": rtt.Milliseconds(),
			},
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("ping ok, rtt=%v, ttl=%d", rtt, res.RecvTTL),
		Fields: map[string]interface{}{
			"rtt": rtt.Milliseconds(),
			"ttl": res.RecvTTL,
		},
	}, nil
}
