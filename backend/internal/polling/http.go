package polling

import (
	"context"
	"crypto/tls"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// HTTPPoller checks web service endpoints.
type HTTPPoller struct {
	client *http.Client
}

func NewHTTPPoller() *HTTPPoller {
	return &HTTPPoller{
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
	}
}

func (p *HTTPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
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

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Second
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return &Result{
			State:   StateHigh,
			Message: fmt.Sprintf("invalid http request: %v", err),
		}, nil
	}

	start := time.Now()
	p.client.Timeout = timeout
	resp, err := p.client.Do(req)
	rtt := time.Since(start)

	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("http request to %s failed: %v", url, err),
			Fields: map[string]interface{}{
				"rtt": rtt.Milliseconds(),
			},
		}, nil
	}
	defer resp.Body.Close()

	bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 1024*1024))
	bodyStr := string(bodyBytes)

	// Status code check
	if resp.StatusCode >= 400 {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("http status %d from %s", resp.StatusCode, url),
			Fields: map[string]interface{}{
				"status": resp.StatusCode,
				"rtt":    rtt.Milliseconds(),
			},
		}, nil
	}

	// Filter regex check
	if pe.Filter != "" {
		matched, err := regexp.MatchString(pe.Filter, bodyStr)
		if err != nil || !matched {
			return &Result{
				State:   StateWarn,
				RTT:     rtt,
				Message: fmt.Sprintf("response body does not match regex '%s'", pe.Filter),
				Fields: map[string]interface{}{
					"status": resp.StatusCode,
					"rtt":    rtt.Milliseconds(),
				},
			}, nil
		}
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("http %d ok, rtt=%v", resp.StatusCode, rtt),
		Fields: map[string]interface{}{
			"status": strconv.Itoa(resp.StatusCode),
			"rtt":    rtt.Milliseconds(),
		},
	}, nil
}
