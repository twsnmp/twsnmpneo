package certmgr

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/tls"
	"crypto/x509"
	"encoding/pem"
	"errors"
	"fmt"
	"os"
	"time"
)

// CertInfo contains metadata of the loaded active certificate.
type CertInfo struct {
	Subject      string    `json:"subject"`
	Issuer       string    `json:"issuer"`
	NotBefore    time.Time `json:"notBefore"`
	NotAfter     time.Time `json:"notAfter"`
	DaysLeft     int       `json:"daysLeft"`
	DNSNames     []string  `json:"dnsNames"`
	IPAddresses  []string  `json:"ipAddresses"`
	KeyType      string    `json:"keyType"`
	IsSelfSigned bool      `json:"isSelfSigned"`
}

// LoadCertKeyPair loads and parses a certificate and private key from files.
func LoadCertKeyPair(certFile, keyFile string) (*tls.Certificate, *CertInfo, error) {
	certPEM, err := os.ReadFile(certFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read cert file: %w", err)
	}

	keyPEM, err := os.ReadFile(keyFile)
	if err != nil {
		return nil, nil, fmt.Errorf("read key file: %w", err)
	}

	return ParseCertKeyPair(certPEM, keyPEM)
}

// ParseCertKeyPair parses PEM bytes into tls.Certificate and CertInfo.
func ParseCertKeyPair(certPEM, keyPEM []byte) (*tls.Certificate, *CertInfo, error) {
	cert, err := tls.X509KeyPair(certPEM, keyPEM)
	if err != nil {
		// Try custom parse in case of legacy PEM blocks or multiple blocks
		privKey, keyErr := parsePrivateKeyPEM(keyPEM)
		if keyErr != nil {
			return nil, nil, fmt.Errorf("parse key pair (standard: %v, private: %v)", err, keyErr)
		}
		var certs [][]byte
		rest := certPEM
		for len(rest) > 0 {
			var block *pem.Block
			block, rest = pem.Decode(rest)
			if block == nil {
				break
			}
			if block.Type == "CERTIFICATE" {
				certs = append(certs, block.Bytes)
			}
		}
		if len(certs) == 0 {
			return nil, nil, errors.New("no certificates found in PEM")
		}
		cert = tls.Certificate{
			Certificate: certs,
			PrivateKey:  privKey,
		}
	}

	if len(cert.Certificate) == 0 {
		return nil, nil, errors.New("empty certificate chain")
	}

	leaf, err := x509.ParseCertificate(cert.Certificate[0])
	if err != nil {
		return nil, nil, fmt.Errorf("parse x509 leaf: %w", err)
	}
	cert.Leaf = leaf

	info := extractCertInfo(leaf)
	return &cert, info, nil
}

func parsePrivateKeyPEM(keyPEM []byte) (crypto.PrivateKey, error) {
	for len(keyPEM) > 0 {
		var block *pem.Block
		block, keyPEM = pem.Decode(keyPEM)
		if block == nil {
			break
		}
		switch block.Type {
		case "RSA PRIVATE KEY":
			return x509.ParsePKCS1PrivateKey(block.Bytes)
		case "EC PRIVATE KEY":
			return x509.ParseECPrivateKey(block.Bytes)
		case "PRIVATE KEY":
			return x509.ParsePKCS8PrivateKey(block.Bytes)
		}
	}
	return nil, errors.New("unsupported or missing private key block in PEM")
}

func extractCertInfo(cert *x509.Certificate) *CertInfo {
	now := time.Now()
	days := int(cert.NotAfter.Sub(now).Hours() / 24)

	var keyType string
	switch cert.PublicKey.(type) {
	case *rsa.PublicKey:
		keyType = "RSA"
	case *ecdsa.PublicKey:
		keyType = "ECDSA"
	case ed25519.PublicKey:
		keyType = "Ed25519"
	default:
		keyType = "Unknown"
	}

	var ips []string
	for _, ip := range cert.IPAddresses {
		ips = append(ips, ip.String())
	}

	isSelfSigned := cert.Issuer.CommonName == cert.Subject.CommonName

	return &CertInfo{
		Subject:      cert.Subject.CommonName,
		Issuer:       cert.Issuer.CommonName,
		NotBefore:    cert.NotBefore,
		NotAfter:     cert.NotAfter,
		DaysLeft:     days,
		DNSNames:     cert.DNSNames,
		IPAddresses:  ips,
		KeyType:      keyType,
		IsSelfSigned: isSelfSigned,
	}
}
