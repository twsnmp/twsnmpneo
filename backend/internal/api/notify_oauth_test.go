package api

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestNotifyOAuth2RedirectURL(t *testing.T) {
	tests := []struct {
		name    string
		host    string
		origin  string
		want    string
		wantErr bool
	}{
		{
			name:   "public HTTPS origin",
			host:   "monitor.example",
			origin: "https://monitor.example",
			want:   "https://monitor.example/api/notify/oauth2/callback",
		},
		{
			name:    "mismatched origin",
			host:    "monitor.example",
			origin:  "https://attacker.example",
			wantErr: true,
		},
		{
			name:    "public HTTP origin",
			host:    "monitor.example",
			origin:  "http://monitor.example",
			wantErr: true,
		},
		{
			name:    "missing origin",
			host:    "monitor.example",
			wantErr: true,
		},
		{
			name:   "localhost HTTP origin",
			host:   "localhost:5173",
			origin: "http://localhost:5173",
			want:   "http://localhost:5173/api/notify/oauth2/callback",
		},
		{
			name:   "localhost development proxy",
			host:   "127.0.0.1:8080",
			origin: "http://localhost:5173",
			want:   "http://localhost:5173/api/notify/oauth2/callback",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/api/notify/oauth2/start", nil)
			req.Host = tt.host
			if tt.origin != "" {
				req.Header.Set("Origin", tt.origin)
			}
			c := echo.New().NewContext(req, httptest.NewRecorder())
			got, err := notifyOAuth2RedirectURL(c)
			if (err != nil) != tt.wantErr {
				t.Fatalf("notifyOAuth2RedirectURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if err == nil && got != tt.want {
				t.Errorf("notifyOAuth2RedirectURL() = %q, want %q", got, tt.want)
			}
		})
	}
}
