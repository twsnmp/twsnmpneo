package pki

import (
	"bytes"
	"context"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math/big"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	jose "github.com/go-jose/go-jose/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

// Helper CBOR encoder for constructing test attestation objects.
func encodeCBORUint(major byte, n uint64) []byte {
	prefix := major << 5
	switch {
	case n < 24:
		return []byte{prefix | byte(n)}
	case n <= 0xff:
		return []byte{prefix | 24, byte(n)}
	case n <= 0xffff:
		b := make([]byte, 3)
		b[0] = prefix | 25
		binary.BigEndian.PutUint16(b[1:], uint16(n))
		return b
	default:
		b := make([]byte, 5)
		b[0] = prefix | 26
		binary.BigEndian.PutUint32(b[1:], uint32(n))
		return b
	}
}

func encodeCBORBytes(b []byte) []byte {
	out := encodeCBORUint(2, uint64(len(b)))
	return append(out, b...)
}

func encodeCBORText(s string) []byte {
	out := encodeCBORUint(3, uint64(len(s)))
	return append(out, []byte(s)...)
}

func encodeCBORAttObj(format string, x5c [][]byte, sig []byte) []byte {
	var buf bytes.Buffer
	// Map of 2 items: "fmt", "attStmt"
	buf.Write(encodeCBORUint(5, 2))
	buf.Write(encodeCBORText("fmt"))
	buf.Write(encodeCBORText(format))

	buf.Write(encodeCBORText("attStmt"))
	stmtCount := uint64(1)
	if len(sig) > 0 {
		stmtCount = 2
	}
	buf.Write(encodeCBORUint(5, stmtCount))
	buf.Write(encodeCBORText("x5c"))
	buf.Write(encodeCBORUint(4, uint64(len(x5c))))
	for _, certDER := range x5c {
		buf.Write(encodeCBORBytes(certDER))
	}
	if len(sig) > 0 {
		buf.Write(encodeCBORText("sig"))
		buf.Write(encodeCBORBytes(sig))
	}
	return buf.Bytes()
}

func TestHTTP01ChallengeHostAndReverseAddr(t *testing.T) {
	if got := http01ChallengeHost("example.com"); got != "example.com" {
		t.Fatalf("expected example.com, got %s", got)
	}
	if got := http01ChallengeHost("192.168.1.10"); got != "192.168.1.10" {
		t.Fatalf("expected 192.168.1.10, got %s", got)
	}
	if got := http01ChallengeHost("2001:db8::1"); got != "[2001:db8::1]" {
		t.Fatalf("expected [2001:db8::1], got %s", got)
	}

	chIPv4 := &challenge{Value: "192.0.2.1"}
	if got := serverName(chIPv4); got != "1.2.0.192.in-addr.arpa." {
		t.Fatalf("unexpected IPv4 reverseAddr: %s", got)
	}

	chIPv6 := &challenge{Value: "2001:db8::1"}
	wantIPv6 := "1.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.0.8.b.d.0.1.0.0.2.ip6.arpa."
	if got := serverName(chIPv6); got != wantIPv6 {
		t.Fatalf("unexpected IPv6 reverseAddr: got %s, want %s", got, wantIPv6)
	}
}

func TestCheckCSR_DNSWildcard_IPv6_PermanentIdentifier(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-acme-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New: %v", err)
	}
	defer store.Close()

	mgr, err := New(store)
	if err != nil {
		t.Fatalf("pki.New: %v", err)
	}

	ctx := context.Background()

	nodeDNS := &datastore.NodeEnt{
		ID:   "node-dns",
		Name: "example.com",
		IP:   "192.168.10.20",
	}
	if err := store.SaveNode(ctx, nodeDNS); err != nil {
		t.Fatalf("SaveNode nodeDNS: %v", err)
	}
	nodeIPv6 := &datastore.NodeEnt{
		ID:   "node-ipv6",
		Name: "ipv6-host",
		IP:   "2001:db8::10",
	}
	if err := store.SaveNode(ctx, nodeIPv6); err != nil {
		t.Fatalf("SaveNode nodeIPv6: %v", err)
	}
	nodeDevice := &datastore.NodeEnt{
		ID:    "node-device",
		Name:  "macbook-pro",
		IP:    "192.168.10.30",
		Descr: "device-serial:C02XYZ123456",
	}
	if err := store.SaveNode(ctx, nodeDevice); err != nil {
		t.Fatalf("SaveNode nodeDevice: %v", err)
	}

	// 1. Wildcard DNS matching (*.example.com -> example.com)
	csrWildcard := &x509.CertificateRequest{
		DNSNames: []string{"*.example.com"},
	}
	if nodeID, err := mgr.checkCSR("acme", csrWildcard, map[string]string{}); err != nil || nodeID != nodeDNS.ID {
		t.Fatalf("expected wildcard CSR *.example.com to match nodeDNS (%s), got id=%s err=%v", nodeDNS.ID, nodeID, err)
	}

	// 2. IPv6 matching
	csrIPv6 := &x509.CertificateRequest{
		IPAddresses: []net.IP{net.ParseIP("2001:db8::10")},
	}
	if nodeID, err := mgr.checkCSR("acme", csrIPv6, map[string]string{}); err != nil || nodeID != nodeIPv6.ID {
		t.Fatalf("expected IPv6 CSR 2001:db8::10 to match nodeIPv6 (%s), got id=%s err=%v", nodeIPv6.ID, nodeID, err)
	}

	// 3. PermanentIdentifier matching via Descr
	csrDevice := &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "C02XYZ123456"},
	}
	if nodeID, err := mgr.checkCSR("acme", csrDevice, map[string]string{"PermanentIdentifier": "C02XYZ123456"}); err != nil || nodeID != nodeDevice.ID {
		t.Fatalf("expected PermanentIdentifier C02XYZ123456 to match nodeDevice (%s), got id=%s err=%v", nodeDevice.ID, nodeID, err)
	}

	// 4. Unknown PermanentIdentifier should fail
	if _, err := mgr.checkCSR("acme", csrDevice, map[string]string{"PermanentIdentifier": "UNKNOWN-SERIAL-999"}); err == nil {
		t.Fatalf("expected unknown PermanentIdentifier to fail checkCSR")
	}
}

func TestBuildPermanentIdentifierSANExt(t *testing.T) {
	ext, err := buildPermanentIdentifierSANExt("C02XYZ123456", nil, nil)
	if err != nil {
		t.Fatalf("buildPermanentIdentifierSANExt failed: %v", err)
	}
	wantSAN := asn1.ObjectIdentifier{2, 5, 29, 17}
	if !ext.Id.Equal(wantSAN) {
		t.Fatalf("expected OID 2.5.29.17, got %v", ext.Id)
	}
	if !bytes.Contains(ext.Value, []byte("C02XYZ123456")) {
		t.Fatalf("expected SAN extension value to contain C02XYZ123456")
	}
	// Verify OID 1.3.6.1.5.5.7.8.3 (id-on-permanentIdentifier) is encoded inside
	oidBytes, err := asn1.Marshal(asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 8, 3})
	if err != nil {
		t.Fatalf("marshal OID failed: %v", err)
	}
	if !bytes.Contains(ext.Value, oidBytes) {
		t.Fatalf("expected SAN extension to contain id-on-permanentIdentifier OID 1.3.6.1.5.5.7.8.3")
	}
}

func TestDeviceAttest01Validate_AppleAndStep(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-acme-attest-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New: %v", err)
	}
	defer store.Close()

	mgr, err := New(store)
	if err != nil {
		t.Fatalf("pki.New: %v", err)
	}

	ctx := context.Background()

	// Create account key
	accPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate account key: %v", err)
	}
	accJWK := &jose.JSONWebKey{Key: &accPriv.PublicKey}
	acc := &account{
		ID:     "test-acc",
		Key:    accJWK,
		Status: "valid",
	}

	// Create test Attestation Root CA
	rootPriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate root key: %v", err)
	}
	rootTpl := &x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "Test Attestation Root CA"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(24 * time.Hour),
		KeyUsage:              x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		BasicConstraintsValid: true,
		IsCA:                  true,
	}
	rootDER, err := x509.CreateCertificate(rand.Reader, rootTpl, rootTpl, &rootPriv.PublicKey, rootPriv)
	if err != nil {
		t.Fatalf("create root cert: %v", err)
	}
	rootCert, err := x509.ParseCertificate(rootDER)
	if err != nil {
		t.Fatalf("parse root cert: %v", err)
	}
	pool := x509.NewCertPool()
	pool.AddCert(rootCert)
	customAttestationRoots = pool
	defer func() {
		customAttestationRoots = nil
	}()

	// Attested device key (will also be used for CSR)
	devicePriv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generate device key: %v", err)
	}

	token := "test-challenge-token-12345"
	keyAuth, err := keyAuthorization(token, acc.Key)
	if err != nil {
		t.Fatalf("keyAuthorization: %v", err)
	}
	keyAuthHash := sha256.Sum256([]byte(keyAuth))

	// Register device nodes
	appleNode := &datastore.NodeEnt{
		ID:    "apple-node-1",
		Name:  "apple-device-node",
		IP:    "192.168.1.50",
		Descr: "C02APPLE1234",
	}
	if err := store.SaveNode(ctx, appleNode); err != nil {
		t.Fatalf("SaveNode appleNode: %v", err)
	}

	stepNode := &datastore.NodeEnt{
		ID:    "step-node-1",
		Name:  "step-device-node",
		IP:    "192.168.1.51",
		Descr: "serial:98765432",
	}
	if err := store.SaveNode(ctx, stepNode); err != nil {
		t.Fatalf("SaveNode stepNode: %v", err)
	}

	// --- 1. Test Apple Attestation Format ---
	serialValue, _ := asn1.Marshal("C02APPLE1234")
	nonceValue, _ := asn1.Marshal(keyAuthHash[:])

	appleLeafTpl := &x509.Certificate{
		SerialNumber: big.NewInt(2),
		Subject:      pkix.Name{CommonName: "Apple Managed Device"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			{
				Id:    asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 9, 1},
				Value: serialValue,
			},
			{
				Id:    asn1.ObjectIdentifier{1, 2, 840, 113635, 100, 8, 11, 1},
				Value: nonceValue,
			},
		},
	}
	appleLeafDER, err := x509.CreateCertificate(rand.Reader, appleLeafTpl, rootCert, &devicePriv.PublicKey, rootPriv)
	if err != nil {
		t.Fatalf("create apple leaf cert: %v", err)
	}

	appleCBOR := encodeCBORAttObj("apple", [][]byte{appleLeafDER}, nil)
	applePayload, _ := json.Marshal(deviceAttestPayload{
		AttObj: base64.RawURLEncoding.EncodeToString(appleCBOR),
	})

	azApple := &authorization{
		ID:         "az-apple",
		Status:     "pending",
		Identifier: identifier{Type: "permanent-identifier", Value: "C02APPLE1234"},
	}
	chApple := &challenge{
		ID:     "ch-apple",
		Type:   "device-attest-01",
		Status: "pending",
		Token:  token,
		Value:  "C02APPLE1234",
	}

	if err := mgr.deviceAttest01Validate(acc, azApple, chApple, applePayload); err != nil {
		t.Fatalf("deviceAttest01Validate (apple) failed: %v", err)
	}
	if chApple.Status != "valid" {
		t.Fatalf("expected chApple status valid, got %s", chApple.Status)
	}
	expectedFP, err := publicKeyFingerprint(&devicePriv.PublicKey)
	if err != nil {
		t.Fatalf("publicKeyFingerprint: %v", err)
	}
	if azApple.Fingerprint != expectedFP {
		t.Fatalf("expected fingerprint %s, got %s", expectedFP, azApple.Fingerprint)
	}

	// Verify mismatched device identifier fails
	azWrongID := &authorization{
		ID:         "az-wrong",
		Status:     "pending",
		Identifier: identifier{Type: "permanent-identifier", Value: "WRONG-SERIAL"},
	}
	if err := mgr.deviceAttest01Validate(acc, azWrongID, chApple, applePayload); err == nil {
		t.Fatalf("expected deviceAttest01Validate to fail for mismatched serial")
	}

	// --- 2. Test Step / YubiKey Attestation Format ---
	yubiSerialValue, _ := asn1.Marshal(int64(98765432))
	stepLeafTpl := &x509.Certificate{
		SerialNumber: big.NewInt(3),
		Subject:      pkix.Name{CommonName: "YubiKey PIV Attestation 9a"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
		KeyUsage:     x509.KeyUsageDigitalSignature,
		ExtraExtensions: []pkix.Extension{
			{
				Id:    asn1.ObjectIdentifier{1, 3, 6, 1, 4, 1, 41482, 3, 7},
				Value: yubiSerialValue,
			},
		},
	}
	stepLeafDER, err := x509.CreateCertificate(rand.Reader, stepLeafTpl, rootCert, &devicePriv.PublicKey, rootPriv)
	if err != nil {
		t.Fatalf("create step leaf cert: %v", err)
	}
	sig, err := ecdsa.SignASN1(rand.Reader, devicePriv, keyAuthHash[:])
	if err != nil {
		t.Fatalf("sign keyAuthHash: %v", err)
	}
	stepCBOR := encodeCBORAttObj("step", [][]byte{stepLeafDER}, sig)
	stepPayload, _ := json.Marshal(deviceAttestPayload{
		AttObj: base64.RawURLEncoding.EncodeToString(stepCBOR),
	})

	azStep := &authorization{
		ID:         "az-step",
		Status:     "pending",
		Identifier: identifier{Type: "permanent-identifier", Value: "98765432"},
	}
	chStep := &challenge{
		ID:     "ch-step",
		Type:   "device-attest-01",
		Status: "pending",
		Token:  token,
		Value:  "98765432",
	}
	if err := mgr.deviceAttest01Validate(acc, azStep, chStep, stepPayload); err != nil {
		t.Fatalf("deviceAttest01Validate (step) failed: %v", err)
	}
	if chStep.Status != "valid" || azStep.Fingerprint != expectedFP {
		t.Fatalf("step attestation validation did not set valid status and fingerprint")
	}

	// --- 3. Test Certificate Creation with PermanentIdentifier & IP in CommonName ---
	mgr.mu.Lock()
	mgr.rootCAPrivateKey = rootPriv
	mgr.rootCAPublicKey = &rootPriv.PublicKey
	mgr.rootCACertificate = rootDER
	mgr.conf.CertTerm = 24
	mgr.mu.Unlock()

	csrDER, err := x509.CreateCertificateRequest(rand.Reader, &x509.CertificateRequest{
		Subject: pkix.Name{CommonName: "192.168.1.50"},
	}, devicePriv)
	if err != nil {
		t.Fatalf("CreateCertificateRequest: %v", err)
	}

	mgr.mu.Lock()
	certDER, _, err := mgr.createCertificateFromCSR(csrDER, "acme", map[string]string{
		"RemoteAddr":          "192.168.1.50",
		"PermanentIdentifier": "C02APPLE1234",
	})
	mgr.mu.Unlock()
	if err != nil {
		t.Fatalf("createCertificateFromCSR failed: %v", err)
	}
	issuedCert, err := x509.ParseCertificate(certDER)
	if err != nil {
		t.Fatalf("failed to parse issued certificate: %v", err)
	}
	if len(issuedCert.IPAddresses) != 1 || !issuedCert.IPAddresses[0].Equal(net.ParseIP("192.168.1.50")) {
		t.Fatalf("expected IP 192.168.1.50 in SAN IPAddresses, got %v", issuedCert.IPAddresses)
	}
	foundPermID := false
	for _, ext := range issuedCert.Extensions {
		if ext.Id.Equal(asn1.ObjectIdentifier{2, 5, 29, 17}) && strings.Contains(string(ext.Value), "C02APPLE1234") {
			foundPermID = true
			break
		}
	}
	if !foundPermID {
		t.Fatalf("expected issued certificate SAN extension to contain PermanentIdentifier C02APPLE1234")
	}
}

func TestACME_ErrorResponse_AndDynamicBaseURL(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-acme-err-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New: %v", err)
	}
	defer store.Close()

	mgr, err := New(store)
	if err != nil {
		t.Fatalf("pki.New: %v", err)
	}

	err = mgr.CreateCA(&datastore.CreateCAReq{
		Name:          "TestACMEErrorCA",
		RootCAKeyType: "ecdsa-256",
		AcmePort:      18083,
		AcmeBaseURL:   "https://localhost:18083",
	})
	if err != nil {
		t.Fatalf("CreateCA: %v", err)
	}

	mgr.StartAcmeServer()
	defer mgr.StopAcmeServer()
	time.Sleep(50 * time.Millisecond)

	// 1. Check Directory with dynamic Host
	echoApp := mgr.acmeServer
	if echoApp == nil {
		t.Fatalf("expected acmeServer to be initialized")
	}
	req := httptest.NewRequest(http.MethodGet, "https://YMIM2ProMacMini.local:8083/directory", nil)
	rec := httptest.NewRecorder()
	echoApp.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("directory request failed: %d", rec.Code)
	}
	var d directory
	if err := json.Unmarshal(rec.Body.Bytes(), &d); err != nil {
		t.Fatalf("unmarshal directory: %v", err)
	}
	if !strings.HasPrefix(d.NewOrder, "https://YMIM2ProMacMini.local:8083") {
		t.Fatalf("expected NewOrder to use request Host YMIM2ProMacMini.local:8083, got %s", d.NewOrder)
	}

	// 2. Test unknown account on /new-order returns RFC 8555 accountDoesNotExist problem JSON
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("GenerateKey: %v", err)
	}
	signer, err := jose.NewSigner(jose.SigningKey{Algorithm: jose.ES256, Key: key}, (&jose.SignerOptions{}).
		WithHeader(jose.HeaderKey("url"), "https://YMIM2ProMacMini.local:8083/new-order").
		WithHeader(jose.HeaderKey("nonce"), createNonce()).
		WithHeader(jose.HeaderKey("kid"), "https://YMIM2ProMacMini.local:8083/account/non-existent-acc-id"))
	if err != nil {
		t.Fatalf("NewSigner: %v", err)
	}
	orderPayload := `{"identifiers":[{"type":"dns","value":"YMIM2ProMacMini.local"}]}`
	signed, err := signer.Sign([]byte(orderPayload))
	if err != nil {
		t.Fatalf("Sign: %v", err)
	}
	req = httptest.NewRequest(http.MethodPost, "https://YMIM2ProMacMini.local:8083/new-order", strings.NewReader(signed.FullSerialize()))
	req.Header.Set("Content-Type", "application/jose+json")
	rec = httptest.NewRecorder()
	echoApp.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("expected status 400 for unknown account, got %d", rec.Code)
	}
	if ctype := rec.Header().Get("Content-Type"); !strings.Contains(ctype, "application/problem+json") {
		t.Fatalf("expected Content-Type application/problem+json, got %s", ctype)
	}
	var aErr acmeError
	if err := json.Unmarshal(rec.Body.Bytes(), &aErr); err != nil {
		t.Fatalf("unmarshal acmeError: %v, raw: %s", err, rec.Body.String())
	}
	if aErr.Type != "urn:ietf:params:acme:error:accountDoesNotExist" {
		t.Fatalf("expected error type urn:ietf:params:acme:error:accountDoesNotExist, got %s", aErr.Type)
	}
	if aErr.Status != 400 {
		t.Fatalf("expected status 400, got %d", aErr.Status)
	}
	if !strings.Contains(aErr.Detail, "non-existent-acc-id") {
		t.Fatalf("expected error detail to mention account id, got %s", aErr.Detail)
	}
}

func TestAcmeServerStopReleasesPort(t *testing.T) {
	dir, err := os.MkdirTemp("", "twsnmpneo-acme-port-test-*")
	if err != nil {
		t.Fatalf("MkdirTemp: %v", err)
	}
	defer os.RemoveAll(dir)

	store, err := bbolt.New(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("bbolt.New: %v", err)
	}
	defer store.Close()

	mgr, err := New(store)
	if err != nil {
		t.Fatalf("pki.New: %v", err)
	}

	testPort := 18094
	err = mgr.CreateCA(&datastore.CreateCAReq{
		Name:          "TestPortCA",
		RootCAKeyType: "ecdsa-256",
		AcmePort:      testPort,
		AcmeBaseURL:   fmt.Sprintf("https://localhost:%d", testPort),
	})
	if err != nil {
		t.Fatalf("CreateCA: %v", err)
	}

	mgr.StartAcmeServer()

	addr := fmt.Sprintf("127.0.0.1:%d", testPort)
	var connected bool
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			connected = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !connected {
		t.Fatalf("ACME server did not start on %s", addr)
	}

	mgr.StopAcmeServer()

	var released bool
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err != nil {
			released = true
			break
		}
		conn.Close()
		time.Sleep(50 * time.Millisecond)
	}

	if !released {
		t.Fatalf("ACME server port %d was NOT released after StopAcmeServer()", testPort)
	}

	// Verify server can be restarted on the same port
	mgr.StartAcmeServer()
	connected = false
	for i := 0; i < 20; i++ {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			connected = true
			break
		}
		time.Sleep(50 * time.Millisecond)
	}
	if !connected {
		t.Fatalf("ACME server could not be restarted on %s", addr)
	}
	mgr.StopAcmeServer()
}

