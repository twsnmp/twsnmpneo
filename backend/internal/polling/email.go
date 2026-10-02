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
)

// EMailPoller tests IMAP/POP3 mail server connectivity and message counts.
type EMailPoller struct{}

func NewEMailPoller() *EMailPoller {
	return &EMailPoller{}
}

func (p *EMailPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()

	target := pe.Params
	if target == "" && node != nil {
		target = fmt.Sprintf("imaps://%s:993", node.IP)
	}

	u, err := url.Parse(target)
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("invalid email target url: %v", err)}, nil
	}

	scheme := strings.ToLower(u.Scheme)
	host := u.Host
	if !strings.Contains(host, ":") {
		switch scheme {
		case "imaps":
			host += ":993"
		case "imap":
			host += ":143"
		case "pop3s":
			host += ":995"
		case "pop3":
			host += ":110"
		default:
			host += ":993"
		}
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	var conn net.Conn
	var dialErr error
	if scheme == "imaps" || scheme == "pop3s" {
		dialer := &tls.Dialer{
			NetDialer: &net.Dialer{Timeout: timeout},
			Config:    &tls.Config{InsecureSkipVerify: true},
		}
		conn, dialErr = dialer.DialContext(ctx, "tcp", host)
	} else {
		dialer := &net.Dialer{Timeout: timeout}
		conn, dialErr = dialer.DialContext(ctx, "tcp", host)
	}

	rtt := time.Since(start)
	if dialErr != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("email server connect failed: %v", dialErr),
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": dialErr.Error()},
		}, nil
	}
	_ = conn.Close()

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(time.Now().UnixNano()),
		"count":    float64(0),
		"size":     float64(0),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("email service %s connected ok: rtt=%v", host, rtt),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("email script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "email script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "email threshold breached", Fields: fields}, nil
}
