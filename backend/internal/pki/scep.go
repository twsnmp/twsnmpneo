package pki

import (
	"crypto/x509"
	"encoding/pem"
	"fmt"
	"net"
	"strings"

	"github.com/smallstep/scep"
)

func (m *Manager) SCEPCACertificates() ([]*x509.Certificate, error) {
	m.mu.RLock()
	defer m.mu.RUnlock()
	if m.caCert == nil || m.scepCert == nil {
		return nil, fmt.Errorf("SCEP CA not initialized")
	}
	return []*x509.Certificate{m.caCert, m.scepCert}, nil
}

func (m *Manager) ProcessSCEPMessage(data []byte, authorize func(*x509.CertificateRequest, string) error) ([]byte, error) {
	msg, err := scep.ParsePKIMessage(data)
	if err != nil {
		return nil, fmt.Errorf("parse SCEP request: %w", err)
	}
	m.mu.RLock()
	if m.caCert == nil || m.caKey == nil || m.scepCert == nil || m.scepKey == nil {
		m.mu.RUnlock()
		return nil, fmt.Errorf("SCEP CA not initialized")
	}
	caCert, caKey := m.scepCert, m.scepKey
	m.mu.RUnlock()

	if err := msg.DecryptPKIEnvelope(caCert, caKey); err != nil {
		return nil, fmt.Errorf("decrypt SCEP request: %w", err)
	}
	if msg.CSR == nil {
		return nil, fmt.Errorf("SCEP request has no certificate request")
	}
	if authorize == nil {
		return nil, fmt.Errorf("SCEP enrollment is not configured")
	}
	if err := authorize(msg.CSR, msg.ChallengePassword); err != nil {
		return nil, fmt.Errorf("authorize SCEP request: %w", err)
	}
	issued, err := m.IssueCertificateFromCSR(msg.CSR.Raw, "scep")
	if err != nil {
		return nil, err
	}
	block, _ := pem.Decode([]byte(issued.CertPEM))
	if block == nil || block.Type != "CERTIFICATE" {
		return nil, fmt.Errorf("issued certificate is not valid PEM")
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("parse issued certificate: %w", err)
	}
	response, err := msg.Success(caCert, caKey, cert)
	if err != nil {
		return nil, fmt.Errorf("create SCEP response: %w", err)
	}
	return response.Raw, nil
}

func MatchSCEPCertificateIdentity(csr *x509.CertificateRequest, nodeName, nodeIP string) bool {
	if csr == nil {
		return false
	}
	if nodeName != "" && strings.EqualFold(csr.Subject.CommonName, nodeName) {
		return true
	}
	if ip := net.ParseIP(nodeIP); ip != nil {
		for _, requested := range csr.IPAddresses {
			if requested.Equal(ip) {
				return true
			}
		}
	}
	for _, requested := range csr.DNSNames {
		if nodeName != "" && strings.EqualFold(requested, nodeName) {
			return true
		}
	}
	return false
}
