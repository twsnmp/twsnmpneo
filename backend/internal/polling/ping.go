package polling

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

// PingPoller performs connectivity check via ICMP.
type PingPoller struct{}

func NewPingPoller() *PingPoller {
	return &PingPoller{}
}

func (p *PingPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	target := ""
	if pe != nil && pe.Params != "" && !strings.Contains(pe.Params, "=") {
		// If pe.Params is just a hostname/IP (no '='), use it as target
		target = strings.TrimSpace(pe.Params)
	}
	if target == "" && node != nil {
		if node.IP != "" {
			target = node.IP
		} else if node.Name != "" {
			target = node.Name
		}
	}
	if target == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address or host target",
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

	res := ping.DoPing(target, timeout, retry, 64, 64)
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
				"rtt": float64(rtt.Nanoseconds()),
			},
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("ping ok, rtt=%v, ttl=%d", rtt, res.RecvTTL),
		Fields: map[string]interface{}{
			"rtt": float64(rtt.Nanoseconds()),
			"ttl": float64(res.RecvTTL),
		},
	}, nil
}
