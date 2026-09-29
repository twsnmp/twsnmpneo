package polling

import (
	"context"
	"fmt"
	"time"

	"github.com/beevik/ntp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// NTPPoller checks NTP server reachability and measures clock offset.
type NTPPoller struct{}

func NewNTPPoller() *NTPPoller {
	return &NTPPoller{}
}

func (p *NTPPoller) Poll(_ context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{State: StateHigh, Message: "missing node IP address"}, nil
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	var lastErr error
	for i := 0; i <= pe.Retry; i++ {
		r, err := ntp.QueryWithOptions(node.IP, ntp.QueryOptions{Timeout: timeout})
		if err != nil {
			lastErr = err
			continue
		}
		rtt := r.RTT
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("ntp ok, rtt=%v, stratum=%d, offset=%v", rtt, r.Stratum, r.ClockOffset),
			Fields: map[string]interface{}{
				"rtt":    float64(rtt.Nanoseconds()),
				"stratum": float64(r.Stratum),
				"refid":  float64(r.ReferenceID),
				"offset": float64(r.ClockOffset.Nanoseconds()),
			},
		}, nil
	}
	return &Result{
		State:   StateHigh,
		Message: fmt.Sprintf("ntp query to %s failed: %v", node.IP, lastErr),
		Fields: map[string]interface{}{
			"rtt":    float64(0),
			"stratum": float64(0),
			"refid":  float64(0),
			"offset": float64(0),
			"error":  lastErr.Error(),
		},
	}, nil
}
