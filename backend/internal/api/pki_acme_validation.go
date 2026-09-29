package api

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/tls"
	"crypto/x509"
	"encoding/asn1"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/pki"
)

var acmeIdentifierOID = asn1.ObjectIdentifier{1, 3, 6, 1, 5, 5, 7, 1, 31}

func validateACMEChallenge(ctx context.Context, challenge pki.ACMEChallenge) error {
	switch challenge.Type {
	case "http-01":
		return validateACMEHTTP01(ctx, challenge)
	case "dns-01":
		return validateACMEDNS01(ctx, challenge)
	case "tls-alpn-01":
		return validateACMETLSALPN01(ctx, challenge)
	default:
		return fmt.Errorf("unsupported ACME challenge type %q", challenge.Type)
	}
}

func validateACMEHTTP01(ctx context.Context, challenge pki.ACMEChallenge) error {
	host := strings.TrimSuffix(challenge.Identifier, ".")
	if host == "" || challenge.Token == "" || challenge.KeyAuthorization == "" {
		return fmt.Errorf("invalid HTTP-01 challenge")
	}
	u := url.URL{
		Scheme: "http",
		Host:   net.JoinHostPort(host, "80"),
		Path:   "/.well-known/acme-challenge/" + challenge.Token,
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	client := &http.Client{
		Timeout: 8 * time.Second,
		Transport: &http.Transport{
			Proxy:                  nil,
			DialContext:            dialer.DialContext,
			TLSHandshakeTimeout:    5 * time.Second,
			ResponseHeaderTimeout:  5 * time.Second,
			DisableKeepAlives:      true,
			MaxResponseHeaderBytes: 16 << 10,
		},
		CheckRedirect: func(*http.Request, []*http.Request) error {
			return http.ErrUseLastResponse
		},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u.String(), nil)
	if err != nil {
		return fmt.Errorf("create HTTP-01 validation request: %w", err)
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("HTTP-01 validation request failed: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("HTTP-01 endpoint returned status %d", resp.StatusCode)
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 4097))
	if err != nil {
		return fmt.Errorf("read HTTP-01 response: %w", err)
	}
	if len(body) > 4096 || string(bytes.TrimSpace(body)) != challenge.KeyAuthorization {
		return fmt.Errorf("HTTP-01 key authorization did not match")
	}
	return nil
}

func validateACMEDNS01(ctx context.Context, challenge pki.ACMEChallenge) error {
	host := strings.TrimSuffix(challenge.Identifier, ".")
	if host == "" || challenge.KeyAuthorization == "" {
		return fmt.Errorf("invalid DNS-01 challenge")
	}
	digest := sha256.Sum256([]byte(challenge.KeyAuthorization))
	expected := base64.RawURLEncoding.EncodeToString(digest[:])
	lookupCtx, cancel := context.WithTimeout(ctx, 8*time.Second)
	defer cancel()
	records, err := net.DefaultResolver.LookupTXT(lookupCtx, "_acme-challenge."+host)
	if err != nil {
		return fmt.Errorf("DNS-01 TXT lookup failed: %w", err)
	}
	for _, record := range records {
		if record == expected {
			return nil
		}
	}
	return fmt.Errorf("DNS-01 TXT record did not match")
}

func validateACMETLSALPN01(ctx context.Context, challenge pki.ACMEChallenge) error {
	host := strings.TrimSuffix(challenge.Identifier, ".")
	if host == "" || challenge.KeyAuthorization == "" {
		return fmt.Errorf("invalid TLS-ALPN-01 challenge")
	}
	expectedDigest := sha256.Sum256([]byte(challenge.KeyAuthorization))
	tlsConfig := &tls.Config{
		ServerName:         host,
		NextProtos:         []string{"acme-tls/1"},
		InsecureSkipVerify: true,
		VerifyConnection: func(state tls.ConnectionState) error {
			if state.NegotiatedProtocol != "acme-tls/1" || len(state.PeerCertificates) == 0 {
				return fmt.Errorf("TLS-ALPN endpoint did not negotiate acme-tls/1")
			}
			cert := state.PeerCertificates[0]
			if err := verifyACMETLSALPNCertificate(cert, host, expectedDigest, time.Now()); err != nil {
				return err
			}
			return nil
		},
	}
	dialer := &net.Dialer{Timeout: 5 * time.Second, KeepAlive: 30 * time.Second}
	conn, err := (&tls.Dialer{NetDialer: dialer, Config: tlsConfig}).DialContext(ctx, "tcp", net.JoinHostPort(host, "443"))
	if err != nil {
		return fmt.Errorf("TLS-ALPN validation failed: %w", err)
	}
	return conn.Close()
}

func verifyACMETLSALPNCertificate(cert *x509.Certificate, host string, expectedDigest [32]byte, now time.Time) error {
	if cert == nil || cert.IsCA || now.Before(cert.NotBefore) || now.After(cert.NotAfter) {
		return fmt.Errorf("TLS-ALPN certificate is invalid or outside its validity period")
	}
	if err := cert.VerifyHostname(host); err != nil {
		return fmt.Errorf("TLS-ALPN certificate does not identify the requested host: %w", err)
	}
	if err := cert.CheckSignatureFrom(cert); err != nil {
		return fmt.Errorf("TLS-ALPN certificate is not self-signed: %w", err)
	}
	var found bool
	for _, extension := range cert.Extensions {
		if !extension.Id.Equal(acmeIdentifierOID) {
			continue
		}
		if found || !extension.Critical {
			return fmt.Errorf("TLS-ALPN acmeIdentifier extension is duplicate or non-critical")
		}
		found = true
		var digest []byte
		rest, err := asn1.Unmarshal(extension.Value, &digest)
		if err != nil || len(rest) != 0 || !bytes.Equal(digest, expectedDigest[:]) {
			return errors.New("TLS-ALPN acmeIdentifier extension did not match")
		}
	}
	if !found {
		return fmt.Errorf("TLS-ALPN certificate is missing the acmeIdentifier extension")
	}
	return nil
}
