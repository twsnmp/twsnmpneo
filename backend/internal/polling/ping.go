package polling

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// PingPoller performs connectivity check via UDP or ICMP.
type PingPoller struct{}

func (p *PingPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address",
		}, nil
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	start := time.Now()
	addr := net.JoinHostPort(node.IP, "7") // Echo port

	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	rtt := time.Since(start)

	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ping failed: %v", err),
			Fields: map[string]interface{}{
				"rtt": rtt.Milliseconds(),
			},
		}, nil
	}
	_ = conn.Close()

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("ping ok, rtt=%v", rtt),
		Fields: map[string]interface{}{
			"rtt": rtt.Milliseconds(),
		},
	}, nil
}
