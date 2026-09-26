package topology

import (
	"context"
	"fmt"
	"log/slog"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

// NeighborLineEnt represents a candidate topology line with confidence rating and rationale.
type NeighborLineEnt struct {
	datastore.LineEnt
	Confidence string `json:"Confidence"` // "strict" | "speculative"
	Reason     string `json:"Reason"`     // "LLDP" | "CDP" | "STP" | "FDB-Edge" | "FDB-Heuristic" | "ARP" | "Subnet Heuristic" | "AI"
}

// FindNeighborNetworksAndLinesResp returns detected candidate lines and neighboring networks.
type FindNeighborNetworksAndLinesResp struct {
	Networks []*datastore.NetworkEnt `json:"Networks"`
	Lines    []NeighborLineEnt       `json:"Lines"`
}

// FDBTableEnt represents a Forwarding Database entry
type FDBTableEnt struct {
	MAC     string
	VLanID  int
	Port    int
	IfIndex int
	Node    string
	Vendor  string
}

// FindTopologyForNetwork searches for candidate connections and adjacent networks for a switch.
func FindTopologyForNetwork(ctx context.Context, store datastore.DataStore, nw *datastore.NetworkEnt) (*FindNeighborNetworksAndLinesResp, error) {
	ret := &FindNeighborNetworksAndLinesResp{
		Networks: []*datastore.NetworkEnt{},
		Lines:    []NeighborLineEnt{},
	}

	if nw == nil {
		return ret, fmt.Errorf("network is nil")
	}

	allNodes, _ := store.ListNodes(ctx)
	allNets, _ := store.ListNetworks(ctx)
	allPollings, _ := store.ListPollings(ctx)
	existingLines, _ := store.ListLines(ctx)

	// 1. Try SNMP-based discovery if managed
	if !nw.Unmanaged && nw.IP != "" {
		// Inherit SNMP settings from corresponding node if switch SNMP config is empty
		if nw.Community == "" && !strings.HasPrefix(nw.SnmpMode, "v3") {
			for _, nd := range allNodes {
				if nd.IP == nw.IP && (nd.Community != "" || strings.HasPrefix(nd.SnmpMode, "v3")) {
					nw.Community = nd.Community
					nw.SnmpMode = nd.SnmpMode
					nw.User = nd.User
					nw.Password = nd.Password
					if nw.SnmpPort == 0 && nd.SnmpPort != 0 {
						nw.SnmpPort = nd.SnmpPort
					}
					_ = store.SaveNetwork(ctx, nw)
					break
				}
			}
		}

		agent := GetSNMPAgentForNetwork(nw, 4, 1)
		if agent != nil {
			if err := agent.Connect(); err == nil {
				defer func() {
					if agent.Conn != nil {
						_ = agent.Conn.Close()
					}
				}()

				// Port map by index and ID for local network
				portByIndex := make(map[string]*datastore.PortEnt)
				portByID := make(map[string]*datastore.PortEnt)
				for i := range nw.Ports {
					p := &nw.Ports[i]
					if p.Index != "" {
						portByIndex[p.Index] = p
					}
					if p.ID != "" {
						portByID[p.ID] = p
					}
				}

				// 1. Build bridge port to ifIndex mapping (dot1dBasePortIfIndex)
				portToIFIndexMap := make(map[int]int)
				dot1dBaseOID := mib.NameToOID("dot1dBasePortIfIndex")
				_ = agent.Walk(dot1dBaseOID, func(variable gosnmp.SnmpPDU) error {
					name := mib.OIDToName(variable.Name)
					a := strings.SplitN(name, ".", 2)
					if len(a) == 2 {
						if idx, err := strconv.Atoi(a[1]); err == nil {
							portToIFIndexMap[idx] = int(gosnmp.ToBigInt(variable.Value).Int64())
						}
					}
					return nil
				})

				// 2. LLDP Discovery
				findLLDPTopology(agent, nw, ret, allNodes, allNets, allPollings, existingLines, portByIndex, portByID)

				// 3. CDP (CISCO-CDP-MIB) Discovery
				findCDPTopology(agent, nw, ret, allNodes, allNets, allPollings, existingLines, portByIndex, portByID)

				// 4. STP (BRIDGE-MIB Spanning Tree) Discovery
				findSTPTopology(agent, nw, ret, allNets, existingLines, portByIndex, portByID, portToIFIndexMap)

				// 5. ARP Table for IP <-> MAC resolution
				ipToMac, macToIP := getNetworkARPTable(agent)

				// 6. FDB Discovery (Edge port & Cascade heuristic)
				findFDBTopology(agent, nw, ret, allNodes, allNets, allPollings, existingLines, portByIndex, portByID, portToIFIndexMap, ipToMac, macToIP)

				// 7. ARP Direct Discovery (Router/switch port to node connections)
				findARPTopology(agent, nw, ret, allNodes, allPollings, existingLines, portByIndex, portByID)
			} else {
				nw.Error = fmt.Sprintf("SNMP connect err=%s", err)
			}
		}
	}

	// 8. If no lines found via SNMP, fallback to Subnet Heuristics
	if len(ret.Lines) == 0 {
		subnetLines := inferSubnetLinesForSwitch(nw, allNodes)
		for _, sl := range subnetLines {
			if !hasLineConnection(existingLines, sl.NodeID1, sl.NodeID2) && !hasNeighborLine(ret.Lines, &sl.LineEnt) {
				ret.Lines = append(ret.Lines, sl)
			}
		}
	}

	return ret, nil
}

// FindAllTopology searches for candidate connections across all networks and nodes.
func FindAllTopology(ctx context.Context, store datastore.DataStore) (*FindNeighborNetworksAndLinesResp, error) {
	allNets, err := store.ListNetworks(ctx)
	if err != nil {
		return nil, err
	}
	existingLines, _ := store.ListLines(ctx)
	resp := &FindNeighborNetworksAndLinesResp{
		Networks: []*datastore.NetworkEnt{},
		Lines:    []NeighborLineEnt{},
	}
	seenLines := make(map[string]bool)
	for _, l := range existingLines {
		seenLines[fmt.Sprintf("%s-%s", l.NodeID1, l.NodeID2)] = true
		seenLines[fmt.Sprintf("%s-%s", l.NodeID2, l.NodeID1)] = true
	}

	for _, nw := range allNets {
		top, err := FindTopologyForNetwork(ctx, store, nw)
		if err == nil && top != nil {
			for _, l := range top.Lines {
				k := fmt.Sprintf("%s-%s", l.NodeID1, l.NodeID2)
				if !seenLines[k] {
					seenLines[k] = true
					seenLines[fmt.Sprintf("%s-%s", l.NodeID2, l.NodeID1)] = true
					resp.Lines = append(resp.Lines, l)
				}
			}
		}
	}
	return resp, nil
}

// FindNodeConnection searches candidate switch connections for a specific regular node.
func FindNodeConnection(ctx context.Context, store datastore.DataStore, nodeID string) ([]NeighborLineEnt, error) {
	cleanID := strings.TrimPrefix(nodeID, "NODE:")
	node, err := store.GetNode(ctx, cleanID)
	if err != nil || node == nil {
		return nil, fmt.Errorf("node not found: %s", cleanID)
	}

	existingLines, _ := store.ListLines(ctx)
	allNets, _ := store.ListNetworks(ctx)
	var candidates []NeighborLineEnt

	// 1. Scan switches with SNMP
	for _, nw := range allNets {
		if nw.Unmanaged || nw.IP == "" {
			continue
		}
		top, err := FindTopologyForNetwork(ctx, store, nw)
		if err == nil && top != nil {
			for _, l := range top.Lines {
				if l.NodeID2 == node.ID || l.NodeID1 == node.ID {
					if !hasNeighborLine(candidates, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
						candidates = append(candidates, l)
					}
				}
			}
		}
	}

	// 2. If no SNMP connections found, use subnet heuristic matching
	if len(candidates) == 0 {
		subLines := inferSubnetLinesForNode(node, allNets)
		for _, l := range subLines {
			if !hasNeighborLine(candidates, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
				candidates = append(candidates, l)
			}
		}
	}

	return candidates, nil
}

// ConnectCandidateLines saves a batch of candidate lines into the datastore.
func ConnectCandidateLines(ctx context.Context, store datastore.DataStore, lines []datastore.LineEnt) (int, error) {
	count := 0
	for _, l := range lines {
		lCopy := l
		if lCopy.Width <= 0 {
			lCopy.Width = 2
		}
		if lCopy.State == "" {
			lCopy.State = "normal"
		}
		if err := store.SaveLine(ctx, &lCopy); err == nil {
			count++
		}
	}
	return count, nil
}

// AutoConnectLines automatically detects and creates lines across all network nodes.
func AutoConnectLines(ctx context.Context, store datastore.DataStore, mode int) (int, int, error) {
	if mode <= datastore.AutoLineNone {
		return 0, 0, nil
	}

	strictCount := 0
	speculativeCount := 0
	var allSpeculativeLines []NeighborLineEnt

	allNets, err := store.ListNetworks(ctx)
	if err != nil {
		return 0, 0, err
	}
	existingLines, err := store.ListLines(ctx)
	if err != nil {
		return 0, 0, err
	}

	hasLine := func(l *datastore.LineEnt) bool {
		for _, el := range existingLines {
			if (el.NodeID1 == l.NodeID1 && el.NodeID2 == l.NodeID2) ||
				(el.NodeID1 == l.NodeID2 && el.NodeID2 == l.NodeID1) {
				return true
			}
		}
		return false
	}

	// 1. Gather lines from all switches
	for _, nw := range allNets {
		resp, err := FindTopologyForNetwork(ctx, store, nw)
		if err != nil {
			slog.Warn("AutoConnectLines FindTopologyForNetwork error", "net", nw.Name, "err", err)
			continue
		}

		for _, l := range resp.Lines {
			lineCopy := l.LineEnt
			if l.Confidence == "strict" {
				if !hasLine(&lineCopy) {
					if err := store.SaveLine(ctx, &lineCopy); err == nil {
						existingLines = append(existingLines, &lineCopy)
						strictCount++
					}
				}
			} else if mode == datastore.AutoLineSpeculative {
				allSpeculativeLines = append(allSpeculativeLines, l)
			}
		}
	}

	// 2. Connect speculative lines if requested
	if mode == datastore.AutoLineSpeculative {
		for _, l := range allSpeculativeLines {
			lineCopy := l.LineEnt
			if !hasLine(&lineCopy) {
				if err := store.SaveLine(ctx, &lineCopy); err == nil {
					existingLines = append(existingLines, &lineCopy)
					speculativeCount++
				}
			}
		}
	}

	if strictCount > 0 || speculativeCount > 0 {
		_ = store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "system",
			Level: "info",
			Event: fmt.Sprintf("Auto connected lines: %d strict, %d speculative", strictCount, speculativeCount),
		})
	}

	return strictCount, speculativeCount, nil
}

// =============================================================================
// SNMP Topology Discovery Implementations (LLDP, CDP, STP, FDB, ARP)
// =============================================================================

// findLLDPTopology gathers LLDP neighbors and links
func findLLDPTopology(
	agent *gosnmp.GoSNMP,
	nw *datastore.NetworkEnt,
	ret *FindNeighborNetworksAndLinesResp,
	allNodes []*datastore.NodeEnt,
	allNets []*datastore.NetworkEnt,
	allPollings []*datastore.PollingEnt,
	existingLines []*datastore.LineEnt,
	portByIndex, portByID map[string]*datastore.PortEnt,
) {
	remoteMap := make(map[string]*datastore.NetworkEnt)

	lldpOID := mib.NameToOID("lldpRemoteSystemsData")
	_ = agent.Walk(lldpOID, func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.SplitN(name, ".", 2)
		if len(a) != 2 {
			return nil
		}
		if _, ok := remoteMap[a[1]]; !ok {
			remoteMap[a[1]] = &datastore.NetworkEnt{}
		}
		rn := remoteMap[a[1]]

		switch a[0] {
		case "lldpRemChassisId":
			rn.SystemID = mib.GetMIBValueString(a[0], &variable, false)
		case "lldpRemPortId":
			b := strings.Split(a[1], ".")
			if len(b) >= 2 {
				id := mib.GetMIBValueString(a[0], &variable, false)
				rn.Ports = append(rn.Ports, datastore.PortEnt{
					ID:    id,
					Index: b[1],
					Name:  id,
					X:     len(nw.Ports),
				})
			}
		case "lldpRemSysName":
			rn.Name = mib.GetMIBValueString(a[0], &variable, false)
		case "lldpRemSysDesc":
			rn.Descr = mib.GetMIBValueString(a[0], &variable, false)
		case "lldpRemSysCapEnabled":
			rn.Descr += " " + mib.GetMIBValueString(a[0], &variable, false)
		case "lldpRemManAddrIfId", "lldpRemManAddrOID", "lldpRemManAddrIfSubtype":
			// Extract IPv4 management address from OID suffix: .1.4.x.x.x.x
			b := strings.Split(a[1], ".")
			for i := 0; i+5 <= len(b); i++ {
				if b[i] == "1" && b[i+1] == "4" { // subtype IPv4(1), length 4
					ip := strings.Join(b[i+2:i+6], ".")
					prefix := strings.Join(b[:i], ".")
					if targetRn, ok := remoteMap[prefix]; ok && targetRn.IP == "" {
						targetRn.IP = ip
					}
					break
				}
			}
		}
		return nil
	})

	for _, rn := range remoteMap {
		rnr := findNetwork(allNets, rn.SystemID, rn.IP)
		if rnr == nil {
			// Check if this LLDP neighbor matches a regular node (e.g. Linux server, AP, router)
			node := findNodeFromIP(allNodes, rn.IP)
			if node == nil {
				node = findNodeFromMAC(allNodes, rn.SystemID)
			}
			if node == nil && rn.Name != "" {
				for _, nd := range allNodes {
					if strings.EqualFold(nd.Name, rn.Name) {
						node = nd
						break
					}
				}
			}

			if node != nil {
				// Matched a regular node via LLDP!
				for _, frp := range rn.Ports {
					lp := resolvePort(frp.Index, portByIndex, portByID, nw)
					if lp != nil {
						pid := findBestPollingIDForNode(allPollings, node, frp.Index)
						l := NeighborLineEnt{
							LineEnt: datastore.LineEnt{
								NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
								PollingID1: lp.ID,
								NodeID2:    node.ID,
								PollingID2: pid,
								Width:      2,
								State:      "normal",
								Info:       "LLDP",
							},
							Confidence: "strict",
							Reason:     "LLDP",
						}
						if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
							ret.Lines = append(ret.Lines, l)
						}
					}
				}
			} else {
				// Unregistered Network switch/router
				rnCopy := *rn
				rnCopy.SnmpMode = nw.SnmpMode
				rnCopy.Community = nw.Community
				rnCopy.Password = nw.Password
				rnCopy.User = nw.User
				rnCopy.HPorts = nw.HPorts
				rnCopy.Ports = []datastore.PortEnt{}
				rnCopy.Y = nw.Y + nw.H
				rnCopy.X = nw.X
				ret.Networks = append(ret.Networks, &rnCopy)
			}
		} else {
			// Registered Network switch
			for _, rp := range rnr.Ports {
				for _, frp := range rn.Ports {
					if frp.ID == rp.ID || frp.Name == rp.Name || frp.Index == rp.Index {
						lp := resolvePort(frp.Index, portByIndex, portByID, nw)
						if lp != nil {
							l := NeighborLineEnt{
								LineEnt: datastore.LineEnt{
									NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
									PollingID1: lp.ID,
									NodeID2:    fmt.Sprintf("NET:%s", rnr.ID),
									PollingID2: rp.ID,
									Width:      2,
									State:      "normal",
									Info:       "LLDP",
								},
								Confidence: "strict",
								Reason:     "LLDP",
							}
							if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
								ret.Lines = append(ret.Lines, l)
							}
						}
					}
				}
			}
		}
	}
}

// findCDPTopology gathers Cisco CDP neighbors and links
func findCDPTopology(
	agent *gosnmp.GoSNMP,
	nw *datastore.NetworkEnt,
	ret *FindNeighborNetworksAndLinesResp,
	allNodes []*datastore.NodeEnt,
	allNets []*datastore.NetworkEnt,
	allPollings []*datastore.PollingEnt,
	existingLines []*datastore.LineEnt,
	portByIndex, portByID map[string]*datastore.PortEnt,
) {
	type cdpNeighbor struct {
		LocalIfIndex string
		DeviceID     string
		Address      string
		DevicePort   string
		Platform     string
	}
	cdpNeighbors := make(map[string]*cdpNeighbor)

	cdpOID := mib.NameToOID("cdpCacheEntry")
	_ = agent.Walk(cdpOID, func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.SplitN(name, ".", 2)
		if len(a) != 2 {
			return nil
		}
		parts := strings.Split(a[1], ".")
		if len(parts) < 2 {
			return nil
		}
		ifIndex := parts[0]
		key := a[1]
		if _, ok := cdpNeighbors[key]; !ok {
			cdpNeighbors[key] = &cdpNeighbor{LocalIfIndex: ifIndex}
		}
		nb := cdpNeighbors[key]

		switch a[0] {
		case "cdpCacheDeviceId":
			nb.DeviceID = mib.GetMIBValueString(a[0], &variable, false)
		case "cdpCacheDevicePort":
			nb.DevicePort = mib.GetMIBValueString(a[0], &variable, false)
		case "cdpCachePlatform":
			nb.Platform = mib.GetMIBValueString(a[0], &variable, false)
		case "cdpCacheAddress":
			switch val := variable.Value.(type) {
			case []byte:
				if len(val) == 4 {
					nb.Address = net.IP(val).String()
				}
			case string:
				if len(val) == 4 {
					nb.Address = net.IP([]byte(val)).String()
				}
			}
		}
		return nil
	})

	for _, cdp := range cdpNeighbors {
		if cdp.DeviceID == "" && cdp.Address == "" {
			continue
		}
		lp := resolvePort(cdp.LocalIfIndex, portByIndex, portByID, nw)
		if lp == nil {
			continue
		}

		// Try to match network node first
		rnr := findNetwork(allNets, "", cdp.Address)
		if rnr == nil && cdp.DeviceID != "" {
			for _, netEnt := range allNets {
				if strings.EqualFold(netEnt.Name, cdp.DeviceID) {
					rnr = netEnt
					break
				}
			}
		}

		if rnr != nil {
			rPortID := ""
			for _, rp := range rnr.Ports {
				if strings.EqualFold(rp.Name, cdp.DevicePort) || rp.Index == cdp.DevicePort {
					rPortID = rp.ID
					break
				}
			}
			if rPortID == "" && len(rnr.Ports) > 0 {
				rPortID = rnr.Ports[0].ID
			}

			l := NeighborLineEnt{
				LineEnt: datastore.LineEnt{
					NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
					PollingID1: lp.ID,
					NodeID2:    fmt.Sprintf("NET:%s", rnr.ID),
					PollingID2: rPortID,
					Width:      2,
					State:      "normal",
					Info:       "CDP",
				},
				Confidence: "strict",
				Reason:     "CDP",
			}
			if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
				ret.Lines = append(ret.Lines, l)
			}
			continue
		}

		// Try to match regular node
		node := findNodeFromIP(allNodes, cdp.Address)
		if node == nil && cdp.DeviceID != "" {
			for _, nd := range allNodes {
				if strings.EqualFold(nd.Name, cdp.DeviceID) {
					node = nd
					break
				}
			}
		}
		if node != nil {
			pid := findBestPollingIDForNode(allPollings, node, cdp.LocalIfIndex)
			l := NeighborLineEnt{
				LineEnt: datastore.LineEnt{
					NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
					PollingID1: lp.ID,
					NodeID2:    node.ID,
					PollingID2: pid,
					Width:      2,
					State:      "normal",
					Info:       "CDP",
				},
				Confidence: "strict",
				Reason:     "CDP",
			}
			if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
				ret.Lines = append(ret.Lines, l)
			}
		}
	}
}

// findSTPTopology discovers switch-to-switch links via STP Designated Bridge
func findSTPTopology(
	agent *gosnmp.GoSNMP,
	nw *datastore.NetworkEnt,
	ret *FindNeighborNetworksAndLinesResp,
	allNets []*datastore.NetworkEnt,
	existingLines []*datastore.LineEnt,
	portByIndex, portByID map[string]*datastore.PortEnt,
	portToIFIndexMap map[int]int,
) {
	stpOID := mib.NameToOID("dot1dStpPortDesignatedBridge")
	_ = agent.Walk(stpOID, func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.SplitN(name, ".", 2)
		if len(a) != 2 {
			return nil
		}
		portNum, err := strconv.Atoi(a[1])
		if err != nil {
			return nil
		}
		ifIndex := portNum
		if mapped, ok := portToIFIndexMap[portNum]; ok {
			ifIndex = mapped
		}
		lp := resolvePort(strconv.Itoa(ifIndex), portByIndex, portByID, nw)
		if lp == nil {
			return nil
		}

		// Designated bridge is 8 bytes: 2 bytes priority + 6 bytes MAC address
		var mac string
		switch val := variable.Value.(type) {
		case []byte:
			if len(val) >= 8 {
				mac = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", val[2], val[3], val[4], val[5], val[6], val[7])
			} else if len(val) == 6 {
				mac = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", val[0], val[1], val[2], val[3], val[4], val[5])
			}
		case string:
			b := []byte(val)
			if len(b) >= 8 {
				mac = fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", b[2], b[3], b[4], b[5], b[6], b[7])
			}
		}

		if mac == "" || normalizeMAC(mac) == normalizeMAC(nw.SystemID) {
			return nil
		}

		normMac := normalizeMAC(mac)
		for _, rNet := range allNets {
			if rNet.ID != nw.ID && normalizeMAC(rNet.SystemID) == normMac {
				l := NeighborLineEnt{
					LineEnt: datastore.LineEnt{
						NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
						PollingID1: lp.ID,
						NodeID2:    fmt.Sprintf("NET:%s", rNet.ID),
						PollingID2: "",
						Width:      2,
						State:      "normal",
						Info:       "STP",
					},
					Confidence: "strict",
					Reason:     "STP",
				}
				if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
					ret.Lines = append(ret.Lines, l)
				}
				break
			}
		}
		return nil
	})
}

// findFDBTopology analyzes FDB (Forwarding Database) with edge port distinction and cascade heuristics
func findFDBTopology(
	agent *gosnmp.GoSNMP,
	nw *datastore.NetworkEnt,
	ret *FindNeighborNetworksAndLinesResp,
	allNodes []*datastore.NodeEnt,
	allNets []*datastore.NetworkEnt,
	allPollings []*datastore.PollingEnt,
	existingLines []*datastore.LineEnt,
	portByIndex, portByID map[string]*datastore.PortEnt,
	portToIFIndexMap map[int]int,
	ipToMac, macToIP map[string]string,
) {
	fdbList := getEnhancedFDB(agent, portToIFIndexMap)

	// Count MACs learned per port
	portMacCount := make(map[int]int)
	for _, e := range fdbList {
		portMacCount[e.IfIndex]++
	}

	// Identify switch/router MACs to avoid treating them as simple endpoints
	knownSwitchMACs := make(map[string]bool)
	for _, netEnt := range allNets {
		if netEnt.SystemID != "" {
			knownSwitchMACs[normalizeMAC(netEnt.SystemID)] = true
		}
	}

	for _, e := range fdbList {
		normMAC := normalizeMAC(e.MAC)
		if normMAC == "" || knownSwitchMACs[normMAC] {
			continue // Handled by LLDP/CDP/STP
		}

		// Find node matching this MAC or corresponding IP
		node := findNodeFromMAC(allNodes, e.MAC)
		if node == nil {
			if ip, ok := macToIP[normMAC]; ok {
				node = findNodeFromIP(allNodes, ip)
			}
		}
		if node == nil {
			continue
		}

		idxStr := strconv.Itoa(e.IfIndex)
		lp := resolvePort(idxStr, portByIndex, portByID, nw)
		if lp == nil {
			continue
		}

		macCount := portMacCount[e.IfIndex]
		confidence := "strict"
		reason := "FDB-Edge"

		if macCount > 2 {
			confidence = "speculative"
			reason = fmt.Sprintf("FDB-Heuristic (%d MACs)", macCount)
		}

		pid := findBestPollingIDForNode(allPollings, node, idxStr)
		l := NeighborLineEnt{
			LineEnt: datastore.LineEnt{
				NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
				PollingID1: lp.ID,
				NodeID2:    node.ID,
				PollingID2: pid,
				Width:      2,
				State:      "normal",
				Info:       reason,
			},
			Confidence: confidence,
			Reason:     reason,
		}

		if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
			ret.Lines = append(ret.Lines, l)
		}
	}
}

// findARPTopology analyzes ARP table to link router/switch ports to nodes
func findARPTopology(
	agent *gosnmp.GoSNMP,
	nw *datastore.NetworkEnt,
	ret *FindNeighborNetworksAndLinesResp,
	allNodes []*datastore.NodeEnt,
	allPollings []*datastore.PollingEnt,
	existingLines []*datastore.LineEnt,
	portByIndex, portByID map[string]*datastore.PortEnt,
) {
	type arpEntry struct {
		IfIndex string
		IP      string
		MAC     string
	}
	var arpEntries []arpEntry
	portArpCount := make(map[string]int)

	arpOID := mib.NameToOID("ipNetToMediaPhysAddress")
	err := agent.Walk(arpOID, func(variable gosnmp.SnmpPDU) error {
		a := strings.SplitN(mib.OIDToName(variable.Name), ".", 2)
		if len(a) != 2 {
			return nil
		}
		parts := strings.Split(a[1], ".")
		if len(parts) >= 5 {
			ifIndex := parts[0]
			ip := strings.Join(parts[len(parts)-4:], ".")
			mac := mib.GetMIBValueString(a[0], &variable, false)
			if ip != "" {
				arpEntries = append(arpEntries, arpEntry{
					IfIndex: ifIndex,
					IP:      ip,
					MAC:     mac,
				})
				portArpCount[ifIndex]++
			}
		}
		return nil
	})
	if err != nil || len(arpEntries) == 0 {
		atOID := mib.NameToOID("atPhysAddress")
		_ = agent.Walk(atOID, func(variable gosnmp.SnmpPDU) error {
			a := strings.SplitN(mib.OIDToName(variable.Name), ".", 2)
			if len(a) != 2 {
				return nil
			}
			parts := strings.Split(a[1], ".")
			if len(parts) >= 5 {
				ifIndex := parts[0]
				ip := strings.Join(parts[len(parts)-4:], ".")
				mac := mib.GetMIBValueString(a[0], &variable, false)
				if ip != "" {
					arpEntries = append(arpEntries, arpEntry{
						IfIndex: ifIndex,
						IP:      ip,
						MAC:     mac,
					})
					portArpCount[ifIndex]++
				}
			}
			return nil
		})
	}

	for _, e := range arpEntries {
		node := findNodeFromIP(allNodes, e.IP)
		if node == nil && e.MAC != "" {
			node = findNodeFromMAC(allNodes, e.MAC)
		}
		if node == nil {
			continue
		}
		lp := resolvePort(e.IfIndex, portByIndex, portByID, nw)
		if lp == nil {
			continue
		}

		confidence := "strict"
		reason := "ARP"
		if portArpCount[e.IfIndex] > 2 {
			confidence = "speculative"
			reason = fmt.Sprintf("ARP-Multi (%d)", portArpCount[e.IfIndex])
		}

		pid := findBestPollingIDForNode(allPollings, node, e.IfIndex)
		l := NeighborLineEnt{
			LineEnt: datastore.LineEnt{
				NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
				PollingID1: lp.ID,
				NodeID2:    node.ID,
				PollingID2: pid,
				Width:      2,
				State:      "normal",
				Info:       reason,
			},
			Confidence: confidence,
			Reason:     reason,
		}
		if !hasNeighborLine(ret.Lines, &l.LineEnt) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
			ret.Lines = append(ret.Lines, l)
		}
	}
}

// getEnhancedFDB reads dot1qTpFdbPort with fallback to dot1dTpFdbPort
func getEnhancedFDB(agent *gosnmp.GoSNMP, portToIFIndexMap map[int]int) []*FDBTableEnt {
	var ret []*FDBTableEnt

	// Try Q-BRIDGE-MIB dot1qTpFdbPort
	dot1qOID := mib.NameToOID("dot1qTpFdbPort")
	_ = agent.Walk(dot1qOID, func(variable gosnmp.SnmpPDU) error {
		a := strings.Split(mib.OIDToName(variable.Name), ".")
		if len(a) != 1+1+6 {
			return nil
		}
		vlan, err := strconv.Atoi(a[1])
		if err != nil {
			return nil
		}
		mac, err := indexToMacAddress(a[2:])
		if err != nil {
			return nil
		}
		port := int(gosnmp.ToBigInt(variable.Value).Int64())
		if port == 0 {
			return nil
		}
		ifIndex := port
		if mapped, ok := portToIFIndexMap[port]; ok {
			ifIndex = mapped
		}

		ret = append(ret, &FDBTableEnt{
			MAC:     mac,
			VLanID:  vlan,
			Port:    port,
			IfIndex: ifIndex,
		})
		return nil
	})

	// Fallback to standard BRIDGE-MIB dot1dTpFdbPort if Q-BRIDGE was empty
	if len(ret) == 0 {
		dot1dOID := mib.NameToOID("dot1dTpFdbPort")
		_ = agent.Walk(dot1dOID, func(variable gosnmp.SnmpPDU) error {
			a := strings.Split(mib.OIDToName(variable.Name), ".")
			if len(a) != 1+6 {
				return nil
			}
			mac, err := indexToMacAddress(a[1:])
			if err != nil {
				return nil
			}
			port := int(gosnmp.ToBigInt(variable.Value).Int64())
			if port == 0 {
				return nil
			}
			ifIndex := port
			if mapped, ok := portToIFIndexMap[port]; ok {
				ifIndex = mapped
			}

			ret = append(ret, &FDBTableEnt{
				MAC:     mac,
				VLanID:  0,
				Port:    port,
				IfIndex: ifIndex,
			})
			return nil
		})
	}

	return ret
}

// getNetworkARPTable retrieves ARP mappings from router/switch SNMP agent
func getNetworkARPTable(agent *gosnmp.GoSNMP) (map[string]string, map[string]string) {
	ipToMac := make(map[string]string)
	macToIP := make(map[string]string)

	arpOID := mib.NameToOID("ipNetToMediaPhysAddress")
	err := agent.Walk(arpOID, func(variable gosnmp.SnmpPDU) error {
		a := strings.SplitN(mib.OIDToName(variable.Name), ".", 2)
		if len(a) != 2 {
			return nil
		}
		parts := strings.Split(a[1], ".")
		if len(parts) >= 5 {
			ip := strings.Join(parts[len(parts)-4:], ".")
			mac := mib.GetMIBValueString(a[0], &variable, false)
			if ip != "" && mac != "" {
				normMac := normalizeMAC(mac)
				ipToMac[ip] = normMac
				macToIP[normMac] = ip
			}
		}
		return nil
	})
	if err != nil || len(ipToMac) == 0 {
		atOID := mib.NameToOID("atPhysAddress")
		_ = agent.Walk(atOID, func(variable gosnmp.SnmpPDU) error {
			a := strings.SplitN(mib.OIDToName(variable.Name), ".", 2)
			if len(a) != 2 {
				return nil
			}
			parts := strings.Split(a[1], ".")
			if len(parts) >= 5 {
				ip := strings.Join(parts[len(parts)-4:], ".")
				mac := mib.GetMIBValueString(a[0], &variable, false)
				if ip != "" && mac != "" {
					normMac := normalizeMAC(mac)
					ipToMac[ip] = normMac
					macToIP[normMac] = ip
				}
			}
			return nil
		})
	}
	return ipToMac, macToIP
}

// =============================================================================
// Helper Functions
// =============================================================================

func normalizeMAC(s string) string {
	clean := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(strings.ReplaceAll(s, "-", ""), ":", ""), ".", ""))
	if len(clean) != 12 {
		return strings.ToUpper(strings.TrimSpace(s))
	}
	return fmt.Sprintf("%s:%s:%s:%s:%s:%s", clean[0:2], clean[2:4], clean[4:6], clean[6:8], clean[8:10], clean[10:12])
}

func indexToMacAddress(a []string) (string, error) {
	var ret []string
	for _, s := range a {
		if i, err := strconv.Atoi(s); err == nil {
			ret = append(ret, fmt.Sprintf("%02X", i))
		} else {
			return "", err
		}
	}
	return strings.Join(ret, ":"), nil
}

func findNetwork(networks []*datastore.NetworkEnt, systemID, ip string) *datastore.NetworkEnt {
	normSys := normalizeMAC(systemID)
	for _, nw := range networks {
		if normSys != "" && normalizeMAC(nw.SystemID) == normSys {
			return nw
		}
		if ip != "" && nw.IP == ip {
			return nw
		}
	}
	return nil
}

func findNodeFromIP(nodes []*datastore.NodeEnt, ip string) *datastore.NodeEnt {
	if ip == "" {
		return nil
	}
	for _, nd := range nodes {
		if nd.IP == ip {
			return nd
		}
	}
	return nil
}

func findNodeFromMAC(nodes []*datastore.NodeEnt, mac string) *datastore.NodeEnt {
	norm := normalizeMAC(mac)
	if norm == "" {
		return nil
	}
	for _, nd := range nodes {
		if nd.MAC != "" && normalizeMAC(nd.MAC) == norm {
			return nd
		}
	}
	return nil
}

func resolvePort(indexOrID string, portByIndex, portByID map[string]*datastore.PortEnt, nw *datastore.NetworkEnt) *datastore.PortEnt {
	if p, ok := portByIndex[indexOrID]; ok {
		return p
	}
	if p, ok := portByID[indexOrID]; ok {
		return p
	}
	for i := range nw.Ports {
		if nw.Ports[i].Index == indexOrID || nw.Ports[i].ID == indexOrID || nw.Ports[i].Name == indexOrID {
			return &nw.Ports[i]
		}
	}
	return nil
}

func findBestPollingIDForNode(pollings []*datastore.PollingEnt, node *datastore.NodeEnt, ifIndex string) string {
	pid := ""
	pcmp := fmt.Sprintf("ifOperStatus.%s", ifIndex)
	for _, p := range pollings {
		if p.NodeID == node.ID {
			if p.Type == "snmp" && p.Params == pcmp {
				return p.ID
			}
			if pid == "" {
				pid = p.ID
			} else if p.Type == "ping" {
				pid = p.ID
			}
		}
	}
	return pid
}

func hasLineConnection(lines []*datastore.LineEnt, n1, n2 string) bool {
	for _, l := range lines {
		if (l.NodeID1 == n1 && l.NodeID2 == n2) || (l.NodeID1 == n2 && l.NodeID2 == n1) {
			return true
		}
	}
	return false
}

func hasNeighborLine(lines []NeighborLineEnt, l1 *datastore.LineEnt) bool {
	for _, l := range lines {
		if (l.NodeID1 == l1.NodeID1 && l.PollingID1 == l1.PollingID1 &&
			l.NodeID2 == l1.NodeID2 && l.PollingID2 == l1.PollingID2) ||
			(l.NodeID1 == l1.NodeID2 && l.PollingID1 == l1.PollingID2 &&
				l.NodeID2 == l1.NodeID1 && l.PollingID2 == l1.PollingID1) {
			return true
		}
	}
	return false
}

func inferSubnetLinesForSwitch(nw *datastore.NetworkEnt, nodes []*datastore.NodeEnt) []NeighborLineEnt {
	var results []NeighborLineEnt
	switchIP := net.ParseIP(nw.IP).To4()
	if switchIP == nil {
		return results
	}

	for i, nd := range nodes {
		nodeIP := net.ParseIP(nd.IP).To4()
		if nodeIP == nil {
			continue
		}

		if switchIP[0] == nodeIP[0] && switchIP[1] == nodeIP[1] && switchIP[2] == nodeIP[2] {
			portID := ""
			if len(nw.Ports) > 0 {
				portIdx := i % len(nw.Ports)
				portID = nw.Ports[portIdx].ID
			}

			reason := fmt.Sprintf("Subnet Heuristic (/24 - %s)", nw.Name)
			results = append(results, NeighborLineEnt{
				LineEnt: datastore.LineEnt{
					NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
					PollingID1: portID,
					NodeID2:    nd.ID,
					PollingID2: "",
					Width:      2,
					State:      "normal",
					Info:       reason,
				},
				Confidence: "speculative",
				Reason:     reason,
			})
		}
	}
	return results
}

func inferSubnetLinesForNode(node *datastore.NodeEnt, networks []*datastore.NetworkEnt) []NeighborLineEnt {
	var results []NeighborLineEnt
	nodeIP := net.ParseIP(node.IP).To4()
	if nodeIP == nil || len(networks) == 0 {
		return results
	}

	var bestNet *datastore.NetworkEnt
	for _, nw := range networks {
		swIP := net.ParseIP(nw.IP).To4()
		if swIP != nil && swIP[0] == nodeIP[0] && swIP[1] == nodeIP[1] && swIP[2] == nodeIP[2] {
			bestNet = nw
			break
		}
	}

	if bestNet == nil && len(networks) > 0 {
		bestNet = networks[0]
	}

	if bestNet != nil {
		portID := ""
		if len(bestNet.Ports) > 0 {
			portID = bestNet.Ports[0].ID
		}
		reason := fmt.Sprintf("Subnet Heuristic (%s)", bestNet.Name)
		results = append(results, NeighborLineEnt{
			LineEnt: datastore.LineEnt{
				NodeID1:    fmt.Sprintf("NET:%s", bestNet.ID),
				PollingID1: portID,
				NodeID2:    node.ID,
				PollingID2: "",
				Width:      2,
				State:      "normal",
				Info:       reason,
			},
			Confidence: "speculative",
			Reason:     reason,
		})
	}
	return results
}
