package polling

import (
	"context"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// SNMPPoller queries SNMP agents for MIB values.
type SNMPPoller struct{}

func (p *SNMPPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{
			State:   StateHigh,
			Message: "missing node IP address",
		}, nil
	}

	oidParam := pe.Params
	if oidParam == "" {
		oidParam = ".1.3.6.1.2.1.1.1.0" // sysDescr.0
	}
	oid := mib.NameToOID(oidParam)

	port := uint16(161)
	if node.SnmpPort > 0 {
		port = uint16(node.SnmpPort)
	}

	timeout := time.Duration(pe.Timeout) * time.Second
	if timeout <= 0 {
		timeout = 2 * time.Second
	}

	version := gosnmp.Version2c
	if node.SnmpMode == "v1" {
		version = gosnmp.Version1
	}

	community := "public"
	if node.Community != "" {
		community = node.Community
	}

	params := &gosnmp.GoSNMP{
		Target:    node.IP,
		Port:      port,
		Community: community,
		Version:   version,
		Timeout:   timeout,
		Retries:   pe.Retry,
	}

	start := time.Now()
	if err := params.Connect(); err != nil {
		return &Result{
			State:   StateHigh,
			Message: fmt.Sprintf("snmp connect failed: %v", err),
		}, nil
	}
	defer params.Conn.Close()

	resp, err := params.Get([]string{oid})
	rtt := time.Since(start)

	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("snmp get failed: %v", err),
		}, nil
	}

	if len(resp.Variables) == 0 {
		return &Result{
			State:   StateWarn,
			RTT:     rtt,
			Message: "no variables returned",
		}, nil
	}

	name := mib.OIDToName(resp.Variables[0].Name)
	valStr := mib.GetMIBValueString(name, &resp.Variables[0], false)
	if valStr == "" {
		valStr = formatSNMPValue(resp.Variables[0])
	}

	// Optional filter comparison
	if pe.Filter != "" && !strings.Contains(valStr, pe.Filter) {
		return &Result{
			State:   StateWarn,
			RTT:     rtt,
			Message: fmt.Sprintf("snmp value '%s' does not match filter '%s'", valStr, pe.Filter),
			Fields: map[string]interface{}{
				"val": valStr,
				"rtt": rtt.Milliseconds(),
			},
		}, nil
	}

	return &Result{
		State:   StateNormal,
		RTT:     rtt,
		Message: fmt.Sprintf("snmp ok, value=%s", valStr),
		Fields: map[string]interface{}{
			"val": valStr,
			"rtt": rtt.Milliseconds(),
		},
	}, nil
}

func formatSNMPValue(v gosnmp.SnmpPDU) string {
	switch v.Type {
	case gosnmp.OctetString:
		return string(v.Value.([]byte))
	case gosnmp.Integer, gosnmp.Counter32, gosnmp.Gauge32, gosnmp.TimeTicks, gosnmp.Counter64:
		return fmt.Sprintf("%v", v.Value)
	case gosnmp.ObjectIdentifier:
		return fmt.Sprintf("%v", v.Value)
	default:
		return strconv.FormatInt(gosnmp.ToBigInt(v.Value).Int64(), 10)
	}
}
