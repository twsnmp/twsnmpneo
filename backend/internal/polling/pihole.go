package polling

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// PiHolePoller polls Pi-hole DNS ad-blocker status and metrics.
type PiHolePoller struct{}

func NewPiHolePoller() *PiHolePoller {
	return &PiHolePoller{}
}

func (p *PiHolePoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()

	urlStr := pe.Params
	if urlStr == "" && node != nil {
		urlStr = fmt.Sprintf("http://%s", node.IP)
	}
	if !strings.HasPrefix(urlStr, "http://") && !strings.HasPrefix(urlStr, "https://") {
		urlStr = "http://" + urlStr
	}

	endpoint := strings.TrimRight(urlStr, "/") + "/admin/api.php?summaryRaw"
	if pe.Mode != "" && !strings.HasPrefix(pe.Mode, "/") {
		endpoint = strings.TrimRight(urlStr, "/") + "/" + pe.Mode
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	client := &http.Client{
		Timeout: timeout,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("invalid pihole request: %v", err)}, nil
	}

	resp, err := client.Do(req)
	rtt := time.Since(start)
	if err != nil {
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("pihole connect error: %v", err),
			Fields:  map[string]interface{}{"rtt": float64(rtt.Nanoseconds()), "error": err.Error()},
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("read pihole response: %v", err)}, nil
	}
	bodyStr := string(bodyBytes)

	fields := map[string]interface{}{
		"rtt":        float64(rtt.Nanoseconds()),
		"code":       float64(resp.StatusCode),
		"statusCode": float64(resp.StatusCode),
		"lastTime":   float64(time.Now().UnixNano()),
	}

	// Unmarshal common stats if JSON
	var parsed map[string]interface{}
	if err := json.Unmarshal(bodyBytes, &parsed); err == nil {
		for k, v := range parsed {
			fields[k] = v
		}
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(bodyStr, vm)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		if resp.StatusCode == 200 {
			return &Result{
				State:   StateNormal,
				RTT:     rtt,
				Message: fmt.Sprintf("pihole ok: status=%d, rtt=%v", resp.StatusCode, rtt),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("pihole status error: %d", resp.StatusCode),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("pihole script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: rtt, Message: "pihole script passed", Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: rtt, Message: "pihole threshold breached", Fields: fields}, nil
}
