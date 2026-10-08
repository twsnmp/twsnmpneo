package polling

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/url"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// MQTTPoller tests MQTT broker connectivity and payload evaluation.
type MQTTPoller struct{}

func NewMQTTPoller() *MQTTPoller {
	return &MQTTPoller{}
}

func (p *MQTTPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "connect", "subscribe":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported mqtt mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported mqtt mode: %s", pe.Mode)},
		}, nil
	}
	start := time.Now()

	rawURL := pe.MqttURL
	if rawURL == "" {
		rawURL = pe.Params
	}
	if rawURL == "" && node != nil {
		rawURL = fmt.Sprintf("tcp://%s:1883", node.IP)
	}
	if !strings.Contains(rawURL, "://") {
		rawURL = "tcp://" + rawURL
	}

	u, err := url.Parse(rawURL)
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("invalid mqtt url: %v", err)}, nil
	}

	hostPort := u.Host
	if !strings.Contains(hostPort, ":") {
		if u.Scheme == "ssl" || u.Scheme == "tls" || u.Scheme == "tcps" {
			hostPort += ":8883"
		} else {
			hostPort += ":1883"
		}
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	var conn net.Conn
	var dialErr error
	if u.Scheme == "ssl" || u.Scheme == "tls" || u.Scheme == "tcps" {
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: timeout},
			Config:    &tls.Config{InsecureSkipVerify: true},
		}
		conn, dialErr = dialer.DialContext(ctx, "tcp", hostPort)
	} else {
		dialer := &net.Dialer{Timeout: timeout}
		conn, dialErr = dialer.DialContext(ctx, "tcp", hostPort)
	}

	rtt := time.Since(start)
	if dialErr != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("mqtt connection failed: %v", dialErr),
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": dialErr.Error()},
		}, nil
	}
	_ = conn.Close()

	topicStr := pe.MqttTopic
	if topicStr == "" && pe.Filter != "" {
		topicStr = pe.Filter
	}
	if topicStr == "" && pe.Result != nil {
		if t, ok := pe.Result["topic"].(string); ok {
			topicStr = t
		}
	}

	payloadStr := ""
	if pe.Result != nil {
		if p, ok := pe.Result["payload"].(string); ok {
			payloadStr = p
		}
	}

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(time.Now().UnixNano()),
		"topic":    topicStr,
		"payload":  payloadStr,
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(payloadStr, vm)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))
	_ = vm.Set("topic", topicStr)
	_ = vm.Set("payload", payloadStr)

	if pe.Extractor != "" && payloadStr != "" {
		_ = extractor.ApplyExtractor(pe.Extractor, payloadStr, vm, fields)
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("mqtt broker connected ok: rtt=%v", rtt),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("mqtt script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "mqtt script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "mqtt threshold breached", Fields: fields}, nil
}
