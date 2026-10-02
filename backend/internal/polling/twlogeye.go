package polling

import (
	"context"
	"fmt"
	"net"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// TwLogEyePoller communicates with central TwLogEye log analytics server.
type TwLogEyePoller struct{}

func NewTwLogEyePoller() *TwLogEyePoller {
	return &TwLogEyePoller{}
}

func (p *TwLogEyePoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()

	addr := pe.Params
	if addr == "" && node != nil {
		addr = fmt.Sprintf("%s:8081", node.IP)
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	dialer := &net.Dialer{Timeout: timeout}
	conn, err := dialer.DialContext(ctx, "tcp", addr)
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("twlogeye connect failed: %v", err),
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": err.Error()},
		}, nil
	}
	_ = conn.Close()

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(time.Now().UnixNano()),
		"count":    float64(0),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("twlogeye connected ok: rtt=%v", rtt),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("twlogeye script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "twlogeye script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "twlogeye threshold breached", Fields: fields}, nil
}
