package polling

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

type restMapStatusEnt struct {
	High      int    `json:"High"`
	Low       int    `json:"Low"`
	Warn      int    `json:"Warn"`
	Normal    int    `json:"Normal"`
	Repair    int    `json:"Repair"`
	Unknown   int    `json:"Unknown"`
	DBSize    int64  `json:"DBSize"`
	DBSizeStr string `json:"DBSizeStr"`
	State     string `json:"State"`
}

// TWSNMPPoller monitors remote TWSNMP instances via their mobile/api/mapstatus endpoint.
type TWSNMPPoller struct {
	client *http.Client
}

func NewTWSNMPPoller() *TWSNMPPoller {
	return &TWSNMPPoller{
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

func (p *TWSNMPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if pe.Mode != "" {
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported twsnmp mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported twsnmp mode: %s", pe.Mode)},
		}, nil
	}
	if node == nil && pe.Params == "" {
		return &Result{State: StateHigh, Message: "missing node or URL target"}, nil
	}

	timeoutSec := pe.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	timeout := time.Duration(timeoutSec) * time.Second

	twsnmpURL, user, password, err := resolveTWSNMPTarget(pe, node)
	if err != nil {
		return &Result{State: StateHigh, Message: fmt.Sprintf("invalid twsnmp target: %v", err)}, nil
	}

	var rtt time.Duration
	var bodyBytes []byte
	var lastErr error
	ok := false

	for i := 0; !ok && i <= pe.Retry; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, twsnmpURL, nil)
		if err != nil {
			lastErr = err
			continue
		}
		if user != "" || password != "" {
			req.SetBasicAuth(user, password)
		}

		p.client.Timeout = timeout
		start := time.Now()
		resp, err := p.client.Do(req)
		rtt = time.Since(start)

		if err != nil {
			lastErr = err
			continue
		}

		if resp.StatusCode != http.StatusOK {
			resp.Body.Close()
			lastErr = fmt.Errorf("twsnmp status code: %d", resp.StatusCode)
			continue
		}

		bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
		resp.Body.Close()
		if err != nil {
			lastErr = err
			continue
		}
		ok = true
	}

	fields := map[string]interface{}{
		"rtt": float64(rtt.Nanoseconds()),
	}

	if !ok {
		errStr := ""
		if lastErr != nil {
			errStr = lastErr.Error()
		}
		fields["error"] = errStr
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("twsnmp poll failed for %s: %v", twsnmpURL, lastErr),
			Fields:  fields,
		}, nil
	}

	var status restMapStatusEnt
	if err := json.Unmarshal(bodyBytes, &status); err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("failed to parse twsnmp mapstatus: %v", err),
			Fields:  fields,
		}, nil
	}

	fields["state"] = status.State
	fields["high"] = float64(status.High)
	fields["low"] = float64(status.Low)
	fields["warn"] = float64(status.Warn)
	fields["normal"] = float64(status.Normal)
	fields["repair"] = float64(status.Repair)
	fields["unknown"] = float64(status.Unknown)
	fields["dbsize"] = float64(status.DBSize)

	resState := mapRemoteState(status.State)
	return &Result{
		State:   resState,
		RTT:     rtt,
		Message: fmt.Sprintf("twsnmp map status: %s (H:%d W:%d L:%d N:%d)", status.State, status.High, status.Warn, status.Low, status.Normal),
		Fields:  fields,
	}, nil
}

func resolveTWSNMPTarget(pe *datastore.PollingEnt, node *datastore.NodeEnt) (targetURL, user, password string, err error) {
	if node != nil {
		user = node.User
		password = node.Password
		if node.IP != "" {
			targetURL = fmt.Sprintf("http://%s:8080/mobile/api/mapstatus", node.IP)
		}
		if node.URL != "" {
			for _, u := range strings.Split(node.URL, ",") {
				if strings.HasPrefix(u, "http") {
					targetURL = strings.TrimRight(u, "/") + "/mobile/api/mapstatus"
					break
				}
			}
		}
	}

	if pe.Params != "" {
		if strings.HasPrefix(pe.Params, "http") {
			u, err := url.Parse(pe.Params)
			if err != nil {
				return "", "", "", err
			}
			if p, ok := u.User.Password(); ok && p != "" {
				password = p
			}
			if us := u.User.Username(); us != "" {
				user = us
			}
			u.User = nil
			targetURL = strings.TrimRight(u.String(), "/")
			if !strings.HasSuffix(targetURL, "/mobile/api/mapstatus") {
				targetURL += "/mobile/api/mapstatus"
			}
		} else if parts := strings.SplitN(pe.Params, ":", 2); len(parts) == 2 {
			user = parts[0]
			password = parts[1]
		}
	}

	if targetURL == "" {
		return "", "", "", fmt.Errorf("no URL or IP specified for twsnmp polling")
	}
	return targetURL, user, password, nil
}

func mapRemoteState(s string) string {
	switch strings.ToLower(s) {
	case "normal":
		return StateNormal
	case "warn", "warning":
		return StateWarn
	case "low":
		return StateLow
	case "high":
		return StateHigh
	case "repair":
		return StateRepair
	default:
		return StateUnknown
	}
}
