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

func NewDNSPoller() *DNSPoller {
	return &DNSPoller{}
}

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
				"rtt": float64(rtt.Nanoseconds()),
			},
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("resolved %s to %v in %v", host, ips, rtt),
		Fields: map[string]interface{}{
			"ips": ips,
			"rtt": float64(rtt.Nanoseconds()),
		},
	}, nil
}
