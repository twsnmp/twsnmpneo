package polling

import (
	"context"
	"fmt"
	"net"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// DNSPoller tests name resolution for various DNS record types.
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
			return &Result{State: StateHigh, Message: "missing host to resolve (set Params)"}, nil
		}
	}

	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "ipaddr", "change":
		mode = "ipaddr"
	case "addr", "host", "mx", "ns", "txt", "cname":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported dns mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported dns mode: %s", pe.Mode)},
		}, nil
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 3 * time.Second
	}

	// Use node IP as DNS server if available
	resolver := net.DefaultResolver
	if node != nil && node.IP != "" {
		dnsAddr := net.JoinHostPort(node.IP, "53")
		resolver = &net.Resolver{
			PreferGo: true,
			Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
				d := net.Dialer{Timeout: timeout}
				return d.DialContext(ctx, "udp", dnsAddr)
			},
		}
	}

	var out []string
	var lastErr error
	var rtt time.Duration
	ok := false
	for i := 0; !ok && i <= pe.Retry; i++ {
		start := time.Now()
		out, lastErr = dnsLookup(ctx, resolver, mode, host)
		rtt = time.Since(start)
		if lastErr == nil && len(out) > 0 {
			ok = true
		}
	}

	if !ok {
		errStr := ""
		if lastErr != nil {
			errStr = lastErr.Error()
		}
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("dns %s lookup for %s failed: %v", mode, host, lastErr),
			Fields: map[string]interface{}{
				"rtt":   float64(0),
				"count": float64(0),
				"ip":    "",
				"error": errStr,
			},
		}, nil
	}

	fields := map[string]interface{}{
		"rtt":   float64(rtt.Nanoseconds()),
		"count": float64(len(out)),
	}

	// ipaddr mode: detect IP address changes
	if mode == "ipaddr" {
		oldIP := ""
		if pe.Result != nil {
			if v, exists := pe.Result["ip"]; exists {
				if s, ok := v.(string); ok {
					oldIP = s
				}
			}
		}
		fields["ip"] = out[0]
		msg := fmt.Sprintf("dns resolved %s -> %s, rtt=%v", host, out[0], rtt)
		if oldIP != "" && oldIP != out[0] {
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("dns IP changed: %s -> %s", oldIP, out[0]),
				Fields:  fields,
			}, nil
		}
		return &Result{State: StateNormal, RTT: rtt, Message: msg, Fields: fields}, nil
	}

	// Other modes: populate field and run JS script if set
	joined := strings.Join(out, ",")
	switch mode {
	case "addr":
		fields["addr"] = joined
	case "host":
		fields["host"] = joined
	case "mx":
		fields["mx"] = joined
	case "ns":
		fields["ns"] = joined
	case "txt":
		fields["txt"] = joined
	case "cname":
		fields["cname"] = out[0]
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("dns %s lookup ok: %s", mode, joined),
			Fields:  fields,
		}, nil
	}

	// JS script evaluation
	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))
	_ = vm.Set("count", float64(len(out)))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}
	value, err := vm.Run(pe.Script)
	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("dns script error: %v", err),
			Fields:  fields,
		}, nil
	}
	if pass, _ := value.ToBoolean(); pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "dns script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "dns script returned false", Fields: fields}, nil
}

func dnsLookup(ctx context.Context, r *net.Resolver, mode, target string) ([]string, error) {
	switch mode {
	case "ipaddr", "addr", "host":
		if mode == "addr" {
			return r.LookupAddr(ctx, target)
		}
		return r.LookupHost(ctx, target)
	case "mx":
		mxs, err := r.LookupMX(ctx, target)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(mxs))
		for _, mx := range mxs {
			out = append(out, mx.Host)
		}
		return out, nil
	case "ns":
		nss, err := r.LookupNS(ctx, target)
		if err != nil {
			return nil, err
		}
		out := make([]string, 0, len(nss))
		for _, ns := range nss {
			out = append(out, ns.Host)
		}
		return out, nil
	case "cname":
		cname, err := r.LookupCNAME(ctx, target)
		if err != nil {
			return nil, err
		}
		return []string{cname}, nil
	case "txt":
		return r.LookupTXT(ctx, target)
	}
	return r.LookupHost(ctx, target)
}

// failureState returns StateHigh unless pe.Level specifies a lower severity.
func failureState(level string) string {
	switch level {
	case "low", "warn", "info":
		return level
	}
	return StateHigh
}
