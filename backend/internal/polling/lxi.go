package polling

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// LXIPoller communicates with LXI / SCPI instruments over raw TCP socket.
type LXIPoller struct{}

func NewLXIPoller() *LXIPoller {
	return &LXIPoller{}
}

func (p *LXIPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if pe.Mode != "" {
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported lxi mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported lxi mode: %s", pe.Mode)},
		}, nil
	}
	start := time.Now()
	if pe.Script == "" {
		return &Result{State: failureState(pe.Level), Message: "lxi polling requires a Script"}, nil
	}

	addr := pe.Params
	if addr == "" && node != nil {
		addr = fmt.Sprintf("%s:5025", node.IP)
	} else if pNum, err := strconv.Atoi(addr); err == nil && pNum > 0 && pNum < 65535 && node != nil {
		addr = fmt.Sprintf("%s:%d", node.IP, pNum)
	}
	if !strings.Contains(addr, ":") {
		addr += ":5025"
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
			Message: fmt.Sprintf("lxi connect failed: %v", err),
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": err.Error()},
		}, nil
	}
	defer conn.Close()

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(time.Now().UnixNano()),
	}

	reader := bufio.NewReader(conn)
	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	// lxiCommand(cmd)
	_ = vm.Set("lxiCommand", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			cmd := call.Argument(0).String()
			if !strings.HasSuffix(cmd, "\n") {
				cmd += "\n"
			}
			_ = conn.SetDeadline(time.Now().Add(timeout))
			if _, err := conn.Write([]byte(cmd)); err == nil {
				if ov, err := otto.ToValue(true); err == nil {
					return ov
				}
			}
		}
		if ov, err := otto.ToValue(false); err == nil {
			return ov
		}
		return otto.UndefinedValue()
	})

	// lxiQuery(query)
	_ = vm.Set("lxiQuery", func(call otto.FunctionCall) otto.Value {
		if call.Argument(0).IsString() {
			query := call.Argument(0).String()
			if !strings.HasSuffix(query, "\n") {
				query += "\n"
			}
			_ = conn.SetDeadline(time.Now().Add(timeout))
			if _, err := conn.Write([]byte(query)); err == nil {
				line, err := reader.ReadString('\n')
				if err == nil {
					trimmed := strings.TrimRight(line, "\r\n")
					if ov, err := otto.ToValue(trimmed); err == nil {
						return ov
					}
				}
			}
		}
		return otto.UndefinedValue()
	})

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("lxi script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "lxi script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "lxi threshold breached", Fields: fields}, nil
}
