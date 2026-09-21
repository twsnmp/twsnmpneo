package topology

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// NeighborLineEnt represents a candidate topology line with confidence rating and rationale.
type NeighborLineEnt struct {
	datastore.LineEnt
	Confidence string `json:"Confidence"` // "strict" | "speculative"
	Reason     string `json:"Reason"`     // "LLDP" | "CDP" | "STP" | "FDB-Edge" | "Subnet Heuristic" | "AI"
}

// FindNeighborNetworksAndLinesResp returns detected candidate lines and neighboring networks.
type FindNeighborNetworksAndLinesResp struct {
	Networks []*datastore.NetworkEnt `json:"Networks"`
	Lines    []NeighborLineEnt       `json:"Lines"`
}

// FindTopologyForNetwork searches for candidate connections and adjacent networks for a switch.
func FindTopologyForNetwork(ctx context.Context, store datastore.DataStore, nw *datastore.NetworkEnt) (*FindNeighborNetworksAndLinesResp, error) {
	resp := &FindNeighborNetworksAndLinesResp{
		Networks: []*datastore.NetworkEnt{},
		Lines:    []NeighborLineEnt{},
	}

	if nw == nil {
		return resp, fmt.Errorf("network is nil")
	}

	existingLines, _ := store.ListLines(ctx)
	allNodes, _ := store.ListNodes(ctx)
	allNets, _ := store.ListNetworks(ctx)

	// 1. Try SNMP-based discovery if managed and SNMP is configured
	if !nw.Unmanaged && nw.IP != "" && (nw.SnmpMode != "" || nw.Community != "") {
		_ = discoverSNMPTopology(nw, allNodes, allNets, resp)
	}

	// 2. If no lines found via SNMP or switch is unmanaged, fallback to Subnet Heuristic
	if len(resp.Lines) == 0 {
		subnetLines := inferSubnetLinesForSwitch(nw, allNodes)
		for _, sl := range subnetLines {
			if !hasLineConnection(existingLines, sl.NodeID1, sl.NodeID2) {
				resp.Lines = append(resp.Lines, sl)
			}
		}
	}

	// Filter out lines that already exist in the datastore
	filteredLines := []NeighborLineEnt{}
	for _, l := range resp.Lines {
		if !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
			filteredLines = append(filteredLines, l)
		}
	}
	resp.Lines = filteredLines

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
	candidates := []NeighborLineEnt{}

	// 1. Scan switches with SNMP
	for _, nw := range allNets {
		if nw.Unmanaged || nw.IP == "" {
			continue
		}
		top, err := FindTopologyForNetwork(ctx, store, nw)
		if err == nil {
			for _, l := range top.Lines {
				if (l.NodeID1 == node.ID || l.NodeID2 == node.ID) && !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
					candidates = append(candidates, l)
				}
			}
		}
	}

	// 2. If no SNMP connections found, use subnet heuristic matching
	if len(candidates) == 0 {
		subLines := inferSubnetLinesForNode(node, allNets)
		for _, l := range subLines {
			if !hasLineConnection(existingLines, l.NodeID1, l.NodeID2) {
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

// Helper: check if a connection line exists between two nodes/networks
func hasLineConnection(lines []*datastore.LineEnt, n1, n2 string) bool {
	for _, l := range lines {
		if (l.NodeID1 == n1 && l.NodeID2 == n2) || (l.NodeID1 == n2 && l.NodeID2 == n1) {
			return true
		}
	}
	return false
}

// Helper: infer subnet matching lines for a switch and regular nodes
func inferSubnetLinesForSwitch(nw *datastore.NetworkEnt, nodes []*datastore.NodeEnt) []NeighborLineEnt {
	results := []NeighborLineEnt{}
	switchIP := net.ParseIP(nw.IP).To4()
	if switchIP == nil {
		return results
	}

	for i, nd := range nodes {
		nodeIP := net.ParseIP(nd.IP).To4()
		if nodeIP == nil {
			continue
		}

		// Match first 3 octets (/24 subnet)
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

// Helper: infer subnet matching lines for a single node across switches
func inferSubnetLinesForNode(node *datastore.NodeEnt, networks []*datastore.NetworkEnt) []NeighborLineEnt {
	results := []NeighborLineEnt{}
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

	// Fallback to first switch if no /24 match
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

// Helper: SNMP walk and topology analysis
func discoverSNMPTopology(nw *datastore.NetworkEnt, allNodes []*datastore.NodeEnt, allNets []*datastore.NetworkEnt, resp *FindNeighborNetworksAndLinesResp) error {
	port := uint16(161)
	if nw.SnmpPort > 0 {
		port = uint16(nw.SnmpPort)
	}

	community := "public"
	if nw.Community != "" {
		community = nw.Community
	}

	agent := &gosnmp.GoSNMP{
		Target:    nw.IP,
		Port:      port,
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   1 * time.Second,
		Retries:   0,
	}

	if err := agent.Connect(); err != nil {
		return err
	}
	if agent.Conn != nil {
		defer agent.Conn.Close()
	}

	// 1. LLDP discovery (.1.0.8802.1.1.2.1.4.1.1)
	_ = agent.Walk(".1.0.8802.1.1.2.1.4.1.1", func(pdu gosnmp.SnmpPDU) error {
		// Parse remote systems info
		return nil
	})

	// 2. FDB discovery (.1.3.6.1.2.1.17.4.3.1.2: dot1dTpFdbPort)
	macToPort := make(map[string]int)
	_ = agent.Walk(".1.3.6.1.2.1.17.4.3.1.2", func(pdu gosnmp.SnmpPDU) error {
		parts := strings.Split(pdu.Name, ".")
		if len(parts) >= 6 {
			macParts := parts[len(parts)-6:]
			var b []string
			for _, mp := range macParts {
				val, _ := strconv.Atoi(mp)
				b = append(b, fmt.Sprintf("%02X", val))
			}
			mac := strings.Join(b, ":")
			if p, ok := pdu.Value.(int); ok {
				macToPort[mac] = p
			}
		}
		return nil
	})

	// Match MACs from FDB table with known nodes
	for _, nd := range allNodes {
		if nd.MAC == "" {
			continue
		}
		cleanMac := strings.ToUpper(strings.ReplaceAll(strings.ReplaceAll(nd.MAC, "-", ":"), ".", ":"))
		if portIdx, found := macToPort[cleanMac]; found {
			portID := ""
			for _, p := range nw.Ports {
				if p.Index == strconv.Itoa(portIdx) || p.Name == fmt.Sprintf("#%d", portIdx) {
					portID = p.ID
					break
				}
			}
			if portID == "" && len(nw.Ports) > 0 {
				portID = nw.Ports[0].ID
			}

			resp.Lines = append(resp.Lines, NeighborLineEnt{
				LineEnt: datastore.LineEnt{
					NodeID1:    fmt.Sprintf("NET:%s", nw.ID),
					PollingID1: portID,
					NodeID2:    nd.ID,
					PollingID2: "",
					Width:      2,
					State:      "normal",
					Info:       "FDB-Edge",
				},
				Confidence: "strict",
				Reason:     "FDB-Edge",
			})
		}
	}

	return nil
}
