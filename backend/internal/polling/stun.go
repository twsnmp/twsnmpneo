package polling

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/stun"
)

// STUNPoller queries a STUN server to discover the external IP/port.
type STUNPoller struct{}

func NewSTUNPoller() *STUNPoller {
	return &STUNPoller{}
}

func (p *STUNPoller) Poll(_ context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	network := "udp4"
	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "ipv4", "udp4":
		network = "udp4"
	case "ipv6", "udp6":
		network = "udp6"
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported stun mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported stun mode: %s", pe.Mode)},
		}, nil
	}

	server := pe.Params
	if server == "" {
		server = stun.DefaultServer
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	var res *stun.Result
	var lastErr error
	ok := false
	for i := 0; !ok && i <= pe.Retry; i++ {
		var err error
		res, err = stun.Query(server, network, timeout)
		if err != nil {
			lastErr = err
			continue
		}
		ok = true
	}

	if !ok || res == nil {
		errStr := ""
		if lastErr != nil {
			errStr = lastErr.Error()
		}
		return &Result{
			State:   StateHigh,
			Message: fmt.Sprintf("stun query to %s failed: %v", server, lastErr),
			Fields: map[string]interface{}{
				"rtt":   float64(0),
				"ip":    "",
				"error": errStr,
			},
		}, nil
	}

	// Detect external IP change from previous result
	oldIP := ""
	if pe.Result != nil {
		if v, exists := pe.Result["ip"]; exists {
			if s, ok2 := v.(string); ok2 {
				oldIP = s
			}
		}
	}

	fields := map[string]interface{}{
		"rtt":   float64(res.RTTNano),
		"ip":    res.IP,
		"port":  float64(res.Port),
		"host":  res.Hostname,
		"local": fmt.Sprintf("%s:%d", res.LocalIP, res.LocalPort),
	}

	ipChanged := oldIP != "" && oldIP != res.IP

	// No script: use default logic (IP change → failure)
	if pe.Script == "" {
		if ipChanged {
			return &Result{
				State:   failureState(pe.Level),
				RTT:     res.RTT,
				Message: fmt.Sprintf("stun external IP changed: %s -> %s", oldIP, res.IP),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   StateNormal,
			RTT:     res.RTT,
			Message: fmt.Sprintf("stun ok, external=%s:%d, rtt=%v", res.IP, res.Port, res.RTT),
			Fields:  fields,
		}, nil
	}

	// JS script evaluation
	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("ip", res.IP)
	_ = vm.Set("oldip", oldIP)
	_ = vm.Set("port", float64(res.Port))
	_ = vm.Set("host", res.Hostname)
	_ = vm.Set("local", res.LocalIP)
	_ = vm.Set("rtt", float64(res.RTTNano))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	// Strip negation flag from script for VM run
	script := strings.TrimPrefix(pe.Script, "!")
	value, err := vm.Run(script)
	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     res.RTT,
			Message: fmt.Sprintf("stun script error: %v", err),
			Fields:  fields,
		}, nil
	}
	pass, _ := value.ToBoolean()
	if strings.HasPrefix(pe.Script, "!") {
		pass = !pass
	}
	if pass {
		return &Result{State: StateNormal, RTT: res.RTT, Message: "stun script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: res.RTT, Message: "stun script returned false", Fields: fields}, nil
}
