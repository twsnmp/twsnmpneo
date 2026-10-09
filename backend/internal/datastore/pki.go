package datastore

import (
	"fmt"
	"net"
	"os"
	"strings"
	"time"
)

// CreateCAReq represents a request from the frontend to create a CA.
type CreateCAReq struct {
	RootCAKeyType string `json:"RootCAKeyType"`
	Name          string `json:"Name"`
	SANs          string `json:"SANs"`
	AcmePort      int    `json:"AcmePort"`
	HTTPBaseURL   string `json:"HttpBaseURL"`
	AcmeBaseURL   string `json:"AcmeBaseURL"`
	HTTPPort      int    `json:"HttpPort"`
	RootCATerm    int    `json:"RootCATerm"`
	CrlInterval   int    `json:"CrlInterval"`
	CertTerm      int    `json:"CertTerm"`
}

// CSRReqEnt represents a request to generate a CSR and private key.
type CSRReqEnt struct {
	KeyType            string `json:"KeyType"`
	CommonName         string `json:"CommonName"`
	OrganizationalUnit string `json:"OrganizationalUnit"`
	Organization       string `json:"Organization"`
	Locality           string `json:"Locality"`
	Province           string `json:"Province"`
	Country            string `json:"Country"`
	Sans               string `json:"Sans"`
}

// PKIControlEnt represents the PKI server control parameters and runtime status.
type PKIControlEnt struct {
	AcmeBaseURL string `json:"AcmeBaseURL"`
	EnableAcme  bool   `json:"EnableAcme"`
	EnableHTTP  bool   `json:"EnableHttp"`
	AcmeStatus  string `json:"AcmeStatus"`
	HTTPStatus  string `json:"HttpStatus"`
	CrlInterval int    `json:"CrlInterval"`
	CertTerm    int    `json:"CertTerm"`
}

// PKIConfEnt is the configuration data for a CA stored in the DB.
type PKIConfEnt struct {
	Name           string `json:"Name"`
	SANs           string `json:"AcmeSANs"`
	RootCAKeyType  string `json:"RootCAKeyType"`
	RootCAKey      string `json:"RootCAKey"`
	RootCACert     string `json:"RootCACert"`
	RootCATerm     int    `json:"RootCATerm"`
	CertTerm       int    `json:"CertTerm"`
	Serial         int64  `json:"Serial"`
	AcmeServerKey  string `json:"AcmeServerKey"`
	AcmeServerCert string `json:"AcmeServerCert"`
	AcmeBaseURL    string `json:"AcmeBaseURL"`
	AcmePort       int    `json:"AcmePort"`
	HTTPBaseURL    string `json:"HttpBaseURL"`
	HTTPPort       int    `json:"HttpPort"`
	ScepCAKey      string `json:"ScepCAKey"`
	ScepCACert     string `json:"ScepCACert"`
	CrlNumber      int64  `json:"CrlNumber"`
	CrlInterval    int    `json:"CrlInterval"`
	EnableAcme     bool   `json:"EnableAcme"`
	EnableHTTP     bool   `json:"EnableHttp"`
}

// PKICertEnt is the issued certificate entity stored in the DB.
type PKICertEnt struct {
	ID          string            `json:"ID"`
	Subject     string            `json:"Subject"`
	NodeID      string            `json:"NodeID"`
	Created     int64             `json:"Created"`
	Revoked     int64             `json:"Revoked"`
	Expire      int64             `json:"Expire"`
	Type        string            `json:"Type"`
	Certificate string            `json:"Certificate"`
	Info        map[string]string `json:"Info"`
}

// DefaultPKIConf returns default PKI configuration.
func DefaultPKIConf() PKIConfEnt {
	return PKIConfEnt{
		CrlInterval:   24,
		RootCAKeyType: "ecdsa-256",
		RootCATerm:    10,
		CertTerm:      365 * 24,
		HTTPPort:      8082,
		AcmePort:      8083,
		Serial:        time.Now().UnixNano(),
		CrlNumber:     1,
		SANs:          GetDefaultSANs(),
	}
}

// InitCAConf initializes PKIConfEnt from CreateCAReq.
func InitCAConf(conf *PKIConfEnt, req *CreateCAReq) {
	if conf == nil || req == nil {
		return
	}
	conf.RootCAKeyType = req.RootCAKeyType
	conf.AcmePort = req.AcmePort
	conf.HTTPPort = req.HTTPPort
	conf.HTTPBaseURL = req.HTTPBaseURL
	conf.Name = req.Name
	if req.SANs == "" {
		conf.SANs = GetDefaultSANs()
	} else {
		conf.SANs = req.SANs
	}
	if req.AcmeBaseURL == "" {
		baseURL := "https://"
		if a := strings.Split(conf.SANs, ","); len(a) > 0 && a[0] != "" {
			baseURL += a[0]
		} else {
			if h, err := os.Hostname(); err == nil {
				baseURL += h
			} else {
				baseURL += "localhost"
			}
		}
		baseURL += fmt.Sprintf(":%d", conf.AcmePort)
		conf.AcmeBaseURL = baseURL
	} else {
		conf.AcmeBaseURL = req.AcmeBaseURL
	}
	conf.RootCATerm = req.RootCATerm
	conf.CertTerm = req.CertTerm
	conf.CrlInterval = req.CrlInterval
	if conf.AcmePort < 1 || conf.AcmePort > 0xfffe {
		conf.AcmePort = 8083
	}
	if conf.HTTPPort < 1 || conf.HTTPPort > 0xfffe {
		conf.HTTPPort = 8082
	}
	if conf.CertTerm < 1 {
		conf.CertTerm = 24 * 365
	}
}

// GetDefaultSANs returns local host IP addresses and host name.
func GetDefaultSANs() string {
	sans := []string{}
	if n, err := os.Hostname(); err == nil && n != "" {
		sans = append(sans, n)
	}
	if ifs, err := net.Interfaces(); err == nil {
		for _, i := range ifs {
			if (i.Flags&net.FlagLoopback) == net.FlagLoopback ||
				(i.Flags&net.FlagUp) != net.FlagUp ||
				(i.Flags&net.FlagPointToPoint) == net.FlagPointToPoint ||
				len(i.HardwareAddr) != 6 {
				continue
			}
			addrs, err := i.Addrs()
			if err != nil {
				continue
			}
			for _, a := range addrs {
				cidr := a.String()
				ip, _, err := net.ParseCIDR(cidr)
				if err != nil {
					continue
				}
				ipv4 := ip.To4()
				if ipv4 == nil {
					continue
				}
				sans = append(sans, ipv4.String())
			}
		}
	}
	return strings.Join(sans, ",")
}
