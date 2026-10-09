package pki

import (
	"archive/zip"
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/asn1"
	"encoding/pem"
	"fmt"
	"log/slog"
	"math/big"
	"net"
	"strings"
	"sync"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

// Manager manages the Private PKI Certificate Authority and protocol servers.
type Manager struct {
	mu     sync.RWMutex
	store  datastore.DataStore
	conf   datastore.PKIConfEnt

	rootCAPrivateKey   any
	rootCAPublicKey    any
	rootCACertificate  []byte
	scepCAPrivateKey   any
	scepCAPublicKey    any
	scepCACertificate  []byte
	crl                []byte

	httpServer         *echo.Echo
	httpServerRunning  bool
	lastHTTPServerErr  error

	acmeServer         *echo.Echo
	acmeServerRunning  bool
	lastAcmeServerErr  error

	cancel             context.CancelFunc
}

// New creates and initializes a PKI Manager instance.
func New(store datastore.DataStore) (*Manager, error) {
	m := &Manager{
		store: store,
	}
	ctx := context.Background()
	if store != nil {
		conf, err := store.GetPKIConf(ctx)
		if err == nil && conf != nil {
			m.conf = *conf
		} else {
			m.conf = datastore.DefaultPKIConf()
		}
	} else {
		m.conf = datastore.DefaultPKIConf()
	}

	if err := m.loadKeyAndCert(); err != nil {
		slog.Warn("PKI load root CA key/cert", "error", err)
	}
	if err := m.loadScepCA(); err != nil {
		slog.Warn("PKI load SCEP CA key/cert", "error", err)
	}
	if m.IsCAValid() {
		if err := m.createCRL(); err != nil {
			slog.Warn("PKI create CRL", "error", err)
		}
	}
	return m, nil
}

// Start launches the PKI background routines and listeners.
func (m *Manager) Start(ctx context.Context, wg *sync.WaitGroup) error {
	m.mu.Lock()
	pkiCtx, cancel := context.WithCancel(ctx)
	m.cancel = cancel
	enableHTTP := m.conf.EnableHTTP
	enableAcme := m.conf.EnableAcme
	m.mu.Unlock()

	if m.store != nil {
		_ = m.store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "pki",
			Level: "info",
			Event: i18n.Trans("Start PKI service"),
		})
	}

	if wg != nil {
		wg.Add(1)
	}
	go m.caServer(pkiCtx, wg)

	if enableHTTP {
		m.StartHTTPServer()
	}
	if enableAcme {
		m.StartAcmeServer()
	}
	return nil
}

// Stop stops the PKI service and listeners.
func (m *Manager) Stop() {
	m.mu.Lock()
	if m.cancel != nil {
		m.cancel()
	}
	m.mu.Unlock()

	m.StopHTTPServer()
	m.StopAcmeServer()
}

func (m *Manager) caServer(ctx context.Context, wg *sync.WaitGroup) {
	if wg != nil {
		defer wg.Done()
	}
	timer := time.NewTicker(time.Hour * 1)
	defer timer.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			m.mu.Lock()
			if m.IsCAValid() {
				_ = m.createCRL()
			}
			m.mu.Unlock()
		}
	}
}

// IsCAValid checks if the Root CA certificate and key are validly loaded.
func (m *Manager) IsCAValid() bool {
	return m.rootCACertificate != nil && m.rootCAPrivateKey != nil
}

// CreateCA creates and initializes a Root CA and SCEP CA.
func (m *Manager) CreateCA(req *datastore.CreateCAReq) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	datastore.InitCAConf(&m.conf, req)
	if err := m.createRootCACertificate(); err != nil {
		return fmt.Errorf("create root CA certificate: %w", err)
	}
	if err := m.createScepCACertificate(); err != nil {
		slog.Warn("Failed to create SCEP CA certificate", "error", err)
	}
	if err := m.createCRL(); err != nil {
		slog.Warn("Failed to create CRL", "error", err)
	}
	if m.store != nil {
		_ = m.store.SavePKIConf(context.Background(), &m.conf)
	}
	return nil
}

// DestroyCA destroys the CA and deletes all issued certificates.
func (m *Manager) DestroyCA() error {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.stopHTTPServerLocked()
	m.stopAcmeServerLocked()

	m.rootCAPrivateKey = nil
	m.rootCAPublicKey = nil
	m.rootCACertificate = nil
	m.scepCAPrivateKey = nil
	m.scepCAPublicKey = nil
	m.scepCACertificate = nil
	m.crl = nil

	m.conf = datastore.DefaultPKIConf()
	if m.store != nil {
		_ = m.store.SavePKIConf(context.Background(), &m.conf)
		_ = m.store.DeleteAllPKICerts(context.Background())
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "ca",
			Level: "warn",
			Event: i18n.Trans("CA destroyed and issued certificates deleted"),
		})
	}
	return nil
}

func (m *Manager) getSerial() int64 {
	sn := m.conf.Serial
	m.conf.Serial++
	if m.store != nil {
		_ = m.store.SavePKIConf(context.Background(), &m.conf)
	}
	return sn
}

func (m *Manager) createRootCACertificate() error {
	var keyBytes []byte
	var isRSA bool
	var err error

	if strings.HasPrefix(m.conf.RootCAKeyType, "rsa") {
		isRSA = true
		bits := 4096
		switch m.conf.RootCAKeyType {
		case "rsa-2048":
			bits = 2048
		case "rsa-8192":
			bits = 8192
		}
		key, kErr := rsa.GenerateKey(rand.Reader, bits)
		if kErr != nil {
			return kErr
		}
		m.rootCAPrivateKey = key
		m.rootCAPublicKey = &key.PublicKey
		keyBytes = x509.MarshalPKCS1PrivateKey(key)
	} else {
		var curve = elliptic.P256()
		switch m.conf.RootCAKeyType {
		case "ecdsa-224":
			curve = elliptic.P224()
		case "ecdsa-384":
			curve = elliptic.P384()
		case "ecdsa-521":
			curve = elliptic.P521()
		}
		key, kErr := ecdsa.GenerateKey(curve, rand.Reader)
		if kErr != nil {
			return kErr
		}
		m.rootCAPrivateKey = key
		m.rootCAPublicKey = &key.PublicKey
		keyBytes, err = x509.MarshalECPrivateKey(key)
		if err != nil {
			return err
		}
	}

	subject := pkix.Name{
		CommonName: m.conf.Name + " Root CA",
	}
	sn := m.getSerial()
	tmp := &x509.Certificate{
		SerialNumber:          big.NewInt(sn),
		Subject:               subject,
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().AddDate(m.conf.RootCATerm, 0, 0).UTC(),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            int(2),
	}

	certBytes, err := x509.CreateCertificate(rand.Reader, tmp, tmp, m.rootCAPublicKey, m.rootCAPrivateKey)
	if err != nil {
		return err
	}
	m.rootCACertificate = certBytes

	m.conf.RootCACert = string(makePEM(m.rootCACertificate, "CERTIFICATE"))
	if isRSA {
		m.conf.RootCAKey = string(makePEM(keyBytes, "RSA PRIVATE KEY"))
	} else {
		m.conf.RootCAKey = string(makePEM(keyBytes, "EC PRIVATE KEY"))
	}

	if m.store != nil {
		_ = m.store.SavePKICert(context.Background(), &datastore.PKICertEnt{
			ID:          fmt.Sprintf("%x", sn),
			Subject:     subject.String(),
			Created:     time.Now().UnixNano(),
			Certificate: m.conf.RootCACert,
			Expire:      tmp.NotAfter.UnixNano(),
			Type:        "system",
		})
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "ca",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Issued CA certificate subject=%s serial=%x"), subject.String(), sn),
		})
	}
	return nil
}

func (m *Manager) createScepCACertificate() error {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		return err
	}
	m.scepCAPrivateKey = key
	m.scepCAPublicKey = &key.PublicKey

	ca, err := x509.ParseCertificate(m.rootCACertificate)
	if err != nil {
		return err
	}
	sn := m.getSerial()
	tmp := &x509.Certificate{
		SerialNumber: big.NewInt(sn),
		Subject: pkix.Name{
			CommonName: m.conf.Name + " SCEP CA",
		},
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().AddDate(m.conf.RootCATerm, 0, 0).UTC(),
		KeyUsage:              x509.KeyUsageDigitalSignature | x509.KeyUsageCertSign | x509.KeyUsageCRLSign,
		IsCA:                  true,
		BasicConstraintsValid: true,
		MaxPathLen:            int(1),
	}
	for _, san := range strings.Split(m.conf.SANs, ",") {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		baseURL := fmt.Sprintf("http://%s:%d", san, m.conf.HTTPPort)
		if ip := net.ParseIP(san); ip == nil {
			tmp.DNSNames = append(tmp.DNSNames, san)
		} else {
			tmp.IPAddresses = append(tmp.IPAddresses, ip)
		}
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"/crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"/ocsp")
	}
	if strings.HasPrefix(m.conf.HTTPBaseURL, "http://") {
		baseURL := strings.TrimRight(m.conf.HTTPBaseURL, "/")
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"/crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"/ocsp")
	}
	scepCertBytes, err := x509.CreateCertificate(rand.Reader, tmp, ca, m.scepCAPublicKey, m.rootCAPrivateKey)
	if err != nil {
		return err
	}
	m.scepCACertificate = scepCertBytes
	m.conf.ScepCACert = string(makePEM(scepCertBytes, "CERTIFICATE"))
	m.conf.ScepCAKey = string(makePEM(x509.MarshalPKCS1PrivateKey(key), "RSA PRIVATE KEY"))

	if m.store != nil {
		_ = m.store.SavePKICert(context.Background(), &datastore.PKICertEnt{
			ID:          fmt.Sprintf("%x", sn),
			Subject:     tmp.Subject.String(),
			Created:     time.Now().UnixNano(),
			Expire:      tmp.NotAfter.UnixNano(),
			Certificate: m.conf.ScepCACert,
			Type:        "system",
		})
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "ca",
			Level: "info",
			Event: fmt.Sprintf(i18n.Trans("Issued SCEP CA certificate subject=%s serial=%x"), tmp.Subject.String(), sn),
		})
	}
	return nil
}

func (m *Manager) loadKeyAndCert() error {
	if m.conf.RootCAKey == "" {
		return nil
	}
	key, pub, err := getPrivateKeyFromPEM(m.conf.RootCAKey)
	if err != nil {
		return err
	}
	m.rootCAPrivateKey = key
	m.rootCAPublicKey = pub
	cert, err := getCertFromPEM(m.conf.RootCACert)
	if err != nil {
		return err
	}
	m.rootCACertificate = cert
	return nil
}

func (m *Manager) loadScepCA() error {
	if m.conf.ScepCAKey == "" {
		return nil
	}
	key, pub, err := getPrivateKeyFromPEM(m.conf.ScepCAKey)
	if err != nil {
		return err
	}
	m.scepCAPrivateKey = key
	m.scepCAPublicKey = pub
	cert, err := getCertFromPEM(m.conf.ScepCACert)
	if err != nil {
		return err
	}
	m.scepCACertificate = cert
	return nil
}

// CreateCertificateRequest creates a CSR and private key and returns a ZIP archive containing csr.pem and key.pem.
func (m *Manager) CreateCertificateRequest(req *datastore.CSRReqEnt) ([]byte, error) {
	var key any
	var keyBytes []byte
	var err error
	var pemKeyType = "RSA PRIVATE KEY"

	tmp := &x509.CertificateRequest{
		Subject: pkix.Name{
			CommonName:         req.CommonName,
			OrganizationalUnit: []string{req.OrganizationalUnit},
			Organization:       []string{req.Organization},
			Locality:           []string{req.Locality},
			Province:           []string{req.Province},
			Country:            []string{req.Country},
		},
	}
	for _, san := range strings.Split(req.Sans, ",") {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		if ip := net.ParseIP(san); ip == nil {
			tmp.DNSNames = append(tmp.DNSNames, san)
		} else {
			tmp.IPAddresses = append(tmp.IPAddresses, ip)
		}
	}

	if strings.HasPrefix(req.KeyType, "rsa") {
		bits := 4096
		switch req.KeyType {
		case "rsa-2048":
			bits = 2048
		case "rsa-8192":
			bits = 8192
		}
		k, kErr := rsa.GenerateKey(rand.Reader, bits)
		if kErr != nil {
			return nil, kErr
		}
		tmp.PublicKeyAlgorithm = x509.RSA
		tmp.SignatureAlgorithm = x509.SHA256WithRSA
		tmp.PublicKey = &k.PublicKey
		key = k
		keyBytes = x509.MarshalPKCS1PrivateKey(k)
	} else {
		var curve = elliptic.P256()
		switch req.KeyType {
		case "ecdsa-224":
			curve = elliptic.P224()
		case "ecdsa-384":
			curve = elliptic.P384()
		case "ecdsa-521":
			curve = elliptic.P521()
		}
		k, kErr := ecdsa.GenerateKey(curve, rand.Reader)
		if kErr != nil {
			return nil, kErr
		}
		tmp.PublicKeyAlgorithm = x509.ECDSA
		tmp.SignatureAlgorithm = x509.ECDSAWithSHA256
		tmp.PublicKey = &k.PublicKey
		key = k
		pemKeyType = "EC PRIVATE KEY"
		keyBytes, err = x509.MarshalECPrivateKey(k)
		if err != nil {
			return nil, err
		}
	}

	csr, err := x509.CreateCertificateRequest(rand.Reader, tmp, key)
	if err != nil {
		return nil, err
	}

	buf := new(bytes.Buffer)
	w := zip.NewWriter(buf)

	f, err := w.Create("csr.pem")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(makePEM(csr, "CERTIFICATE REQUEST")); err != nil {
		return nil, err
	}

	f, err = w.Create("key.pem")
	if err != nil {
		return nil, err
	}
	if _, err := f.Write(makePEM(keyBytes, pemKeyType)); err != nil {
		return nil, err
	}
	_ = w.Close()
	return buf.Bytes(), nil
}

// CreateCertificate manually issues a certificate from uploaded CSR PEM bytes.
func (m *Manager) CreateCertificate(csrBytes []byte) ([]byte, error) {
	block, _ := pem.Decode(csrBytes)
	if block == nil || block.Type != "CERTIFICATE REQUEST" {
		return nil, fmt.Errorf("certificate request PEM not found")
	}
	m.mu.Lock()
	defer m.mu.Unlock()

	crt, _, err := m.createCertificateFromCSR(block.Bytes, "manual", make(map[string]string))
	if err != nil {
		return nil, err
	}
	return makePEM(crt, "CERTIFICATE"), nil
}

func (m *Manager) createCertificateFromCSR(csrBytes []byte, certType string, info map[string]string) ([]byte, string, error) {
	csr, err := x509.ParseCertificateRequest(csrBytes)
	if err != nil {
		return nil, "", err
	}
	nodeID, err := m.checkCSR(certType, csr, info)
	if err != nil {
		if m.store != nil {
			_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
				Time:  time.Now().UnixNano(),
				Type:  "ca",
				Level: "low",
				Event: fmt.Sprintf(i18n.Trans("Rejected certificate issuance subject=%s err=%v"), csr.Subject.String(), err),
			})
		}
		return nil, "", err
	}
	ca, err := x509.ParseCertificate(m.rootCACertificate)
	if err != nil {
		return nil, "", err
	}
	term := m.conf.CertTerm
	if term < 1 {
		term = 24 * 30
	}
	sn := m.getSerial()
	tmp := &x509.Certificate{
		SerialNumber:          big.NewInt(sn),
		Subject:               csr.Subject,
		NotBefore:             time.Now().UTC(),
		NotAfter:              time.Now().Add(time.Hour * time.Duration(term)).UTC(),
		KeyUsage:              x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth, x509.ExtKeyUsageServerAuth},
		DNSNames:              csr.DNSNames,
		EmailAddresses:        csr.EmailAddresses,
		IPAddresses:           csr.IPAddresses,
		ExtraExtensions:       csr.Extensions,
		IsCA:                  false,
		MaxPathLen:            0,
		BasicConstraintsValid: true,
		CRLDistributionPoints: []string{},
		OCSPServer:            []string{},
	}
	for _, san := range strings.Split(m.conf.SANs, ",") {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		baseURL := fmt.Sprintf("http://%s:%d/", san, m.conf.HTTPPort)
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"ocsp")
	}
	if strings.HasPrefix(m.conf.HTTPBaseURL, "http://") {
		baseURL := strings.TrimRight(m.conf.HTTPBaseURL, "/")
		tmp.CRLDistributionPoints = append(tmp.CRLDistributionPoints, baseURL+"/crl")
		tmp.OCSPServer = append(tmp.OCSPServer, baseURL+"/ocsp")
	}

	ret, err := x509.CreateCertificate(rand.Reader, tmp, ca, csr.PublicKey, m.rootCAPrivateKey)
	if err != nil {
		return nil, "", err
	}
	certID := fmt.Sprintf("%x", sn)
	if m.store != nil {
		_ = m.store.SavePKICert(context.Background(), &datastore.PKICertEnt{
			ID:          certID,
			Subject:     tmp.Subject.String(),
			Created:     time.Now().UnixNano(),
			Expire:      tmp.NotAfter.UnixNano(),
			NodeID:      nodeID,
			Info:        info,
			Certificate: string(makePEM(ret, "CERTIFICATE")),
			Type:        certType,
		})
		nodeName := ""
		if node, _ := m.store.GetNode(context.Background(), nodeID); node != nil {
			nodeName = node.Name
		}
		_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Time:     time.Now().UnixNano(),
			Type:     "ca",
			Level:    "info",
			NodeID:   nodeID,
			NodeName: nodeName,
			Event:    fmt.Sprintf(i18n.Trans("Issued certificate subject=%s serial=%s"), tmp.Subject.String(), certID),
		})
	}
	return ret, certID, nil
}

func (m *Manager) checkCSR(certType string, csr *x509.CertificateRequest, info map[string]string) (string, error) {
	if m.store == nil {
		return "", nil
	}
	var node *datastore.NodeEnt
	for _, n := range csr.DNSNames {
		m.store.ForEachNodes(func(cand *datastore.NodeEnt) bool {
			if strings.EqualFold(cand.Name, n) {
				node = cand
				return false
			}
			return true
		})
		if node != nil {
			break
		}
	}
	if node == nil {
		for _, ip := range csr.IPAddresses {
			if ipv4 := ip.To4(); ipv4 != nil {
				m.store.ForEachNodes(func(cand *datastore.NodeEnt) bool {
					if cand.IP == ipv4.String() {
						node = cand
						return false
					}
					return true
				})
				if node != nil {
					break
				}
			}
		}
		if node == nil {
			m.store.ForEachNodes(func(cand *datastore.NodeEnt) bool {
				if strings.EqualFold(cand.Name, csr.Subject.CommonName) {
					node = cand
					return false
				}
				return true
			})
			if node == nil {
				if remoteIP, ok := info["RemoteAddr"]; ok && remoteIP != "" {
					m.store.ForEachNodes(func(cand *datastore.NodeEnt) bool {
						if cand.IP == remoteIP {
							node = cand
							return false
						}
						return true
					})
				}
			}
		}
	}
	if certType == "scep" {
		if node == nil {
			return "", fmt.Errorf("node not found")
		}
		if p, ok := info["ChallengePassword"]; !ok || p == "" || node.Password != p {
			return "", fmt.Errorf("challenge password mismatch")
		}
	}
	if node != nil {
		return node.ID, nil
	}
	return "", nil
}

type issuingDistributionPoint struct {
	DistributionPoint          distributionPointName `asn1:"optional,tag:0"`
	OnlyContainsUserCerts      bool                  `asn1:"optional,tag:1"`
	OnlyContainsCACerts        bool                  `asn1:"optional,tag:2"`
	OnlySomeReasons            asn1.BitString        `asn1:"optional,tag:3"`
	IndirectCRL                bool                  `asn1:"optional,tag:4"`
	OnlyContainsAttributeCerts bool                  `asn1:"optional,tag:5"`
}

type distributionPointName struct {
	FullName     []asn1.RawValue  `asn1:"optional,tag:0"`
	RelativeName pkix.RDNSequence `asn1:"optional,tag:1"`
}

func (m *Manager) createCRL() error {
	dp := distributionPointName{
		FullName: []asn1.RawValue{},
	}
	for _, san := range strings.Split(m.conf.SANs, ",") {
		san = strings.TrimSpace(san)
		if san == "" {
			continue
		}
		cdp := fmt.Sprintf("http://%s:%d/crl", san, m.conf.HTTPPort)
		dp.FullName = append(dp.FullName, asn1.RawValue{Tag: 6, Class: 2, Bytes: []byte(cdp)})
	}
	var oidExtensionIssuingDistributionPoint = []int{2, 5, 29, 28}
	idp := issuingDistributionPoint{
		DistributionPoint: dp,
	}
	v, err := asn1.Marshal(idp)
	if err != nil {
		return err
	}

	cdpExt := pkix.Extension{
		Id:       oidExtensionIssuingDistributionPoint,
		Critical: true,
		Value:    v,
	}

	key, ok := m.rootCAPrivateKey.(crypto.Signer)
	if !ok {
		return fmt.Errorf("invalid key type")
	}
	ca, err := x509.ParseCertificate(m.rootCACertificate)
	if err != nil {
		return err
	}
	revokedCerts := []pkix.RevokedCertificate{}
	now := time.Now().UnixNano()
	if m.store != nil {
		m.store.ForEachPKICert(func(c *datastore.PKICertEnt) bool {
			if c.Revoked > 0 && c.Expire > now {
				if s, ok := big.NewInt(0).SetString(c.ID, 16); ok {
					revokedCerts = append(revokedCerts, pkix.RevokedCertificate{
						RevocationTime: time.Unix(0, c.Revoked),
						SerialNumber:   s,
					})
				}
			}
			return true
		})
	}
	tmp := &x509.RevocationList{
		SignatureAlgorithm:  x509.ECDSAWithSHA256,
		RevokedCertificates: revokedCerts,
		Number:              big.NewInt(m.conf.CrlNumber),
		ThisUpdate:          time.Now(),
		NextUpdate:          time.Now().Add(time.Duration(m.conf.CrlInterval) * time.Hour),
		ExtraExtensions:     []pkix.Extension{cdpExt},
	}
	m.crl, err = x509.CreateRevocationList(rand.Reader, tmp, ca, key)
	if err == nil {
		m.conf.CrlNumber++
		if m.store != nil {
			_ = m.store.SavePKIConf(context.Background(), &m.conf)
		}
	}
	return err
}

// RevokeCert revokes a certificate by its ID and refreshes CRL.
func (m *Manager) RevokeCert(id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.store == nil {
		return nil
	}
	cert, err := m.store.GetPKICert(context.Background(), id)
	if err != nil || cert == nil {
		return fmt.Errorf("cert not found")
	}
	cert.Revoked = time.Now().UnixNano()
	_ = m.store.SavePKICert(context.Background(), cert)
	_ = m.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
		Time:  cert.Revoked,
		Level: "low",
		Type:  "ca",
		Event: fmt.Sprintf(i18n.Trans("Revoked certificate subject=%s serial=%s"), cert.Subject, cert.ID),
	})
	_ = m.createCRL()
	return nil
}

// GetPKIControl returns current server control information.
func (m *Manager) GetPKIControl() *datastore.PKIControlEnt {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return &datastore.PKIControlEnt{
		EnableAcme:  m.conf.EnableAcme,
		EnableHTTP:  m.conf.EnableHTTP,
		AcmeBaseURL: m.conf.AcmeBaseURL,
		CertTerm:    m.conf.CertTerm,
		CrlInterval: m.conf.CrlInterval,
		AcmeStatus:  m.GetAcmeServerStatus(),
		HTTPStatus:  m.GetHTTPServerStatus(),
	}
}

// UpdatePKIControl updates PKI control settings and reflects listener states.
func (m *Manager) UpdatePKIControl(req *datastore.PKIControlEnt) error {
	m.mu.Lock()
	m.conf.EnableAcme = req.EnableAcme
	m.conf.EnableHTTP = req.EnableHTTP
	m.conf.AcmeBaseURL = req.AcmeBaseURL
	m.conf.CertTerm = req.CertTerm
	m.conf.CrlInterval = req.CrlInterval
	if m.store != nil {
		_ = m.store.SavePKIConf(context.Background(), &m.conf)
	}
	enableHTTP := m.conf.EnableHTTP
	enableAcme := m.conf.EnableAcme
	m.mu.Unlock()

	if enableHTTP {
		m.StartHTTPServer()
	} else {
		m.StopHTTPServer()
	}

	if enableAcme {
		m.StartAcmeServer()
	} else {
		m.StopAcmeServer()
	}
	return nil
}

// GetCACertPEM returns the Root CA certificate PEM string.
func (m *Manager) GetCACertPEM() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conf.RootCACert
}

// GetSCEPCACertPEM returns the SCEP CA certificate PEM string.
func (m *Manager) GetSCEPCACertPEM() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.conf.ScepCACert
}

// GetCRL returns the binary CRL bytes.
func (m *Manager) GetCRL() []byte {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.crl
}

// Helper functions for PEM encoding/decoding

func makePEM(b []byte, t string) []byte {
	return pem.EncodeToMemory(
		&pem.Block{
			Type:  t,
			Bytes: b,
		},
	)
}

func getCertFromPEM(certPEM string) ([]byte, error) {
	block, _ := pem.Decode([]byte(certPEM))
	if block == nil {
		return nil, fmt.Errorf("invalid PEM certificate")
	}
	return block.Bytes, nil
}

func getPrivateKeyFromPEM(keyPEM string) (any, any, error) {
	block, _ := pem.Decode([]byte(keyPEM))
	if block == nil {
		return nil, nil, fmt.Errorf("invalid PEM private key")
	}
	switch block.Type {
	case "RSA PRIVATE KEY":
		key, err := x509.ParsePKCS1PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		return key, &key.PublicKey, nil
	case "EC PRIVATE KEY":
		key, err := x509.ParseECPrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		return key, &key.PublicKey, nil
	default:
		key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
		if err != nil {
			return nil, nil, err
		}
		switch k := key.(type) {
		case *rsa.PrivateKey:
			return k, &k.PublicKey, nil
		case *ecdsa.PrivateKey:
			return k, &k.PublicKey, nil
		default:
			return nil, nil, fmt.Errorf("unsupported key type")
		}
	}
}
