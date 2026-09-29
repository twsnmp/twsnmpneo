package polling

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// TCPPoller checks TCP port reachability with optional TLS certificate verification.
type TCPPoller struct{}

func NewTCPPoller() *TCPPoller {
	return &TCPPoller{}
}

func (p *TCPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{State: StateHigh, Message: "missing node IP address"}, nil
	}

	mode := pe.Mode
	switch mode {
	case "verify", "version", "expire":
		return p.pollTLS(pe, node)
	}
	return p.pollTCP(ctx, pe, node)
}

// pollTCP performs a plain TCP connection check.
func (p *TCPPoller) pollTCP(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	portStr := pe.Params
	if portStr == "" {
		portStr = "80"
	}
	// strip non-port prefix like "host:port"
	if strings.Contains(portStr, ":") {
		parts := strings.SplitN(portStr, ":", 2)
		portStr = parts[1]
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

	var rtt time.Duration
	var lastErr error
	for i := 0; i <= pe.Retry; i++ {
		if i > 0 {
			time.Sleep(100 * time.Millisecond)
		}
		start := time.Now()
		d := net.Dialer{Timeout: timeout}
		conn, err := d.DialContext(ctx, "tcp", target)
		rtt = time.Since(start)
		if err != nil {
			lastErr = err
			continue
		}
		// If Filter is specified, check banner
		banner := ""
		if pe.Filter != "" {
			_ = conn.SetReadDeadline(time.Now().Add(1 * time.Second))
			buf := make([]byte, 1024)
			n, _ := conn.Read(buf)
			banner = strings.TrimSpace(string(buf[:n]))
			conn.Close()
			if !strings.Contains(banner, pe.Filter) {
				return &Result{
					State:   StateWarn,
					RTT:     rtt,
					Message: fmt.Sprintf("banner mismatch: got '%s', expected '%s'", banner, pe.Filter),
					Fields: map[string]interface{}{
						"rtt":    float64(rtt.Nanoseconds()),
						"port":   float64(port),
						"banner": banner,
					},
				}, nil
			}
		} else {
			conn.Close()
		}
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("tcp port %d open, rtt=%v", port, rtt),
			Fields: map[string]interface{}{
				"rtt":    float64(rtt.Nanoseconds()),
				"port":   float64(port),
				"banner": banner,
			},
		}, nil
	}
	return &Result{
		State:   StateHigh,
		RTT:     rtt,
		Message: fmt.Sprintf("tcp connection to %s failed: %v", target, lastErr),
		Fields: map[string]interface{}{
			"rtt":   float64(rtt.Nanoseconds()),
			"port":  float64(port),
			"error": lastErr.Error(),
		},
	}, nil
}

// pollTLS performs a TLS handshake and optionally verifies certificate validity.
func (p *TCPPoller) pollTLS(pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := pe.Mode
	if mode == "" {
		mode = "verify"
	}
	target := pe.Params
	if target == "" {
		target = net.JoinHostPort(node.IP, "443")
	} else if !strings.Contains(target, ":") {
		target = net.JoinHostPort(node.IP, target)
	}

	tlsCfg := &tls.Config{
		InsecureSkipVerify: true, //nolint:gosec
	}
	switch mode {
	case "verify":
		tlsCfg.InsecureSkipVerify = false
	case "version":
		// Script field contains desired version string e.g. "1.2"
		switch {
		case strings.Contains(pe.Script, "1.0"):
			tlsCfg.MinVersion = tls.VersionTLS10
			tlsCfg.MaxVersion = tls.VersionTLS10
		case strings.Contains(pe.Script, "1.1"):
			tlsCfg.MinVersion = tls.VersionTLS11
			tlsCfg.MaxVersion = tls.VersionTLS11
		case strings.Contains(pe.Script, "1.2"):
			tlsCfg.MinVersion = tls.VersionTLS12
			tlsCfg.MaxVersion = tls.VersionTLS12
		case strings.Contains(pe.Script, "1.3"):
			tlsCfg.MinVersion = tls.VersionTLS13
			tlsCfg.MaxVersion = tls.VersionTLS13
		}
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}
	d := &net.Dialer{Timeout: timeout}

	var rtt time.Duration
	var cs tls.ConnectionState
	var lastErr error
	ok := false
	for i := 0; !ok && i <= pe.Retry; i++ {
		start := time.Now()
		conn, err := tls.DialWithDialer(d, "tcp", target, tlsCfg)
		rtt = time.Since(start)
		if err != nil {
			lastErr = err
			continue
		}
		cs = conn.ConnectionState()
		conn.Close()
		ok = true
	}

	fields := map[string]interface{}{
		"rtt": float64(rtt.Nanoseconds()),
	}
	if !ok {
		fields["error"] = lastErr.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("tls connect to %s failed: %v", target, lastErr),
			Fields:  fields,
		}, nil
	}

	// Populate TLS state fields (FK compatible)
	host := node.Name
	if a := strings.SplitN(target, ":", 2); len(a) > 1 {
		host = a[0]
	}
	fillTLSFields(fields, host, &cs)

	// expire mode: check if cert expires within Script days
	if mode == "expire" {
		var days int
		if _, err := fmt.Sscanf(pe.Script, "%d", &days); err == nil && days > 0 {
			cert := serverCert(host, &cs)
			if cert == nil {
				fields["error"] = "server certificate not found"
				return &Result{
					State:   StateHigh,
					RTT:     rtt,
					Message: "tls expire: server certificate not found",
					Fields:  fields,
				}, nil
			}
			deadline := time.Now().AddDate(0, 0, days)
			if deadline.After(cert.NotAfter) {
				return &Result{
					State:   StateHigh,
					RTT:     rtt,
					Message: fmt.Sprintf("tls cert expires %s (within %d days)", cert.NotAfter.Format("2006/01/02"), days),
					Fields:  fields,
				}, nil
			}
		}
	}

	// version mode: negate result if Script contains "!"
	if mode == "version" && strings.Contains(pe.Script, "!") {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: "tls version check failed (negated)",
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("tls %s ok, rtt=%v", mode, rtt),
		Fields:  fields,
	}, nil
}

func fillTLSFields(fields map[string]interface{}, host string, cs *tls.ConnectionState) {
	switch cs.Version {
	case 0x0300:
		fields["version"] = "SSLv3"
	case tls.VersionTLS10:
		fields["version"] = "TLSv1.0"
	case tls.VersionTLS11:
		fields["version"] = "TLSv1.1"
	case tls.VersionTLS12:
		fields["version"] = "TLSv1.2"
	case tls.VersionTLS13:
		fields["version"] = "TLSv1.3"
	default:
		fields["version"] = "Unknown"
	}
	fields["cipherSuite"] = fmt.Sprintf("%04x", cs.CipherSuite)
	fields["valid"] = "false"
	if cert := serverCert(host, cs); cert != nil {
		fields["issuer"] = cert.Issuer.String()
		fields["subject"] = cert.Subject.String()
		fields["notAfter"] = cert.NotAfter.Format("2006/01/02")
		fields["subjectKeyID"] = fmt.Sprintf("%x", cert.SubjectKeyId)
		if cert.NotAfter.After(time.Now()) {
			fields["valid"] = "true"
		}
	}
}

func serverCert(host string, cs *tls.ConnectionState) *x509.Certificate {
	for _, chain := range cs.VerifiedChains {
		for _, c := range chain {
			if c.VerifyHostname(host) == nil {
				return c
			}
		}
	}
	if ip := net.ParseIP(host); ip != nil {
		host = "[" + host + "]"
	}
	for _, c := range cs.PeerCertificates {
		if c.VerifyHostname(host) == nil {
			return c
		}
	}
	if len(cs.PeerCertificates) > 0 {
		return cs.PeerCertificates[0]
	}
	return nil
}
