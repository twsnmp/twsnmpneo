package notify

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestPublicOutboundIP(t *testing.T) {
	tests := []struct {
		ip      string
		allowed bool
	}{
		{ip: "8.8.8.8", allowed: true},
		{ip: "2001:4860:4860::8888", allowed: true},
		{ip: "10.0.0.1"},
		{ip: "127.0.0.1"},
		{ip: "169.254.169.254"},
		{ip: "100.64.0.1"},
		{ip: "192.0.2.1"},
		{ip: "::1"},
		{ip: "fc00::1"},
		{ip: "fe80::1"},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := publicOutboundIP(net.ParseIP(tt.ip)); got != tt.allowed {
				t.Errorf("publicOutboundIP(%q) = %v, want %v", tt.ip, got, tt.allowed)
			}
		})
	}
}

func TestSMTPOutboundIP(t *testing.T) {
	tests := []struct {
		ip      string
		allowed bool
	}{
		{ip: "8.8.8.8", allowed: true},
		{ip: "127.0.0.1", allowed: true},
		{ip: "10.0.0.1", allowed: true},
		{ip: "192.168.1.25", allowed: true},
		{ip: "::1", allowed: true},
		{ip: "fc00::1", allowed: true},
		{ip: "169.254.169.254"},
		{ip: "fe80::1"},
		{ip: "224.0.0.1"},
	}
	for _, tt := range tests {
		t.Run(tt.ip, func(t *testing.T) {
			if got := smtpOutboundIP(net.ParseIP(tt.ip)); got != tt.allowed {
				t.Errorf("smtpOutboundIP(%q) = %v, want %v", tt.ip, got, tt.allowed)
			}
		})
	}
}

func TestPublicDialContextRejectsPrivateIP(t *testing.T) {
	if _, err := publicDialContext(context.Background(), "tcp", "127.0.0.1:80"); err == nil {
		t.Fatal("expected private destination to be rejected")
	}
}

func TestValidateWebhookURL(t *testing.T) {
	for _, rawURL := range []string{
		"ftp://example.com/hook",
		"http://example@host/hook",
		"******example.com/hook",
		"/relative/path",
	} {
		if err := validateWebhookURL(rawURL); err == nil {
			t.Errorf("expected %q to be rejected", rawURL)
		}
	}
	for _, rawURL := range []string{
		"https://example.com/hook",
		"http://127.0.0.1/hook",
		"http://localhost:8080/hook",
		"http://192.168.1.25/hook",
		"http://[::1]/hook",
	} {
		if err := validateWebhookURL(rawURL); err != nil {
			t.Errorf("expected valid webhook URL %q: %v", rawURL, err)
		}
	}
}

func TestPostTestWebhookToLocalServer(t *testing.T) {
	received := false
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received = r.Method == http.MethodPost
		w.WriteHeader(http.StatusNoContent)
	}))
	defer server.Close()

	if err := PostTestWebhook(server.URL, []byte(`{"test":true}`)); err != nil {
		t.Fatalf("post test webhook to local server: %v", err)
	}
	if !received {
		t.Fatal("local server did not receive test webhook")
	}
}
