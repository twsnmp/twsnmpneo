package pki

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/labstack/echo/v4"
)

func TestACMEJWSRejectsTamperingAndWrongURL(t *testing.T) {
	key, jwk := testACMEAccountKey(t)
	jws := signTestACMEJWS(t, key, jwk, "https://acme.example.test/acme/new-account", "fresh-nonce", []byte(`{}`))
	if _, err := decodeACMEJWS(jws, "https://acme.example.test/acme/new-account"); err != nil {
		t.Fatalf("valid JWS rejected: %v", err)
	}
	if _, err := decodeACMEJWS(jws, "https://acme.example.test/acme/new-order"); err == nil {
		t.Fatal("JWS with an incorrect protected url was accepted")
	}
	var envelope acmeJWS
	if err := json.Unmarshal(jws, &envelope); err != nil {
		t.Fatal(err)
	}
	sig, err := base64.RawURLEncoding.DecodeString(envelope.Signature)
	if err != nil {
		t.Fatal(err)
	}
	sig[0] ^= 0x80
	envelope.Signature = base64.RawURLEncoding.EncodeToString(sig)
	tampered, _ := json.Marshal(envelope)
	if _, err := decodeACMEJWS(tampered, "https://acme.example.test/acme/new-account"); err == nil {
		t.Fatal("tampered JWS signature was accepted")
	}
}

func TestACMECoreEndpointsAndUnsupportedChallenges(t *testing.T) {
	const base = "https://acme.example.test/acme"
	key, jwk := testACMEAccountKey(t)
	e := echo.New()
	registration, err := RegisterACME(e, &Manager{}, ACMEConfig{BaseURL: base})
	if err != nil {
		t.Fatal(err)
	}
	defer registration.Close()

	directoryResponse := serveACME(e, http.MethodGet, base+"/directory", nil)
	if directoryResponse.Code != http.StatusOK || directoryResponse.Header().Get("Replay-Nonce") == "" {
		t.Fatalf("directory status/header = %d / %q", directoryResponse.Code, directoryResponse.Header().Get("Replay-Nonce"))
	}
	var directory map[string]string
	if err := json.Unmarshal(directoryResponse.Body.Bytes(), &directory); err != nil {
		t.Fatalf("decode directory: %v", err)
	}
	if directory["newOrder"] != base+"/new-order" || directory["newNonce"] != base+"/new-nonce" {
		t.Fatalf("unexpected directory: %#v", directory)
	}

	newAccountURL := base + "/new-account"
	accountResponse := serveACME(e, http.MethodPost, newAccountURL, signTestACMEJWS(
		t, key, jwk, newAccountURL, directoryResponse.Header().Get("Replay-Nonce"), []byte(`{}`)))
	if accountResponse.Code != http.StatusCreated {
		t.Fatalf("new account status = %d, body=%s", accountResponse.Code, accountResponse.Body.String())
	}
	accountURL := accountResponse.Header().Get("Location")
	if !strings.HasPrefix(accountURL, base+"/account/") {
		t.Fatalf("missing account Location: %q", accountURL)
	}
	if accountResponse.Header().Get("Replay-Nonce") == "" {
		t.Fatal("new account response omitted Replay-Nonce")
	}
	replayResponse := serveACME(e, http.MethodPost, newAccountURL, signTestACMEJWS(
		t, key, jwk, newAccountURL, directoryResponse.Header().Get("Replay-Nonce"), []byte(`{}`)))
	if replayResponse.Code != http.StatusBadRequest || !strings.Contains(replayResponse.Body.String(), "badNonce") {
		t.Fatalf("replayed nonce not rejected: status=%d body=%s", replayResponse.Code, replayResponse.Body.String())
	}
	if replayResponse.Header().Get(echo.HeaderContentType) != "application/problem+json" {
		t.Fatalf("ACME error Content-Type = %q", replayResponse.Header().Get(echo.HeaderContentType))
	}

	newOrderURL := base + "/new-order"
	orderPayload := []byte(`{"identifiers":[{"type":"dns","value":"acme.example.test"}]}`)
	orderResponse := serveACME(e, http.MethodPost, newOrderURL, signTestACMEJWS(
		t, key, jwk, newOrderURL, accountResponse.Header().Get("Replay-Nonce"), orderPayload, accountURL))
	if orderResponse.Code != http.StatusCreated {
		t.Fatalf("new order status = %d, body=%s", orderResponse.Code, orderResponse.Body.String())
	}
	var order struct {
		Status         string   `json:"status"`
		Authorizations []string `json:"authorizations"`
	}
	if err := json.Unmarshal(orderResponse.Body.Bytes(), &order); err != nil {
		t.Fatal(err)
	}
	if order.Status != "pending" || len(order.Authorizations) != 1 {
		t.Fatalf("unexpected order response: %#v", order)
	}
	authzURL := order.Authorizations[0]
	authzResponse := serveACME(e, http.MethodPost, authzURL, signTestACMEJWS(
		t, key, jwk, authzURL, orderResponse.Header().Get("Replay-Nonce"), nil, accountURL))
	if authzResponse.Code != http.StatusOK {
		t.Fatalf("authorization status = %d, body=%s", authzResponse.Code, authzResponse.Body.String())
	}
	var authz struct {
		Challenges []struct {
			URL string `json:"url"`
		} `json:"challenges"`
	}
	if err := json.Unmarshal(authzResponse.Body.Bytes(), &authz); err != nil {
		t.Fatal(err)
	}
	if len(authz.Challenges) != 3 {
		t.Fatalf("expected three challenge resources, got %d", len(authz.Challenges))
	}
	challengeURL := authz.Challenges[0].URL
	challengeResponse := serveACME(e, http.MethodPost, challengeURL, signTestACMEJWS(
		t, key, jwk, challengeURL, authzResponse.Header().Get("Replay-Nonce"), []byte(`{}`), accountURL))
	if challengeResponse.Code != http.StatusOK || !strings.Contains(challengeResponse.Body.String(), `"status":"invalid"`) ||
		!strings.Contains(challengeResponse.Body.String(), "not configured") {
		t.Fatalf("unconfigured challenge was not explicitly rejected: status=%d body=%s", challengeResponse.Code, challengeResponse.Body.String())
	}
	orderURL := orderResponse.Header().Get("Location")
	orderStatusResponse := serveACME(e, http.MethodPost, orderURL, signTestACMEJWS(
		t, key, jwk, orderURL, challengeResponse.Header().Get("Replay-Nonce"), nil, accountURL))
	if orderStatusResponse.Code != http.StatusOK || !strings.Contains(orderStatusResponse.Body.String(), `"status":"invalid"`) {
		t.Fatalf("failed authorization did not invalidate its order: status=%d body=%s", orderStatusResponse.Code, orderStatusResponse.Body.String())
	}
	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "acme.example.test"}, DNSNames: []string{"acme.example.test"},
	}, key)
	if err != nil {
		t.Fatal(err)
	}
	finalizeURL := base + "/finalize/" + strings.TrimPrefix(orderURL, base+"/order/")
	finalizePayload, _ := json.Marshal(map[string]string{"csr": base64.RawURLEncoding.EncodeToString(csrDER)})
	finalizeResponse := serveACME(e, http.MethodPost, finalizeURL, signTestACMEJWS(
		t, key, jwk, finalizeURL, orderStatusResponse.Header().Get("Replay-Nonce"), finalizePayload, accountURL))
	if finalizeResponse.Code != http.StatusBadRequest || !strings.Contains(finalizeResponse.Body.String(), "orderNotReady") {
		t.Fatalf("finalize did not require valid authorizations: status=%d body=%s", finalizeResponse.Code, finalizeResponse.Body.String())
	}
}

func TestACMEBoundedRegistrationConfiguration(t *testing.T) {
	if _, err := RegisterACME(echo.New(), &Manager{}, ACMEConfig{}); err == nil {
		t.Fatal("registration without an advertised base URL succeeded")
	}
	if _, err := RegisterACME(echo.New(), &Manager{}, ACMEConfig{BaseURL: "http://example.test/acme"}); err == nil {
		t.Fatal("registration with insecure HTTP succeeded")
	}
	if _, err := RegisterACME(echo.New(), &Manager{}, ACMEConfig{BaseURL: "https://@example.test/acme"}); err == nil {
		t.Fatal("registration with credentials in BaseURL succeeded")
	}
	if _, err := normalizeACMEIdentifier(acmeIdentifier{Type: "dns", Value: "*.example.test"}); err == nil {
		t.Fatal("wildcard DNS identifier was accepted")
	}
}

func testACMEAccountKey(t *testing.T) (*ecdsa.PrivateKey, map[string]string) {
	t.Helper()
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	jwk := map[string]string{
		"kty": "EC", "crv": "P-256",
		"x": base64.RawURLEncoding.EncodeToString(key.X.FillBytes(make([]byte, 32))),
		"y": base64.RawURLEncoding.EncodeToString(key.Y.FillBytes(make([]byte, 32))),
	}
	return key, jwk
}

func signTestACMEJWS(t *testing.T, key *ecdsa.PrivateKey, jwk map[string]string, requestURL, nonce string, payload []byte, kid ...string) []byte {
	t.Helper()
	protected := map[string]any{"alg": "ES256", "nonce": nonce, "url": requestURL}
	if len(kid) != 0 {
		protected["kid"] = kid[0]
	} else {
		protected["jwk"] = jwk
	}
	protectedJSON, _ := json.Marshal(protected)
	payload64 := base64.RawURLEncoding.EncodeToString(payload)
	protected64 := base64.RawURLEncoding.EncodeToString(protectedJSON)
	digest := sha256.Sum256([]byte(protected64 + "." + payload64))
	r, s, err := ecdsa.Sign(rand.Reader, key, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	signature := append(r.FillBytes(make([]byte, 32)), s.FillBytes(make([]byte, 32))...)
	body, _ := json.Marshal(acmeJWS{
		Protected: protected64, Payload: payload64,
		Signature: base64.RawURLEncoding.EncodeToString(signature),
	})
	return body
}

func serveACME(e *echo.Echo, method, requestURL string, body []byte) *httptest.ResponseRecorder {
	request := httptest.NewRequest(method, requestURL, strings.NewReader(string(body)))
	request.Header.Set(echo.HeaderContentType, "application/jose+json")
	response := httptest.NewRecorder()
	e.ServeHTTP(response, request)
	return response
}
