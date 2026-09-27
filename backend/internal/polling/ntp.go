package polling

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// NTPPoller checks NTP server responsiveness via UDP 123.
type NTPPoller struct{}

func NewNTPPoller() *NTPPoller {
	return &NTPPoller{}
}

func (p *NTPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
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

	addr := net.JoinHostPort(node.IP, "123")
	start := time.Now()

	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "udp", addr)
	rtt := time.Since(start)

	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ntp connection to %s failed: %v", addr, err),
		}, nil
	}
	defer conn.Close()

	// Send minimal NTP request (client mode 3, version 4)
	req := make([]byte, 48)
	req[0] = 0x23 // LI 0, VN 4, Mode 3
	_ = conn.SetDeadline(time.Now().Add(timeout))

	if _, err := conn.Write(req); err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ntp write error: %v", err),
		}, nil
	}

	resp := make([]byte, 48)
	n, err := conn.Read(resp)
	rtt = time.Since(start)

	if err != nil || n < 48 {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ntp response error: %v", err),
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("ntp response ok, rtt=%v", rtt),
		Fields: map[string]interface{}{
			"rtt": float64(rtt.Nanoseconds()),
		},
	}, nil
}
