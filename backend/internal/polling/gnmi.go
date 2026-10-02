package polling

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/openconfig/gnmic/pkg/api"
	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// GNMIPoller monitors network telemetry using gNMI (Get).
type GNMIPoller struct{}

func NewGNMIPoller() *GNMIPoller {
	return &GNMIPoller{}
}

func (p *GNMIPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()
	if pe.Script == "" {
		return &Result{State: StateHigh, Message: "gnmi polling requires a Script"}, nil
	}

	target := pe.Params
	if target == "" {
		if node != nil && node.GNMIPort != "" {
			target = fmt.Sprintf("%s:%s", node.IP, node.GNMIPort)
		} else if node != nil {
			target = fmt.Sprintf("%s:57400", node.IP)
		} else {
			return &Result{State: StateHigh, Message: "target address not found"}, nil
		}
	} else if pNum, err := strconv.Atoi(target); err == nil && pNum > 0 && pNum < 65535 {
		if node != nil {
			target = fmt.Sprintf("%s:%d", node.IP, pNum)
		}
	}

	username := ""
	password := ""
	encoding := "json_ietf"
	if node != nil {
		username = node.GNMIUser
		password = node.GNMIPassword
		if node.GNMIEncoding != "" {
			encoding = node.GNMIEncoding
		}
	}

	tg, err := api.NewTarget(
		api.Name(target),
		api.Address(target),
		api.Username(username),
		api.Password(password),
		api.SkipVerify(true),
	)
	if err != nil {
		return &Result{State: failureState(pe.Level), Message: fmt.Sprintf("gnmi target error: %v", err)}, nil
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 10 * time.Second
	}
	subCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	if err := tg.CreateGNMIClient(subCtx); err != nil {
		return &Result{State: failureState(pe.Level), Message: fmt.Sprintf("gnmi connect error: %v", err)}, nil
	}
	defer tg.Close()

	path := pe.Filter
	if path == "" {
		path = "/"
	}

	getReq, err := api.NewGetRequest(api.Path(path), api.Encoding(encoding))
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("gnmi get request error: %v", err)}, nil
	}

	getResp, err := tg.Get(subCtx, getReq)
	if err != nil {
		return &Result{State: failureState(pe.Level), Message: fmt.Sprintf("gnmi get error: %v", err)}, nil
	}

	var data []byte
	for _, not := range getResp.GetNotification() {
		for _, u := range not.GetUpdate() {
			data = u.GetVal().GetJsonIetfVal()
			if len(data) > 0 {
				break
			}
			data = u.GetVal().GetJsonVal()
			if len(data) > 0 {
				break
			}
		}
		if len(data) > 0 {
			break
		}
	}

	if len(data) == 0 {
		return &Result{State: StateHigh, Message: "gnmi response contained no json data"}, nil
	}

	dataStr := string(data)
	rtt := time.Since(start)
	fields := map[string]interface{}{
		"data":     dataStr,
		"now":      float64(time.Now().UnixMilli()),
		"rtt":      float64(rtt.Nanoseconds()),
		"lastTime": float64(time.Now().UnixNano()),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(dataStr, vm)

	_ = vm.Set("data", dataStr)
	_ = vm.Set("now", float64(time.Now().UnixMilli()))
	if pe.Result != nil {
		if d, ok := pe.Result["data"].(string); ok {
			_ = vm.Set("last_data", d)
		}
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("gnmi script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "gnmi script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "gnmi threshold breached", Fields: fields}, nil
}
