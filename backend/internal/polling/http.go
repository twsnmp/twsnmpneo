package polling

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// HTTPPoller checks web service endpoints with support for metrics, hashing, extractors, and scripting.
type HTTPPoller struct {
	client *http.Client
}

func NewHTTPPoller() *HTTPPoller {
	return &HTTPPoller{
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true}, //nolint:gosec
			},
		},
	}
}

func (p *HTTPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	mode := strings.ToLower(pe.Mode)
	switch mode {
	case "", "http", "https", "hash", "metrics", "apache", "nginx", "fiber":
	default:
		return &Result{
			State:   StateUnknown,
			Message: fmt.Sprintf("unsupported http mode: %s", pe.Mode),
			Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported http mode: %s", pe.Mode)},
		}, nil
	}
	url := pe.Params
	if url == "" {
		if node != nil && node.URL != "" {
			url = node.URL
		} else if node != nil && node.IP != "" {
			url = "http://" + node.IP
		} else {
			return &Result{
				State:   StateHigh,
				Message: "missing target URL or IP",
			}, nil
		}
	}

	if !strings.HasPrefix(url, "http://") && !strings.HasPrefix(url, "https://") {
		url = "http://" + url
	}

	timeoutSec := pe.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 5
	}
	timeout := time.Duration(timeoutSec) * time.Second

	var rtt time.Duration
	var resp *http.Response
	var bodyBytes []byte
	var lastErr error
	ok := false

	for i := 0; !ok && i <= pe.Retry; i++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
		if err != nil {
			lastErr = err
			continue
		}

		p.client.Timeout = timeout
		start := time.Now()
		resp, err = p.client.Do(req)
		rtt = time.Since(start)

		if err != nil {
			lastErr = err
			continue
		}

		bodyBytes, err = io.ReadAll(io.LimitReader(resp.Body, 10*1024*1024))
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
			Message: fmt.Sprintf("http request to %s failed: %v", url, lastErr),
			Fields:  fields,
		}, nil
	}

	statusCode := resp.StatusCode
	bodyStr := string(bodyBytes)
	fields["status"] = strconv.Itoa(statusCode)
	fields["code"] = float64(statusCode)

	// Status code >= 400 check
	if statusCode >= 400 {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("http status %d from %s", statusCode, url),
			Fields:  fields,
		}, nil
	}

	// Filter regex check (backward compatibility)
	if pe.Filter != "" {
		matched, err := regexp.MatchString(pe.Filter, bodyStr)
		if err != nil || !matched {
			return &Result{
				State:   StateWarn,
				RTT:     rtt,
				Message: fmt.Sprintf("response body does not match regex '%s'", pe.Filter),
				Fields:  fields,
			}, nil
		}
	}

	// Mode handling: hash change detection
	if pe.Mode == "hash" {
		nh := getHash(bodyStr)
		fields["sha256"] = nh
		oldHash := ""
		if pe.Result != nil {
			if oh, ok := pe.Result["sha256"].(string); ok && oh != "" {
				oldHash = oh
			}
		}
		if oldHash == "" {
			fields["first_sha256"] = nh
			fields["last_sha256"] = nh
		} else {
			fields["last_sha256"] = oldHash
			if pe.Result != nil {
				if fh, ok := pe.Result["first_sha256"].(string); ok && fh != "" {
					fields["first_sha256"] = fh
				} else {
					fields["first_sha256"] = oldHash
				}
			} else {
				fields["first_sha256"] = oldHash
			}
			if pe.Script == "" && oldHash != nh {
				return &Result{
					State:   failureState(pe.Level),
					RTT:     rtt,
					Message: fmt.Sprintf("http body hash changed: %s -> %s", oldHash[:8], nh[:8]),
					Fields:  fields,
				}, nil
			}
		}
	}

	// Mode handling: metrics parsing (Apache, Nginx, Fiber)
	if strings.Contains(pe.Mode, "metrics") {
		if m, err := getMetrics(bodyStr); err == nil {
			for k, v := range m {
				fields[k] = v
			}
		}
	}

	// Set up Otto VM for extractor and script evaluation
	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(bodyStr, vm)
	_ = vm.Set("status", strconv.Itoa(statusCode))
	_ = vm.Set("code", statusCode)
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))
	_ = vm.Set("interval", float64(pe.PollInt))

	// Extractor support (getBody, jsonpath, goquery, grok)
	if pe.Extractor != "" {
		if err := extractor.ApplyExtractor(pe.Extractor, bodyStr, vm, fields); err != nil {
			fields["error"] = err.Error()
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("http extractor error: %v", err),
				Fields:  fields,
			}, nil
		}
	}

	// No script: standard success
	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("http %d ok, rtt=%v", statusCode, rtt),
			Fields:  fields,
		}, nil
	}

	// Run Otto Script
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	value, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("http script error: %v", err),
			Fields:  fields,
		}, nil
	}

	pass, _ := value.ToBoolean()
	if pass {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("http %d script passed, rtt=%v", statusCode, rtt),
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   failureState(pe.Level),
		RTT:     rtt,
		Message: fmt.Sprintf("http %d script returned false", statusCode),
		Fields:  fields,
	}, nil
}

func getHash(body string) string {
	return fmt.Sprintf("%x", sha256.Sum256([]byte(body)))
}

type fiberMetricsEnt struct {
	PID struct {
		CPU   float64 `json:"cpu"`
		RAM   float64 `json:"ram"`
		Conns float64 `json:"conns"`
	} `json:"pid"`
	OS struct {
		CPU      float64 `json:"cpu"`
		RAM      float64 `json:"ram"`
		Conns    float64 `json:"conns"`
		TotalRAM float64 `json:"total_ram"`
		LoadAvg  float64 `json:"load_avg"`
	} `json:"os"`
}

var numReg = regexp.MustCompile(`[\.0-9]`)
var nginxAConnsReg = regexp.MustCompile(`Active connections: (\d+)`)
var nginxAHRReg = regexp.MustCompile(`\s*(\d+)\s+(\d+)\s+(\d+)`)
var nginxRWWReg = regexp.MustCompile(`Reading:\s*(\d+)\s+Writing:\s*(\d+)Waiting:\s*(\d+)`)

func getMetrics(body string) (map[string]interface{}, error) {
	r := make(map[string]interface{})
	if strings.Contains(body, "Apache") {
		for _, l := range strings.Split(body, "\n") {
			l = strings.TrimSpace(l)
			a := strings.SplitN(l, ": ", 2)
			if len(a) != 2 {
				continue
			}
			if numReg.MatchString(a[1]) {
				if v, err := strconv.ParseFloat(a[1], 64); err == nil {
					r[a[0]] = v
					continue
				}
			}
			r[a[0]] = a[1]
		}
	} else if strings.Contains(body, "Active conn") {
		if a := nginxAConnsReg.FindStringSubmatch(body); len(a) == 2 && a[1] != "" {
			if v, err := strconv.ParseFloat(a[1], 64); err == nil {
				r["active_connections"] = v
			}
		}
		if a := nginxAHRReg.FindStringSubmatch(body); len(a) == 4 && a[1] != "" {
			if v, err := strconv.ParseFloat(a[1], 64); err == nil {
				r["accepts"] = v
			}
			if v, err := strconv.ParseFloat(a[2], 64); err == nil {
				r["handled"] = v
			}
			if v, err := strconv.ParseFloat(a[3], 64); err == nil {
				r["requests"] = v
			}
		}
		if a := nginxRWWReg.FindStringSubmatch(body); len(a) == 4 && a[1] != "" {
			if v, err := strconv.ParseFloat(a[1], 64); err == nil {
				r["reading"] = v
			}
			if v, err := strconv.ParseFloat(a[2], 64); err == nil {
				r["writing"] = v
			}
			if v, err := strconv.ParseFloat(a[3], 64); err == nil {
				r["waiting"] = v
			}
		}
	} else if strings.Contains(body, `{"pid"`) {
		var s fiberMetricsEnt
		if err := json.Unmarshal([]byte(strings.TrimSpace(body)), &s); err == nil {
			r["pid_cpu"] = s.PID.CPU
			r["pid_ram"] = s.PID.RAM
			r["pid_conns"] = s.PID.Conns
			r["os_cpu"] = s.OS.CPU
			r["os_ram"] = s.OS.RAM
			r["os_load_avg"] = s.OS.LoadAvg
		}
	}
	return r, nil
}
