package notify

import (
	"context"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func publicOutboundIP(ip net.IP) bool {
	if ipv4 := ip.To4(); ipv4 != nil {
		ip = ipv4
		if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
			return false
		}
		return ip[0] != 0 && ip[0] < 224 && !(ip[0] == 100 && ip[1]&0xc0 == 64) &&
			!(ip[0] == 192 && ip[1] == 0 && ip[2] == 0) &&
			!(ip[0] == 192 && ip[1] == 0 && ip[2] == 2) &&
			!(ip[0] == 192 && ip[1] == 88 && ip[2] == 99) &&
			!(ip[0] == 198 && (ip[1] == 18 || ip[1] == 19 || ip[1] == 51 && ip[2] == 100)) &&
			!(ip[0] == 203 && ip[1] == 0 && ip[2] == 113)
	}
	ip = ip.To16()
	if ip == nil || !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() ||
		ip.IsLinkLocalUnicast() || ip.IsUnspecified() || ip.IsMulticast() {
		return false
	}
	return !(ip[0] == 0x20 && ip[1] == 0x01 && ip[2] == 0x0d && ip[3] == 0xb8) &&
		!(ip[0] == 0xfe && ip[1]&0xc0 == 0xc0)
}

func smtpOutboundIP(ip net.IP) bool {
	if ip == nil || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return false
	}
	return ip.IsPrivate() || ip.IsLoopback() || publicOutboundIP(ip)
}

func publicDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialResolvedContext(ctx, network, address, publicOutboundIP)
}

func smtpDialContext(ctx context.Context, network, address string) (net.Conn, error) {
	return dialResolvedContext(ctx, network, address, smtpOutboundIP)
}

func dialResolvedContext(ctx context.Context, network, address string, allowed func(net.IP) bool) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, fmt.Errorf("invalid outbound address: %w", err)
	}
	if _, err := strconv.Atoi(port); err != nil {
		return nil, fmt.Errorf("invalid outbound port: %w", err)
	}

	var ips []net.IP
	if ip := net.ParseIP(host); ip != nil {
		ips = []net.IP{ip}
	} else {
		resolved, err := net.DefaultResolver.LookupIPAddr(ctx, host)
		if err != nil {
			return nil, fmt.Errorf("resolve outbound host: %w", err)
		}
		for _, entry := range resolved {
			ips = append(ips, entry.IP)
		}
	}
	if len(ips) == 0 {
		return nil, fmt.Errorf("outbound host resolved to no addresses")
	}
	for _, ip := range ips {
		if !allowed(ip) {
			return nil, fmt.Errorf("outbound host resolves to a disallowed address")
		}
	}

	dialer := net.Dialer{Timeout: 10 * time.Second}
	var lastErr error
	for _, ip := range ips {
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
		lastErr = err
	}
	return nil, lastErr
}

func newTestWebhookClient() *http.Client {
	return &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			DialContext: smtpDialContext,
		},
		CheckRedirect: func(_ *http.Request, _ []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
}

func validateWebhookURL(rawURL string) error {
	u, err := url.Parse(rawURL)
	if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil {
		return fmt.Errorf("webhook URL must be an http(s) URL without user information")
	}
	if strings.TrimSpace(u.Hostname()) != u.Hostname() {
		return fmt.Errorf("invalid webhook hostname")
	}
	return nil
}
