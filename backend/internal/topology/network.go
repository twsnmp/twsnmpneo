package topology

import (
	"context"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// GetSNMPAgentForNetwork creates and returns a configured GoSNMP agent for a NetworkEnt.
func GetSNMPAgentForNetwork(n *datastore.NetworkEnt, timeoutSec, retries int) *gosnmp.GoSNMP {
	if n == nil || n.IP == "" {
		return nil
	}
	if strings.HasPrefix(n.SnmpMode, "v3") {
		if n.User == "" {
			return nil
		}
	} else {
		if n.Community == "" {
			n.Community = "public"
		}
	}
	port := uint16(n.SnmpPort)
	if port == 0 {
		port = 161
	}
	if timeoutSec <= 0 {
		timeoutSec = 2
	}
	if retries < 0 {
		retries = 0
	}
	agent := &gosnmp.GoSNMP{
		Target:    n.IP,
		Port:      port,
		Transport: "udp",
		Community: n.Community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Retries:   retries,
		MaxOids:   gosnmp.MaxOids,
	}
	switch n.SnmpMode {
	case "v1":
		agent.Version = gosnmp.Version1
	case "v3auth":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthNoPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 n.User,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: n.Password,
		}
	case "v3authpriv":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 n.User,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: n.Password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        n.Password,
		}
	case "v3authprivex":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 n.User,
			AuthenticationProtocol:   gosnmp.SHA256,
			AuthenticationPassphrase: n.Password,
			PrivacyProtocol:          gosnmp.AES256,
			PrivacyPassphrase:        n.Password,
		}
	case "v3sha256aes128":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 n.User,
			AuthenticationProtocol:   gosnmp.SHA256,
			AuthenticationPassphrase: n.Password,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        n.Password,
		}
	case "v3sha512aes256":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 n.User,
			AuthenticationProtocol:   gosnmp.SHA512,
			AuthenticationPassphrase: n.Password,
			PrivacyProtocol:          gosnmp.AES256,
			PrivacyPassphrase:        n.Password,
		}
	}
	return agent
}

// FetchNetworkPorts queries the network device via SNMP to discover ports (LLDP first, fallback to IF-MIB).
func FetchNetworkPorts(ctx context.Context, nw *datastore.NetworkEnt, timeoutSec, retries int) ([]datastore.PortEnt, error) {
	if nw == nil {
		return nil, fmt.Errorf("network is nil")
	}
	agent := GetSNMPAgentForNetwork(nw, timeoutSec, retries)
	if agent == nil {
		nw.Error = "Invalid SNMP config"
		return nil, fmt.Errorf("invalid SNMP config")
	}

	if err := agent.Connect(); err != nil {
		nw.Error = fmt.Sprintf("SNMP access err: %v", err)
		return nil, err
	}
	defer func() {
		if agent.Conn != nil {
			_ = agent.Conn.Close()
		}
	}()

	setName := nw.Name == ""
	setDescr := nw.Descr == ""
	hports := nw.HPorts
	if hports <= 0 {
		hports = 24
	}

	// 1. Try LLDP-MIB (lldpLocalSystemData)
	portMap := make(map[string]string)
	var lldpPorts []datastore.PortEnt
	x := 0
	y := 0

	err := agent.Walk(".1.0.8802.1.1.2.1", func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.SplitN(name, ".", 2)
		if len(a) != 2 {
			return nil
		}
		switch a[0] {
		case "lldpLocChassisId":
			nw.SystemID = mib.GetMIBValueString(a[0], &variable, false)
		case "lldpLocSysName":
			if setName {
				nw.Name = mib.GetMIBValueString(a[0], &variable, false)
			}
		case "lldpLocSysDesc":
			if setDescr {
				nw.Descr = mib.GetMIBValueString(a[0], &variable, false)
			}
		case "lldpLocSysCapEnabled":
			if setDescr {
				nw.Descr += " " + mib.GetMIBValueString(a[0], &variable, false)
			}
		case "lldpLocPortId":
			portMap[a[1]] = mib.GetMIBValueString(a[0], &variable, false)
		case "lldpLocPortDesc":
			id, ok := portMap[a[1]]
			if !ok {
				id = a[1]
			}
			pName := mib.GetMIBValueString(a[0], &variable, false)
			if pName == "" {
				pName = id
			}
			lldpPorts = append(lldpPorts, datastore.PortEnt{
				Name:    pName,
				ID:      id,
				Index:   a[1],
				X:       x,
				Y:       y,
				Polling: fmt.Sprintf("ifOperStatus.%s", a[1]),
				State:   "unknown",
			})
			x++
			if x >= hports {
				y++
				x = 0
			}
		}
		return nil
	})

	if err == nil && len(lldpPorts) > 0 {
		nw.LLDP = true
		nw.Error = ""
		return lldpPorts, nil
	}

	// 2. If LLDP not available or yielded no ports, query sysName & sysDescr
	if setName || setDescr {
		if r, err := agent.Get([]string{
			".1.3.6.1.2.1.1.5.0", // sysName.0
			".1.3.6.1.2.1.1.1.0", // sysDescr.0
		}); err == nil {
			for _, variable := range r.Variables {
				switch variable.Name {
				case ".1.3.6.1.2.1.1.5.0":
					if setName && nw.Name == "" {
						nw.Name = mibValToString(variable.Value)
					}
				case ".1.3.6.1.2.1.1.1.0":
					if setDescr && nw.Descr == "" {
						nw.Descr = mibValToString(variable.Value)
					}
				}
			}
		}
	}

	// 3. IF-MIB: Walk ifType (.1.3.6.1.2.1.2.2.1.3)
	type ifEntry struct {
		index  string
		numIdx int
		ifType int64
	}
	var ifEntries []ifEntry
	err = agent.Walk(".1.3.6.1.2.1.2.2.1.3", func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.SplitN(name, ".", 2)
		idxStr := ""
		if len(a) == 2 {
			idxStr = a[1]
		} else {
			parts := strings.Split(variable.Name, ".")
			idxStr = parts[len(parts)-1]
		}
		tVal := gosnmp.ToBigInt(variable.Value).Int64()
		num, _ := strconv.Atoi(idxStr)
		ifEntries = append(ifEntries, ifEntry{
			index:  idxStr,
			numIdx: num,
			ifType: tVal,
		})
		return nil
	})
	if err != nil {
		nw.Error = fmt.Sprintf("SNMP walk ifType err: %v", err)
		return nil, err
	}

	var candidateIndices []ifEntry
	for _, e := range ifEntries {
		// Ethernet ONLY (ethernetCsmacd = 6)
		if e.ifType == 6 {
			candidateIndices = append(candidateIndices, e)
		}
	}
	// Fallback: if no ethernetCsmacd interfaces found, accept all non-loopback interfaces
	if len(candidateIndices) == 0 {
		for _, e := range ifEntries {
			if e.ifType != 24 { // 24 = softwareLoopback
				candidateIndices = append(candidateIndices, e)
			}
		}
	}

	// Walk ifName (.1.3.6.1.2.1.31.1.1.1.1)
	nameMap := make(map[string]string)
	_ = agent.Walk(".1.3.6.1.2.1.31.1.1.1.1", func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		idx := parts[len(parts)-1]
		nameMap[idx] = mibValToString(pdu.Value)
		return nil
	})

	// Walk ifDescr (.1.3.6.1.2.1.2.2.1.2) for any missing names
	_ = agent.Walk(".1.3.6.1.2.1.2.2.1.2", func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		idx := parts[len(parts)-1]
		if _, ok := nameMap[idx]; !ok || nameMap[idx] == "" {
			nameMap[idx] = mibValToString(pdu.Value)
		}
		return nil
	})

	// Walk ifOperStatus (.1.3.6.1.2.1.2.2.1.8) for initial state (1=up, 2=down)
	statusMap := make(map[string]string)
	_ = agent.Walk(".1.3.6.1.2.1.2.2.1.8", func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		idx := parts[len(parts)-1]
		val := gosnmp.ToBigInt(pdu.Value).Int64()
		if val == 1 {
			statusMap[idx] = "up"
		} else if val == 2 {
			statusMap[idx] = "down"
		} else {
			statusMap[idx] = "unknown"
		}
		return nil
	})

	// Sort ports by interface numeric index
	sort.Slice(candidateIndices, func(i, j int) bool {
		if candidateIndices[i].numIdx != candidateIndices[j].numIdx {
			return candidateIndices[i].numIdx < candidateIndices[j].numIdx
		}
		return candidateIndices[i].index < candidateIndices[j].index
	})

	var ports []datastore.PortEnt
	for i, ent := range candidateIndices {
		pn := nameMap[ent.index]
		if pn == "" {
			pn = "#" + ent.index
		}
		st := statusMap[ent.index]
		if st == "" {
			st = "unknown"
		}
		ports = append(ports, datastore.PortEnt{
			Name:    pn,
			X:       i % hports,
			Y:       i / hports,
			ID:      ent.index,
			Index:   ent.index,
			Polling: fmt.Sprintf("ifOperStatus.%s", ent.index),
			State:   st,
		})
	}

	nw.Error = ""
	return ports, nil
}

// UpdateNetworkPorts discovers and saves ports for a NetworkEnt.
func UpdateNetworkPorts(ctx context.Context, store datastore.DataStore, nw *datastore.NetworkEnt) error {
	ports, err := FetchNetworkPorts(ctx, nw, 2, 0)
	if err != nil {
		_ = store.SaveNetwork(ctx, nw)
		return err
	}
	if len(ports) > 0 {
		nw.Ports = ports
		nw.Error = ""
		return store.SaveNetwork(ctx, nw)
	}
	return nil
}

// StartNetworkBackend runs a background daemon checking for unpopulated networks and polling port states.
func StartNetworkBackend(ctx context.Context, store datastore.DataStore) {
	go func() {
		ticker := time.NewTicker(15 * time.Second)
		defer ticker.Stop()

		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				nets, err := store.ListNetworks(ctx)
				if err != nil {
					continue
				}
				for _, nw := range nets {
					if nw.Unmanaged || nw.IP == "" {
						continue
					}
					// If ports are missing and no permanent error, try discovering ports
					if len(nw.Ports) == 0 && nw.Error == "" {
						slog.Info("Discovering network ports in background", "ip", nw.IP, "name", nw.Name)
						ports, err := FetchNetworkPorts(ctx, nw, 2, 0)
						if err == nil && len(ports) > 0 {
							nw.Ports = ports
							nw.Error = ""
							_ = store.SaveNetwork(ctx, nw)
						}
					}
				}
			}
		}
	}()
}

func mibValToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}
