package receiver

import (
	"bufio"
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

// ArpWatchConfig holds configuration for the ARP Watch engine.
type ArpWatchConfig struct {
	Store    datastore.DataStore
	LogStore *parquet.Store
	Enabled  bool
	Range    string
	Timeout  int // Retention in hours/days (0 = default)
}

// ArpWatchServer watches local ARP entries and detects new devices and IP/MAC changes.
type ArpWatchServer struct {
	cfg ArpWatchConfig

	arpTable         sync.Map // IP (string) -> MAC (string)
	macToIPTable     sync.Map // MAC (string) -> IP (string)
	localSegment     []*net.IPNet
	localCheckAddrs  []string
	lastAddressUsage map[string]float64
	pinger           *pinger
	mu               sync.Mutex
}

// NewArpWatchServer creates a new ARP Watch server instance.
func NewArpWatchServer(cfg ArpWatchConfig) *ArpWatchServer {
	return &ArpWatchServer{
		cfg:              cfg,
		lastAddressUsage: make(map[string]float64),
		pinger:           newPinger(),
	}
}

// Start launches the ARP watch engine and periodic sweeps until ctx is canceled.
func (s *ArpWatchServer) Start(ctx context.Context) error {
	if !s.cfg.Enabled {
		slog.Info("ARP Watch is disabled")
		return nil
	}

	slog.Info("Starting ARP Watch Engine...", "range", s.cfg.Range)
	s.setLocalSegment()

	if s.cfg.Store != nil {
		_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
			Time:  time.Now().UnixNano(),
			Type:  "arpwatch",
			Level: "info",
			Event: "ARP監視を開始しました",
		})

		// Restore existing table from store
		if saved, err := s.cfg.Store.LoadArpTable(ctx); err == nil {
			for _, e := range saved {
				if e.IP != "" && e.MAC != "" {
					s.arpTable.Store(e.IP, e.MAC)
					s.macToIPTable.Store(e.MAC, e.IP)
				}
			}
		}
	}

	// Initial check
	s.checkArpTable(ctx)
	s.makeLocalCheckAddrs(ctx)

	probeTicker := time.NewTicker(200 * time.Millisecond)
	checkTicker := time.NewTicker(60 * time.Second)
	defer probeTicker.Stop()
	defer checkTicker.Stop()
	defer func() {
		if s.pinger != nil {
			s.pinger.close()
		}
	}()

	for {
		select {
		case <-ctx.Done():
			slog.Info("Stopping ARP Watch Engine...")
			s.saveArpTableToStore(ctx)
			if s.cfg.Store != nil {
				_ = s.cfg.Store.AddEventLog(context.Background(), &datastore.EventLogEnt{
					Time:  time.Now().UnixNano(),
					Type:  "arpwatch",
					Level: "info",
					Event: "ARP監視を停止しました",
				})
			}
			return nil

		case <-probeTicker.C:
			s.mu.Lock()
			if len(s.localCheckAddrs) > 0 {
				targetIP := s.localCheckAddrs[0]
				s.localCheckAddrs = s.localCheckAddrs[1:]
				s.mu.Unlock()
				s.sendProbe(targetIP)
			} else {
				s.mu.Unlock()
			}

		case <-checkTicker.C:
			s.checkArpTable(ctx)
			s.mu.Lock()
			needRefill := len(s.localCheckAddrs) == 0
			s.mu.Unlock()
			if needRefill {
				s.makeLocalCheckAddrs(ctx)
			}
		}
	}
}

// setLocalSegment collects IPv4 subnets from all active physical local interfaces.
func (s *ArpWatchServer) setLocalSegment() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.localSegment = []*net.IPNet{}

	ifs, err := net.Interfaces()
	if err != nil {
		slog.Warn("Failed to enumerate network interfaces", "error", err)
		return
	}

	for _, iface := range ifs {
		if (iface.Flags&net.FlagLoopback) != 0 ||
			(iface.Flags&net.FlagPointToPoint) != 0 ||
			(iface.Flags&net.FlagUp) == 0 ||
			len(iface.HardwareAddr) != 6 {
			continue
		}

		addrs, err := iface.Addrs()
		if err != nil {
			continue
		}

		for _, addr := range addrs {
			_, ipnet, err := net.ParseCIDR(addr.String())
			if err != nil || ipnet.IP.To4() == nil {
				continue
			}
			s.localSegment = append(s.localSegment, ipnet)
			slog.Debug("Added local network segment", "interface", iface.Name, "subnet", ipnet.String())
		}
	}
}

func (s *ArpWatchServer) isOnLocalSegment(ip net.IP) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, n := range s.localSegment {
		if n.Contains(ip) {
			return true
		}
	}
	return false
}

// makeLocalCheckAddrs expands target range and queues addresses needing probe.
func (s *ArpWatchServer) makeLocalCheckAddrs(ctx context.Context) {
	rangeStr := s.cfg.Range
	if strings.TrimSpace(rangeStr) == "" {
		// Default to all discovered local segments
		s.mu.Lock()
		segments := make([]string, len(s.localSegment))
		for i, n := range s.localSegment {
			segments[i] = n.String()
		}
		s.mu.Unlock()
		rangeStr = strings.Join(segments, ",")
	}

	if rangeStr == "" {
		return
	}

	ipMap := make(map[string]bool)
	var newCheckAddrs []string

	for _, r := range strings.Split(rangeStr, ",") {
		r = strings.TrimSpace(r)
		if r == "" {
			continue
		}

		var sIP, eIP uint32
		if strings.Contains(r, "-") {
			parts := strings.SplitN(r, "-", 2)
			p1 := net.ParseIP(strings.TrimSpace(parts[0])).To4()
			p2 := net.ParseIP(strings.TrimSpace(parts[1])).To4()
			if p1 == nil || p2 == nil {
				continue
			}
			sIP = ip2int(p1)
			eIP = ip2int(p2)
		} else {
			// CIDR
			ip, ipnet, err := net.ParseCIDR(r)
			if err != nil || ip.To4() == nil {
				continue
			}
			sIP = ip2int(ip.To4())
			for eIP = sIP; ipnet.Contains(int2ip(eIP)); eIP++ {
			}
			eIP--
		}

		if sIP >= eIP {
			continue
		}

		localIPCount := 0
		localHitCount := 0

		for nIP := sIP; nIP <= eIP; nIP++ {
			ip := int2ip(nIP)
			if !ip.IsGlobalUnicast() || ip.IsMulticast() || !s.isOnLocalSegment(ip) {
				continue
			}
			sa := ip.String()
			localIPCount++

			if _, ok := s.arpTable.Load(sa); ok {
				localHitCount++
				ipMap[sa] = true
				continue
			}

			if !ipMap[sa] {
				ipMap[sa] = true
				newCheckAddrs = append(newCheckAddrs, sa)
			}
		}

		if localIPCount > 0 {
			lau := 100.0 * float64(localHitCount) / float64(localIPCount)
			s.mu.Lock()
			oldLau, exists := s.lastAddressUsage[r]
			s.lastAddressUsage[r] = lau
			s.mu.Unlock()

			if !exists || fmt.Sprintf("%.1f", oldLau) != fmt.Sprintf("%.1f", lau) {
				if s.cfg.Store != nil {
					_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
						Time:  time.Now().UnixNano(),
						Type:  "arpwatch",
						Level: "info",
						Event: fmt.Sprintf("ARP監視範囲 %s 利用率: %d/%d (%.2f%%)", r, localHitCount, localIPCount, lau),
					})
				}
			}
		}
	}

	s.mu.Lock()
	s.localCheckAddrs = newCheckAddrs
	s.mu.Unlock()
}

// sendProbe sends an ICMP Echo (PING) or benign UDP probe to trigger OS kernel ARP address resolution.
func (s *ArpWatchServer) sendProbe(ip string) {
	if s.pinger != nil {
		s.pinger.sendPing(ip)
	}
}

// checkArpTable queries the operating system's ARP table and evaluates changes.
func (s *ArpWatchServer) checkArpTable(ctx context.Context) {
	var lines []string
	if runtime.GOOS == "windows" {
		lines = s.getArpOutputWindows()
	} else {
		lines = s.getArpOutputUnix()
	}

	for _, line := range lines {
		ip, mac := parseArpLine(line)
		if ip != "" && mac != "" {
			s.updateArpTable(ctx, ip, mac)
		}
	}

	s.checkNodeMAC(ctx)
	s.saveArpTableToStore(ctx)
}

// GetArpTable returns all current ARP entries from the in-memory table.
func (s *ArpWatchServer) GetArpTable() []*datastore.ArpEnt {
	var entries []*datastore.ArpEnt
	now := time.Now().Unix()
	s.arpTable.Range(func(k, v any) bool {
		ip := k.(string)
		mac := v.(string)
		entries = append(entries, &datastore.ArpEnt{
			IP:        ip,
			MAC:       mac,
			Vendor:    datastore.FindVendor(mac),
			FirstTime: now,
			LastTime:  now,
		})
		return true
	})
	return entries
}

// DeleteEntries removes the specified IP entries from memory.
func (s *ArpWatchServer) DeleteEntries(ips []string) {
	for _, ip := range ips {
		if macVal, ok := s.arpTable.LoadAndDelete(ip); ok {
			if mac, ok := macVal.(string); ok {
				s.macToIPTable.Delete(mac)
			}
		}
	}
}

// ResetTable removes all entries from memory.
func (s *ArpWatchServer) ResetTable() {
	s.arpTable.Range(func(k, _ any) bool {
		s.arpTable.Delete(k)
		return true
	})
	s.macToIPTable.Range(func(k, _ any) bool {
		s.macToIPTable.Delete(k)
		return true
	})
}

// pinger manages non-root ICMP Echo transmission for ARP table population.
type pinger struct {
	conn     *icmp.PacketConn
	isUDP    bool
	fallback bool
}

func newPinger() *pinger {
	netProto := "udp4"
	if runtime.GOOS != "darwin" && runtime.GOOS != "windows" {
		if c, err := icmp.ListenPacket("ip4:icmp", "0.0.0.0"); err == nil {
			return &pinger{conn: c, isUDP: false}
		}
	}
	if c, err := icmp.ListenPacket(netProto, "0.0.0.0"); err == nil {
		return &pinger{conn: c, isUDP: true}
	}
	// Fallback to UDP echo probe if ICMP sockets are completely unavailable
	return &pinger{fallback: true}
}

func (p *pinger) sendPing(targetIP string) {
	if p.fallback || p.conn == nil {
		// Benign UDP probe fallback (echo port 7)
		d := net.Dialer{Timeout: 50 * time.Millisecond}
		conn, err := d.Dial("udp4", net.JoinHostPort(targetIP, "7"))
		if err == nil {
			_, _ = conn.Write([]byte{0})
			_ = conn.Close()
		}
		return
	}

	dstIP := net.ParseIP(targetIP)
	if dstIP == nil {
		return
	}

	wm := icmp.Message{
		Type: ipv4.ICMPTypeEcho,
		Code: 0,
		Body: &icmp.Echo{
			ID:   os.Getpid() & 0xffff,
			Seq:  1,
			Data: []byte("TWSNMP_NEO_ARP_PING"),
		},
	}
	wb, err := wm.Marshal(nil)
	if err != nil {
		return
	}

	if p.isUDP {
		_, _ = p.conn.WriteTo(wb, &net.UDPAddr{IP: dstIP})
	} else {
		_, _ = p.conn.WriteTo(wb, &net.IPAddr{IP: dstIP})
	}
}

func (p *pinger) close() {
	if p.conn != nil {
		_ = p.conn.Close()
	}
}

func (s *ArpWatchServer) getArpOutputWindows() []string {
	out, err := exec.Command("arp", "-a").Output()
	if err != nil {
		slog.Debug("arp -a failed", "error", err)
		return nil
	}
	return strings.Split(string(out), "\n")
}

func (s *ArpWatchServer) getArpOutputUnix() []string {
	// Try standard arp -an command
	out, err := exec.Command("arp", "-an").Output()
	if err == nil {
		return strings.Split(string(out), "\n")
	}

	// Fallback for Linux containers where `arp` CLI binary is absent
	if runtime.GOOS == "linux" {
		if f, err := os.Open("/proc/net/arp"); err == nil {
			defer f.Close()
			var res []string
			scanner := bufio.NewScanner(f)
			if scanner.Scan() {
				// Skip header line
			}
			for scanner.Scan() {
				fields := strings.Fields(scanner.Text())
				if len(fields) >= 4 && fields[2] != "0x0" {
					res = append(res, fmt.Sprintf("? (%s) at %s on %s", fields[0], fields[3], fields[5]))
				}
			}
			return res
		}
	}

	slog.Debug("arp command execution failed", "error", err)
	return nil
}

// parseArpLine parses an ARP table line into clean IP and MAC.
func parseArpLine(line string) (ip, mac string) {
	line = strings.TrimSpace(line)
	if line == "" || strings.Contains(line, "(incomplete)") {
		return "", ""
	}

	// Unix / macOS: "? (192.168.1.1) at 0:11:22:33:44:55 on en0..."
	if strings.Contains(line, " at ") && strings.Contains(line, "(") && strings.Contains(line, ")") {
		start := strings.Index(line, "(")
		end := strings.Index(line, ")")
		if start < end {
			ip = line[start+1 : end]
		}
		atIdx := strings.Index(line, " at ")
		if atIdx != -1 {
			afterAt := strings.TrimSpace(line[atIdx+4:])
			fields := strings.Fields(afterAt)
			if len(fields) > 0 {
				mac = fields[0]
			}
		}
		return ip, mac
	}

	// Windows: "192.168.1.1    00-11-22-33-44-55     dynamic"
	fields := strings.Fields(line)
	if len(fields) >= 2 {
		potentialIP := fields[0]
		potentialMAC := fields[1]
		if strings.Contains(potentialIP, ".") && strings.ContainsAny(potentialMAC, ":-") {
			return potentialIP, potentialMAC
		}
	}
	return "", ""
}

// updateArpTable compares current IP/MAC with cached state and writes logs on changes.
func (s *ArpWatchServer) updateArpTable(ctx context.Context, ip, mac string) {
	if !strings.Contains(ip, ".") || !strings.ContainsAny(mac, ":-") {
		return
	}
	mac = normMACAddr(mac)
	if strings.HasPrefix(mac, "FF:") || strings.HasPrefix(mac, "01:") {
		return // Broadcast and multicast
	}

	vendor := datastore.FindVendor(mac)
	nodeName := ""
	if s.cfg.Store != nil {
		if nodes, err := s.cfg.Store.ListNodes(ctx); err == nil {
			for _, n := range nodes {
				if n.IP == ip || normMACAddr(n.MAC) == mac {
					nodeName = n.Name
					break
				}
			}
		}
	}

	oldVal, exists := s.arpTable.Load(ip)
	if !exists {
		// New entry discovered
		s.arpTable.Store(ip, mac)
		s.macToIPTable.Store(mac, ip)

		logEnt := datastore.ArpLogEnt{
			Time:      time.Now().UnixNano(),
			State:     "New",
			IP:        ip,
			Node:      nodeName,
			NewMAC:    mac,
			NewVendor: vendor,
		}

		s.writeParquetLog(&logEnt)

		if s.cfg.Store != nil {
			_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:     logEnt.Time,
				Type:     "arpwatch",
				Level:    "info",
				NodeName: nodeName,
				Event:    fmt.Sprintf("新規MACアドレス検知 %s (%s - %s)", ip, mac, vendor),
			})
		}
		slog.Info("ARP Watch: New device detected", "ip", ip, "mac", mac, "vendor", vendor)
		return
	}

	oldMAC := oldVal.(string)
	if oldMAC != mac {
		// MAC address change detected!
		s.arpTable.Store(ip, mac)
		s.macToIPTable.Store(mac, ip)

		oldVendor := datastore.FindVendor(oldMAC)

		logEnt := datastore.ArpLogEnt{
			Time:      time.Now().UnixNano(),
			State:     "Change",
			IP:        ip,
			Node:      nodeName,
			NewMAC:    mac,
			NewVendor: vendor,
			OldMAC:    oldMAC,
			OldVendor: oldVendor,
		}

		s.writeParquetLog(&logEnt)

		if s.cfg.Store != nil {
			_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
				Time:     logEnt.Time,
				Type:     "arpwatch",
				Level:    "warn",
				NodeName: nodeName,
				Event:    fmt.Sprintf("MACアドレス変更検知 %s (%s -> %s)", ip, oldMAC, mac),
			})
		}
		slog.Warn("ARP Watch: MAC address change detected", "ip", ip, "oldMAC", oldMAC, "newMAC", mac)
		return
	}

	// No change - refresh association
	s.macToIPTable.Store(mac, ip)
}

func (s *ArpWatchServer) writeParquetLog(ent *datastore.ArpLogEnt) {
	if s.cfg.LogStore == nil {
		return
	}
	rawJSON, err := json.Marshal(ent)
	if err != nil {
		return
	}
	_ = s.cfg.LogStore.WriteLog(&parquet.ParquetLogRecord{
		Time: ent.Time,
		Type: "arplog",
		Src:  ent.IP,
		Log:  string(rawJSON),
	})
}

// checkNodeMAC populates or updates MAC address for managed nodes.
func (s *ArpWatchServer) checkNodeMAC(ctx context.Context) {
	if s.cfg.Store == nil {
		return
	}
	nodes, err := s.cfg.Store.ListNodes(ctx)
	if err != nil {
		return
	}

	for _, n := range nodes {
		if n.IP == "" {
			continue
		}
		if v, ok := s.arpTable.Load(n.IP); ok {
			mac := v.(string)
			if n.MAC == "" {
				n.MAC = mac
				if n.Vendor == "" || n.Vendor == "Unknown" {
					n.Vendor = datastore.FindVendor(mac)
				}
				_ = s.cfg.Store.SaveNode(ctx, n)
				_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
					Time:     time.Now().UnixNano(),
					Type:     "arpwatch",
					Level:    "info",
					NodeID:   n.ID,
					NodeName: n.Name,
					Event:    fmt.Sprintf("ノード %s のMACアドレスを自動登録しました: %s (%s)", n.Name, mac, n.Vendor),
				})
			} else if normMACAddr(n.MAC) != mac {
				oldMAC := n.MAC
				n.MAC = mac
				n.Vendor = datastore.FindVendor(mac)
				_ = s.cfg.Store.SaveNode(ctx, n)
				_ = s.cfg.Store.AddEventLog(ctx, &datastore.EventLogEnt{
					Time:     time.Now().UnixNano(),
					Type:     "arpwatch",
					Level:    "warn",
					NodeID:   n.ID,
					NodeName: n.Name,
					Event:    fmt.Sprintf("ノード %s のMACアドレス変更を検知・更新しました: %s -> %s", n.Name, oldMAC, mac),
				})
			}
		}
	}
}

func (s *ArpWatchServer) saveArpTableToStore(ctx context.Context) {
	if s.cfg.Store == nil {
		return
	}
	var entries []*datastore.ArpEnt
	now := time.Now().Unix()
	s.arpTable.Range(func(k, v any) bool {
		ip := k.(string)
		mac := v.(string)
		entries = append(entries, &datastore.ArpEnt{
			IP:        ip,
			MAC:       mac,
			Vendor:    datastore.FindVendor(mac),
			FirstTime: now,
			LastTime:  now,
		})
		return true
	})
	_ = s.cfg.Store.SaveArpTable(ctx, entries)
}

// normMACAddr formats MAC into uppercase colon-delimited string (e.g. 00:11:22:33:44:55).
func normMACAddr(m string) string {
	if hw, err := net.ParseMAC(m); err == nil {
		return strings.ToUpper(hw.String())
	}
	m = strings.ReplaceAll(m, "-", ":")
	parts := strings.Split(m, ":")
	var b strings.Builder
	for i, p := range parts {
		if i > 0 {
			b.WriteString(":")
		}
		if len(p) == 1 {
			b.WriteString("0")
		}
		b.WriteString(p)
	}
	return strings.ToUpper(b.String())
}

func ip2int(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func int2ip(nIP uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, nIP)
	return ip
}
