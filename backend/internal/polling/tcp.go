package polling

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// TCPPoller checks TCP port reachability and optionally verifies banner text.
type TCPPoller struct{}

func (p *TCPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address",
		}, nil
	}

	portStr := pe.Params
	if portStr == "" {
		portStr = "80"
	}
	port, err := strconv.Atoi(portStr)
	if err != nil || port <= 0 || port > 65535 {
		port = 80
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	target := net.JoinHostPort(node.IP, strconv.Itoa(port))
	start := time.Now()

	d := net.Dialer{Timeout: timeout}
	conn, err := d.DialContext(ctx, "tcp", target)
	rtt := time.Since(start)

	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("tcp connection to %s failed: %v", target, err),
			Fields: map[string]interface{}{
				"port": port,
				"rtt":  rtt.Milliseconds(),
			},
		}, nil
	}
	defer conn.Close()

	// If Filter is set, check banner text
	banner := ""
	if pe.Filter != "" {
		_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
		reader := bufio.NewReader(conn)
		line, _ := reader.ReadString('\n')
		banner = strings.TrimSpace(line)
		if !strings.Contains(banner, pe.Filter) {
			return &Result{
				State:   StateWarn,
				RTT:     rtt,
				Message: fmt.Sprintf("banner mismatch: got '%s', expected '%s'", banner, pe.Filter),
				Fields: map[string]interface{}{
					"port":   port,
					"rtt":    rtt.Milliseconds(),
					"banner": banner,
				},
			}, nil
		}
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("tcp port %d open, rtt=%v", port, rtt),
		Fields: map[string]interface{}{
			"port":   port,
			"rtt":    rtt.Milliseconds(),
			"banner": banner,
		},
	}, nil
}
