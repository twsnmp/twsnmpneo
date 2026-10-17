package certmgr

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"os"
	"strings"
	"time"
)

// GenerateSelfSignedCert generates an ECDSA P-256 self-signed certificate and private key.
func GenerateSelfSignedCert(host, ips string) ([]byte, []byte, error) {
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, nil, fmt.Errorf("generate private key: %w", err)
	}

	hostName, detectedIPs := detectHostAndIPs()
	if host == "" {
		host = hostName
	}

	subject := pkix.Name{
		CommonName:   host,
		Organization: []string{"TWSNMP NEO"},
	}

	notBefore := time.Now().Add(-1 * time.Hour) // clock skew safety
	notAfter := notBefore.Add(365 * 24 * time.Hour)

	serialNumberLimit := new(big.Int).Lsh(big.NewInt(1), 128)
	serialNumber, err := rand.Int(rand.Reader, serialNumberLimit)
	if err != nil {
		return nil, nil, fmt.Errorf("generate serial: %w", err)
	}

	template := x509.Certificate{
		SerialNumber:          serialNumber,
		Subject:               subject,
		NotBefore:             notBefore,
		NotAfter:              notAfter,
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IsCA:                  true,
	}

	dnsMap := map[string]bool{"localhost": true}
	if host != "" {
		dnsMap[host] = true
	}
	for dns := range dnsMap {
		template.DNSNames = append(template.DNSNames, dns)
	}

	ipMap := map[string]bool{"127.0.0.1": true, "::1": true}
	for _, ipStr := range detectedIPs {
		ipMap[ipStr] = true
	}
	if ips != "" {
		for _, part := range strings.Split(ips, ",") {
			part = strings.TrimSpace(part)
			if part != "" {
				ipMap[part] = true
			}
		}
	}

	for ipStr := range ipMap {
		if parsed := net.ParseIP(ipStr); parsed != nil {
			template.IPAddresses = append(template.IPAddresses, parsed)
		}
	}

	derBytes, err := x509.CreateCertificate(rand.Reader, &template, &template, &priv.PublicKey, priv)
	if err != nil {
		return nil, nil, fmt.Errorf("create certificate: %w", err)
	}

	certPem := pem.EncodeToMemory(&pem.Block{
		Type:  "CERTIFICATE",
		Bytes: derBytes,
	})

	keyBytes, err := x509.MarshalECPrivateKey(priv)
	if err != nil {
		return nil, nil, fmt.Errorf("marshal ec key: %w", err)
	}
	keyPem := pem.EncodeToMemory(&pem.Block{
		Type:  "EC PRIVATE KEY",
		Bytes: keyBytes,
	})

	return certPem, keyPem, nil
}

func detectHostAndIPs() (string, []string) {
	host, err := os.Hostname()
	if err != nil || host == "" {
		host = "twsnmpneo"
	}
	var ips []string
	ifs, err := net.Interfaces()
	if err != nil {
		return host, ips
	}
	for _, i := range ifs {
		if (i.Flags&net.FlagLoopback) != 0 || (i.Flags&net.FlagUp) == 0 {
			continue
		}
		addrs, err := i.Addrs()
		if err != nil {
			continue
		}
		for _, a := range addrs {
			ip, _, err := net.ParseCIDR(a.String())
			if err != nil || ip == nil {
				continue
			}
			if ip.To4() != nil {
				ips = append(ips, ip.String())
			}
		}
	}
	return host, ips
}
