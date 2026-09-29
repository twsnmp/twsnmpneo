package polling_test

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/polling"
	"golang.org/x/crypto/ssh"
)

func TestNTPPoller_Success(t *testing.T) {
	poller := polling.NewNTPPoller()
	ctx := context.Background()

	// Missing IP should return StateHigh
	res, err := poller.Poll(ctx, &datastore.PollingEnt{}, nil)
	if err != nil || res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on nil node, got %s", res.State)
	}

	// Unreachable IP should fail gracefully with StateHigh
	pe := &datastore.PollingEnt{Timeout: 1, Retry: 0}
	node := &datastore.NodeEnt{IP: "127.0.0.1"}
	res, err = poller.Poll(ctx, pe, node)
	if err != nil || res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on unreachable NTP server, got %s", res.State)
	}
}

func TestTCPPoller_TLSModes(t *testing.T) {
	// Generate self-signed cert for mock TLS server
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate key: %v", err)
	}
	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject: pkix.Name{
			CommonName:   "127.0.0.1",
			Organization: []string{"TWSNMP Test"},
		},
		NotBefore:             time.Now().Add(-1 * time.Hour),
		NotAfter:              time.Now().Add(30 * 24 * time.Hour), // 30 days
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1")},
	}
	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		t.Fatalf("failed to create cert: %v", err)
	}
	cert := tls.Certificate{
		Certificate: [][]byte{derBytes},
		PrivateKey:  priv,
	}

	tlsListener, err := tls.Listen("tcp", "127.0.0.1:0", &tls.Config{
		Certificates: []tls.Certificate{cert},
	})
	if err != nil {
		t.Fatalf("failed to start TLS server: %v", err)
	}
	defer tlsListener.Close()

	go func() {
		for {
			conn, err := tlsListener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				if tc, ok := c.(*tls.Conn); ok {
					_ = tc.Handshake()
				}
			}(conn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(tlsListener.Addr().String())
	poller := polling.NewTCPPoller()
	ctx := context.Background()
	node := &datastore.NodeEnt{IP: "127.0.0.1", Name: "127.0.0.1"}

	// 1. Verify TLS mode (insecureSkipVerify is false in "verify" mode unless trusted, so self-signed fails verify)
	peVerify := &datastore.PollingEnt{
		Type:    "tcp",
		Mode:    "verify",
		Params:  portStr,
		Timeout: 2,
	}
	res, _ := poller.Poll(ctx, peVerify, node)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on untrusted self-signed cert, got %s", res.State)
	}

	// 2. Version mode (check TLS connection without strict cert validation)
	peVersion := &datastore.PollingEnt{
		Type:    "tcp",
		Mode:    "version",
		Params:  portStr,
		Script:  "1.2",
		Timeout: 2,
	}
	res, err = poller.Poll(ctx, peVersion, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on TLS version mode, got %s (msg: %s, err: %v)", res.State, res.Message, err)
	}
	if res.Fields["cipherSuite"] == nil || res.Fields["cipherSuite"] == "" {
		t.Fatalf("expected cipherSuite field, got nil")
	}

	// 3. Expire mode: cert expires in 30 days. If script asks for 60 days, it should trigger warning/high
	peExpireFail := &datastore.PollingEnt{
		Type:    "tcp",
		Mode:    "expire",
		Params:  portStr,
		Script:  "60", // asks for at least 60 days remaining
		Timeout: 2,
	}
	res, _ = poller.Poll(ctx, peExpireFail, node)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh when cert expires within 60 days, got %s", res.State)
	}

	// 4. Expire mode: cert expires in 30 days. If script asks for 5 days, it should pass
	peExpirePass := &datastore.PollingEnt{
		Type:    "tcp",
		Mode:    "expire",
		Params:  portStr,
		Script:  "5", // only need 5 days
		Timeout: 2,
	}
	res, err = poller.Poll(ctx, peExpirePass, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal when cert has plenty of days, got %s (err: %v)", res.State, err)
	}
}

func TestDNSPoller_ModesAndScript(t *testing.T) {
	poller := polling.NewDNSPoller()
	ctx := context.Background()

	// 1. Resolve localhost
	pe := &datastore.PollingEnt{
		Type:    "dns",
		Mode:    "ipaddr",
		Params:  "localhost",
		Timeout: 2,
	}
	node := &datastore.NodeEnt{Name: "localhost"}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal for localhost, got %s (err: %v)", res.State, err)
	}
	if res.Fields["ip"] == nil || res.Fields["ip"] == "" {
		t.Fatalf("expected ip field in result, got empty")
	}

	// 2. IP change detection
	peChange := &datastore.PollingEnt{
		Type:    "dns",
		Mode:    "ipaddr",
		Params:  "localhost",
		Timeout: 2,
		Result: map[string]interface{}{
			"ip": "192.0.2.1", // previously fake IP
		},
	}
	res, _ = poller.Poll(ctx, peChange, node)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh when IP changes, got %s", res.State)
	}

	// 3. JS script evaluation
	peScript := &datastore.PollingEnt{
		Type:    "dns",
		Mode:    "host",
		Params:  "localhost",
		Script:  "count >= 1",
		Timeout: 2,
	}
	res, err = poller.Poll(ctx, peScript, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on script pass, got %s", res.State)
	}

	peScriptFail := &datastore.PollingEnt{
		Type:    "dns",
		Mode:    "host",
		Params:  "localhost",
		Script:  "count > 9999", // should fail
		Level:   "warn",
		Timeout: 2,
	}
	res, _ = poller.Poll(ctx, peScriptFail, node)
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on script fail, got %s", res.State)
	}
}

func TestTWSNMPPoller(t *testing.T) {
	// Mock TWSNMP server
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/mobile/api/mapstatus" {
			http.NotFound(w, r)
			return
		}
		u, p, ok := r.BasicAuth()
		if !ok || u != "admin" || p != "password" {
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
			return
		}
		resp := map[string]interface{}{
			"High":      0,
			"Low":       1,
			"Warn":      2,
			"Normal":    10,
			"Repair":    0,
			"Unknown":   0,
			"DBSize":    1048576,
			"DBSizeStr": "1.0 MB",
			"State":     "warn",
		}
		_ = json.NewEncoder(w).Encode(resp)
	}))
	defer server.Close()

	poller := polling.NewTWSNMPPoller()
	ctx := context.Background()

	// 1. Success with basic auth in params
	pe := &datastore.PollingEnt{
		Type:    "twsnmp",
		Params:  fmt.Sprintf("%s/mobile/api/mapstatus", server.URL),
		Timeout: 3,
	}
	node := &datastore.NodeEnt{
		User:     "admin",
		Password: "password",
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil || res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn from mock server, got %s (msg: %s, err: %v)", res.State, res.Message, err)
	}
	if res.Fields["normal"] != float64(10) || res.Fields["warn"] != float64(2) {
		t.Fatalf("unexpected fields: %+v", res.Fields)
	}

	// 2. Auth failure
	nodeBad := &datastore.NodeEnt{User: "wrong", Password: "credentials"}
	res, _ = poller.Poll(ctx, pe, nodeBad)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on auth error, got %s", res.State)
	}
}

func TestMonitorPoller(t *testing.T) {
	poller := polling.NewMonitorPoller(nil)
	ctx := context.Background()

	// 1. Plain monitoring without script
	pe := &datastore.PollingEnt{
		Type: "monitor",
	}
	res, err := poller.Poll(ctx, pe, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on monitor poll, got %s (err: %v)", res.State, err)
	}
	if res.Fields["cpu"] == nil || res.Fields["mem"] == nil {
		t.Fatalf("expected cpu and mem fields in monitor result")
	}

	// 2. Script passing
	pePass := &datastore.PollingEnt{
		Type:   "monitor",
		Script: "cpu >= 0 && mem >= 0",
	}
	res, err = poller.Poll(ctx, pePass, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on passing script, got %s", res.State)
	}

	// 3. Script failing
	peFail := &datastore.PollingEnt{
		Type:   "monitor",
		Script: "cpu > 9999",
		Level:  "warn",
	}
	res, _ = poller.Poll(ctx, peFail, nil)
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on failing script, got %s", res.State)
	}
}

func TestCmdPoller(t *testing.T) {
	poller := polling.NewCmdPoller()
	ctx := context.Background()
	node := &datastore.NodeEnt{IP: "127.0.0.1", Name: "My Test Node"}

	// 1. Success exit 0
	pe := &datastore.PollingEnt{
		Type:    "cmd",
		Params:  "echo hello $NODE",
		Timeout: 2,
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on echo, got %s (err: %v)", res.State, err)
	}

	// 2. Grok extractor + script
	peGrok := &datastore.PollingEnt{
		Type:      "cmd",
		Params:    "echo 'TEMP=42 HUM=60'",
		Extractor: "TEMP=%{NUMBER:temp} HUM=%{NUMBER:hum}",
		Script:    "temp == '42' && hum == '60'",
		Timeout:   2,
	}
	res, err = poller.Poll(ctx, peGrok, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on grok script match, got %s (err: %v, fields: %+v)", res.State, err, res.Fields)
	}

	// 3. Command failure non-zero exit code
	peFail := &datastore.PollingEnt{
		Type:    "cmd",
		Params:  "exit 2",
		Level:   "warn",
		Timeout: 2,
	}
	res, _ = poller.Poll(ctx, peFail, node)
	if res.State != polling.StateWarn {
		t.Fatalf("expected StateWarn on non-zero exit, got %s", res.State)
	}
}

func TestHTTPPoller_ExtractorsAndScript(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/json":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"server": "alpha", "load": 2.5}`))
		case "/html":
			w.Header().Set("Content-Type", "text/html")
			_, _ = w.Write([]byte(`<html><body><span id="ver">2.1.0</span></body></html>`))
		case "/hash":
			_, _ = w.Write([]byte("constant content"))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	poller := polling.NewHTTPPoller()
	ctx := context.Background()

	// 1. JSONPath Extractor
	peJSON := &datastore.PollingEnt{
		Type:      "http",
		Params:    server.URL + "/json",
		Extractor: "jsonpath",
		Script:    "jsonpath('$.server') === 'alpha' && jsonpath('$.load') === 2.5",
		Timeout:   2,
	}
	res, err := poller.Poll(ctx, peJSON, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on jsonpath, got %s (err: %v)", res.State, err)
	}

	// 2. GoQuery Extractor
	peHTML := &datastore.PollingEnt{
		Type:      "http",
		Params:    server.URL + "/html",
		Extractor: "goquery",
		Script:    "goquery('#ver') === '2.1.0'",
		Timeout:   2,
	}
	res, err = poller.Poll(ctx, peHTML, nil)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on goquery, got %s (err: %v)", res.State, err)
	}

	// 3. Hash mode - detects changes
	peHash := &datastore.PollingEnt{
		Type:    "http",
		Mode:    "hash",
		Params:  server.URL + "/hash",
		Timeout: 2,
		Result: map[string]interface{}{
			"sha256": "different_previous_hash_value",
		},
	}
	res, _ = poller.Poll(ctx, peHash, nil)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on hash mismatch, got %s", res.State)
	}
}

func TestSSHPoller(t *testing.T) {
	// Generate host key for mock SSH server
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate host key: %v", err)
	}
	signer, err := ssh.NewSignerFromKey(key)
	if err != nil {
		t.Fatalf("failed to create signer: %v", err)
	}

	config := &ssh.ServerConfig{
		PasswordCallback: func(c ssh.ConnMetadata, pass []byte) (*ssh.Permissions, error) {
			if c.User() == "testuser" && string(pass) == "testpass" {
				return nil, nil
			}
			return nil, fmt.Errorf("password rejected for %q", c.User())
		},
	}
	config.AddHostKey(signer)

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to listen: %v", err)
	}
	defer listener.Close()

	go func() {
		for {
			nConn, err := listener.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, chans, reqs, err := ssh.NewServerConn(c, config)
				if err != nil {
					return
				}
				go ssh.DiscardRequests(reqs)
				for newChannel := range chans {
					if newChannel.ChannelType() != "session" {
						_ = newChannel.Reject(ssh.UnknownChannelType, "unknown channel type")
						continue
					}
					channel, requests, err := newChannel.Accept()
					if err != nil {
						return
					}
					go func(ch ssh.Channel, in <-chan *ssh.Request) {
						defer ch.Close()
						for req := range in {
							if req.Type == "exec" {
								_ = req.Reply(true, nil)
								_, _ = ch.Write([]byte("SSH_OK: count=42\n"))
								// Send exit-status
								_, _ = ch.SendRequest("exit-status", false, []byte{0, 0, 0, 0})
								return
							}
						}
					}(channel, requests)
				}
			}(nConn)
		}
	}()

	_, portStr, _ := net.SplitHostPort(listener.Addr().String())
	poller := polling.NewSSHPoller()
	ctx := context.Background()

	node := &datastore.NodeEnt{
		IP:       "127.0.0.1",
		SSHUser:  "testuser",
		Password: "testpass",
	}

	// 1. Success with Grok extractor
	pe := &datastore.PollingEnt{
		Type:      "ssh",
		Params:    "status",
		Mode:      portStr,
		Extractor: "SSH_OK: count=%{NUMBER:count}",
		Script:    "count == '42'",
		Timeout:   3,
	}
	res, err := poller.Poll(ctx, pe, node)
	if err != nil || res.State != polling.StateNormal {
		t.Fatalf("expected StateNormal on SSH poll, got %s (err: %v, fields: %+v)", res.State, err, res.Fields)
	}

	// 2. Auth failure with wrong password
	nodeBad := &datastore.NodeEnt{
		IP:       "127.0.0.1",
		SSHUser:  "testuser",
		Password: "wrongpassword",
	}
	res, _ = poller.Poll(ctx, pe, nodeBad)
	if res.State != polling.StateHigh {
		t.Fatalf("expected StateHigh on SSH auth failure, got %s", res.State)
	}
}

func TestSNMPPoller_Modes(t *testing.T) {
	poller := polling.NewSNMPPoller()
	ctx := context.Background()
	node := &datastore.NodeEnt{
		IP:        "127.0.0.1",
		SnmpPort:  65534, // closed port
		Community: "public",
	}

	modes := []string{"sysUpTime", "ifOperStatus", "hrSystemDate", "count", "process", "stats", "traffic", "script"}
	for _, m := range modes {
		pe := &datastore.PollingEnt{
			Type:    "snmp",
			Mode:    m,
			Params:  "1",
			Timeout: 1,
			Level:   "warn",
		}
		res, err := poller.Poll(ctx, pe, node)
		if err != nil {
			t.Fatalf("unexpected error for mode %s: %v", m, err)
		}
		if res.State != polling.StateWarn && res.State != polling.StateHigh {
			t.Fatalf("expected failure state for mode %s on unreachable agent, got %s", m, res.State)
		}
	}
}



