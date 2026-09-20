package polling

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// DNSPoller tests name resolution.
type DNSPoller struct{}

func (p *DNSPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	host := pe.Params
	if host == "" {
		if node != nil && node.Name != "" {
			host = node.Name
		} else {
			return &Result{
				State:   StateHigh,
				Message: "missing host to resolve",
			}, nil
		}
	}

	resolver := net.DefaultResolver
	if node != nil && node.IP != "" {
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: time.Duration(pe.Timeout) * time.Second}
				return d.DialContext(ctx, "udp", net.JoinHostPort(node.IP, "53"))
			},
		}
	}

	start := time.Now()
	ips, err := resolver.LookupHost(ctx, host)
	rtt := time.Since(start)

	if err != nil || len(ips) == 0 {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("dns lookup for %s failed: %v", host, err),
			Fields: map[string]interface{}{
				"rtt": rtt.Milliseconds(),
			},
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("resolved %s to %v in %v", host, ips, rtt),
		Fields: map[string]interface{}{
			"ips": ips,
			"rtt": rtt.Milliseconds(),
		},
	}, nil
}

// NTPPoller checks NTP server responsiveness via UDP 123.
type NTPPoller struct{}

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
			"rtt": rtt.Milliseconds(),
		},
	}, nil
}
