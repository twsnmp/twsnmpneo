package diagnose

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/http"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
)

type PingDiagnoseResult struct {
	Success  bool    `json:"success"`
	Sent     int     `json:"sent"`
	Received int     `json:"received"`
	Loss     float64 `json:"loss"`
	AvgRTT   float64 `json:"avgRtt"` // ms
	Error    string  `json:"error,omitempty"`
}

type SNMPDiagnoseResult struct {
	Configured bool    `json:"configured"`
	Success    bool    `json:"success"`
	SysDescr   string  `json:"sysDescr,omitempty"`
	SysUpTime  string  `json:"sysUpTime,omitempty"`
	RTT        float64 `json:"rtt"` // ms
	Error      string  `json:"error,omitempty"`
}

type WebPortDiagnose struct {
	Port          int     `json:"port"`
	Success       bool    `json:"success"`
	StatusCode    int     `json:"statusCode"`
	RTT           float64 `json:"rtt"` // ms
	CertIssuer    string  `json:"certIssuer,omitempty"`
	RemainingDays int     `json:"remainingDays,omitempty"`
	Error         string  `json:"error,omitempty"`
}

type WebDiagnoseResult struct {
	HTTP  *WebPortDiagnose `json:"http,omitempty"`
	HTTPS *WebPortDiagnose `json:"https,omitempty"`
}

type NodeDiagnoseResult struct {
	NodeID   string             `json:"nodeId"`
	NodeName string             `json:"nodeName"`
	IP       string             `json:"ip"`
	Status   string             `json:"status"` // healthy, warning, critical
	Ping     PingDiagnoseResult `json:"ping"`
	SNMP     SNMPDiagnoseResult `json:"snmp"`
	Web      WebDiagnoseResult  `json:"web"`
	Summary  string             `json:"summary"`
	Time     string             `json:"time"`
}

// DiagnoseNode runs Ping, SNMP, and Web connectivity probes against a managed node.
func DiagnoseNode(ctx context.Context, node *datastore.NodeEnt) *NodeDiagnoseResult {
	res := &NodeDiagnoseResult{
		NodeID:   node.ID,
		NodeName: node.Name,
		IP:       node.IP,
		Status:   "healthy",
		Time:     time.Now().Format(time.RFC3339),
	}

	if node.IP == "" {
		res.Status = "critical"
		res.Summary = "Node IP address is empty"
		return res
	}

	// 1. Ping Probe (3 packets)
	sent := 3
	received := 0
	var totalRTT float64
	for i := 0; i < sent; i++ {
		pr := ping.DoPing(node.IP, 64, 1, 1000, 0)
		if pr.Stat == ping.PingOK {
			received++
			totalRTT += float64(pr.Time) / 1e6 // convert ns to ms
		}
	}
	res.Ping.Sent = sent
	res.Ping.Received = received
	res.Ping.Loss = float64(sent-received) / float64(sent) * 100.0
	if received > 0 {
		res.Ping.Success = true
		res.Ping.AvgRTT = totalRTT / float64(received)
	} else {
		res.Ping.Success = false
		res.Ping.Error = "No ping response received (100% loss)"
	}

	// 2. SNMP Probe
	snmpMode := strings.ToLower(node.SnmpMode)
	if snmpMode != "" && snmpMode != "none" {
		res.SNMP.Configured = true
		port := uint16(node.SnmpPort)
		if port == 0 {
			port = 161
		}
		community := node.Community
		if community == "" && !strings.HasPrefix(snmpMode, "v3") {
			community = "public"
		}
		agent := &gosnmp.GoSNMP{
			Target:    node.IP,
			Port:      port,
			Community: community,
			Version:   gosnmp.Version2c,
			Timeout:   2 * time.Second,
			Retries:   1,
		}
		if snmpMode == "v1" {
			agent.Version = gosnmp.Version1
		}
		startSNMP := time.Now()
		if err := agent.Connect(); err != nil {
			res.SNMP.Error = fmt.Sprintf("connect failed: %v", err)
		} else {
			defer agent.Conn.Close()
			oids := []string{".1.3.6.1.2.1.1.1.0", ".1.3.6.1.2.1.1.3.0"} // sysDescr, sysUpTime
			pdu, err := agent.Get(oids)
			res.SNMP.RTT = float64(time.Since(startSNMP).Microseconds()) / 1000.0
			if err != nil {
				res.SNMP.Error = fmt.Sprintf("get failed: %v", err)
			} else if len(pdu.Variables) > 0 {
				res.SNMP.Success = true
				for _, v := range pdu.Variables {
					if strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.1") {
						switch val := v.Value.(type) {
						case []byte:
							res.SNMP.SysDescr = string(val)
						default:
							res.SNMP.SysDescr = fmt.Sprintf("%v", val)
						}
					} else if strings.HasPrefix(v.Name, ".1.3.6.1.2.1.1.3") {
						res.SNMP.SysUpTime = fmt.Sprintf("%v", v.Value)
					}
				}
			}
		}
	} else {
		res.SNMP.Configured = false
	}

	// 3. Web Probes (Port 80 HTTP, Port 443 HTTPS)
	client := &http.Client{
		Timeout: 2 * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			DialContext: (&net.Dialer{
				Timeout: 2 * time.Second,
			}).DialContext,
		},
	}

	// HTTP Probe
	httpStart := time.Now()
	httpResp, httpErr := client.Get(fmt.Sprintf("http://%s:80", node.IP))
	res.Web.HTTP = &WebPortDiagnose{Port: 80}
	if httpErr == nil {
		res.Web.HTTP.Success = true
		res.Web.HTTP.StatusCode = httpResp.StatusCode
		res.Web.HTTP.RTT = float64(time.Since(httpStart).Microseconds()) / 1000.0
		_ = httpResp.Body.Close()
	} else {
		res.Web.HTTP.Error = httpErr.Error()
	}

	// HTTPS Probe
	httpsStart := time.Now()
	httpsResp, httpsErr := client.Get(fmt.Sprintf("https://%s:443", node.IP))
	res.Web.HTTPS = &WebPortDiagnose{Port: 443}
	if httpsErr == nil {
		res.Web.HTTPS.Success = true
		res.Web.HTTPS.StatusCode = httpsResp.StatusCode
		res.Web.HTTPS.RTT = float64(time.Since(httpsStart).Microseconds()) / 1000.0
		if httpsResp.TLS != nil && len(httpsResp.TLS.PeerCertificates) > 0 {
			cert := httpsResp.TLS.PeerCertificates[0]
			res.Web.HTTPS.CertIssuer = cert.Issuer.CommonName
			rem := int(time.Until(cert.NotAfter).Hours() / 24)
			res.Web.HTTPS.RemainingDays = rem
		}
		_ = httpsResp.Body.Close()
	} else {
		res.Web.HTTPS.Error = httpsErr.Error()
	}

	// Formulate health status & summary
	if !res.Ping.Success && (!res.SNMP.Configured || !res.SNMP.Success) && !res.Web.HTTP.Success && !res.Web.HTTPS.Success {
		res.Status = "critical"
		res.Summary = fmt.Sprintf("Node %s (%s) is unreachable via Ping, SNMP, and Web ports.", node.Name, node.IP)
	} else if !res.Ping.Success || (res.SNMP.Configured && !res.SNMP.Success) {
		res.Status = "warning"
		res.Summary = fmt.Sprintf("Node %s (%s) responded partially (Ping: %v, SNMP: %v).", node.Name, node.IP, res.Ping.Success, res.SNMP.Success)
	} else {
		res.Status = "healthy"
		res.Summary = fmt.Sprintf("Node %s (%s) is healthy with Ping RTT %.1fms.", node.Name, node.IP, res.Ping.AvgRTT)
	}

	return res
}
