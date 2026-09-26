package discover

import (
	"context"
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"log"
	"math/rand"
	"net"
	"net/http"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/layout"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
	"github.com/twsnmp/twsnmpneo/backend/internal/ping"
	"github.com/twsnmp/twsnmpneo/backend/internal/topology"
)

const GRID = 90

var (
	titleRegex = regexp.MustCompile(`(?i)<title[^>]*>(.*?)</title>`)
)

// DiscoverStat holds statistics of discovery process.
type DiscoverStat struct {
	Running   bool   `json:"Running"`
	Total     uint32 `json:"Total"`
	Sent      uint32 `json:"Sent"`
	Found     uint32 `json:"Found"`
	Snmp      uint32 `json:"Snmp"`
	Web       uint32 `json:"Web"`
	Mail      uint32 `json:"Mail"`
	SSH       uint32 `json:"SSH"`
	File      uint32 `json:"File"`
	RDP       uint32 `json:"RDP"`
	LDAP      uint32 `json:"LDAP"`
	Wait      int    `json:"Wait"`
	StartTime int64  `json:"StartTime"`
	Now       int64  `json:"Now"`
}

type discoverInfoEnt struct {
	IP          string
	HostName    string
	SysName     string
	SysObjectID string
	SysDescr    string
	MAC         string
	Vendor      string
	HTTPTitle   string
	HTTPServer  string
	HTTPBody    string
	IfMap       map[string]string
	ServerList  map[string]bool
	X           int
	Y           int
	SnmpConf    *datastore.SnmpConfEnt
}

// Engine manages the discovery lifecycle.
type Engine struct {
	mu     sync.Mutex
	stat   DiscoverStat
	stop   bool
	cancel context.CancelFunc
	store  datastore.DataStore
	posX   int
	posY   int
}

var defaultEngine = &Engine{}

// GetDefaultEngine returns the singleton discovery engine.
func GetDefaultEngine() *Engine {
	return defaultEngine
}

// GetDiscoverStats returns current statistics.
func (e *Engine) GetDiscoverStats() DiscoverStat {
	e.mu.Lock()
	defer e.mu.Unlock()
	return e.stat
}

// StopDiscover cancels running discovery process.
func (e *Engine) StopDiscover() {
	e.mu.Lock()
	e.stop = true
	if e.cancel != nil {
		e.cancel()
	}
	e.mu.Unlock()

	st := time.Now()
	for {
		e.mu.Lock()
		running := e.stat.Running
		e.mu.Unlock()
		if !running || time.Since(st) > 3*time.Second {
			break
		}
		time.Sleep(100 * time.Millisecond)
	}
}

// GetDiscoverAddressRange returns local interface IPv4 ranges (StartIP, EndIP pairs).
func GetDiscoverAddressRange() []string {
	ret := []string{}
	ifs, err := net.Interfaces()
	if err != nil {
		return ret
	}
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
			ipTmp, ipnet, err := net.ParseCIDR(cidr)
			if err != nil {
				continue
			}
			ip := ipTmp.To4()
			if ip == nil {
				continue
			}
			start := ip.Mask(ipnet.Mask)
			mask := ipnet.Mask
			end := net.IP(make([]byte, 4))
			for idx := range ip {
				end[idx] = ip[idx] | ^mask[idx]
			}
			if end[3] > 0 {
				end[3] -= 1
			}
			ret = append(ret, start.String())
			ret = append(ret, end.String())
		}
	}
	return ret
}

func ipToUint32(ipStr string) (uint32, error) {
	ip := net.ParseIP(ipStr).To4()
	if ip == nil {
		return 0, fmt.Errorf("invalid IPv4: %s", ipStr)
	}
	return binary.BigEndian.Uint32(ip), nil
}

func uint32ToIP(n uint32) string {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip.String()
}

// StartDiscover starts the discovery scan in background.
func (e *Engine) StartDiscover(ctx context.Context, store datastore.DataStore, conf *datastore.DiscoverConfEnt) error {
	e.mu.Lock()
	if e.stat.Running {
		e.mu.Unlock()
		return fmt.Errorf("discovery is already running")
	}

	sip, err := ipToUint32(conf.StartIP)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("start IP error: %w", err)
	}
	eip, err := ipToUint32(conf.EndIP)
	if err != nil {
		e.mu.Unlock()
		return fmt.Errorf("end IP error: %w", err)
	}
	if sip > eip {
		e.mu.Unlock()
		return fmt.Errorf("start IP > end IP")
	}

	e.store = store
	e.stop = false
	e.stat = DiscoverStat{
		Total:     eip - sip + 1,
		Sent:      0,
		Found:     0,
		Snmp:      0,
		Web:       0,
		Mail:      0,
		SSH:       0,
		File:      0,
		RDP:       0,
		LDAP:      0,
		Wait:      0,
		Running:   true,
		StartTime: time.Now().Unix(),
		Now:       time.Now().Unix(),
	}

	e.posX = (1 + conf.X/GRID) * GRID
	e.posY = (1 + conf.Y/GRID) * GRID

	scanCtx, cancel := context.WithCancel(context.Background())
	e.cancel = cancel
	e.mu.Unlock()

	_ = store.AddEventLog(scanCtx, &datastore.EventLogEnt{
		Type:  "system",
		Level: "info",
		Event: fmt.Sprintf("Start discover %s - %s", conf.StartIP, conf.EndIP),
	})

	go e.runScan(scanCtx, conf, sip, eip)

	return nil
}

func (e *Engine) runScan(ctx context.Context, conf *datastore.DiscoverConfEnt, sip, eip uint32) {
	defer func() {
		e.mu.Lock()
		e.stat.Running = false
		e.stat.Now = time.Now().Unix()
		e.mu.Unlock()

		_ = e.store.AddEventLog(context.Background(), &datastore.EventLogEnt{
			Type:  "system",
			Level: "info",
			Event: fmt.Sprintf("End discover %s - %s (found: %d)", conf.StartIP, conf.EndIP, e.stat.Found),
		})
	}()

	sem := make(chan struct{}, 16)
	portScanSem := make(chan struct{}, 2)
	pacer := time.NewTicker(80 * time.Millisecond)
	defer pacer.Stop()

	mapConf, _ := e.store.GetMapConf(ctx)

	for cur := sip; cur <= eip; cur++ {
		select {
		case <-ctx.Done():
			return
		case <-pacer.C:
		}

		e.mu.Lock()
		if e.stop {
			e.mu.Unlock()
			return
		}
		e.stat.Sent++
		e.stat.Now = time.Now().Unix()
		e.mu.Unlock()

		sem <- struct{}{}
		go func(ipNum uint32) {
			defer func() { <-sem }()

			e.mu.Lock()
			if e.stop {
				e.mu.Unlock()
				return
			}
			e.mu.Unlock()

			ipStr := uint32ToIP(ipNum)

			// 1. Check existing node
			nodes, _ := e.store.ListNodes(ctx)
			var existingNode *datastore.NodeEnt
			for _, n := range nodes {
				if n.IP == ipStr {
					existingNode = n
					break
				}
			}

			if existingNode != nil && !conf.ReCheck {
				return
			}

			// 2. Connectivity check (Ping)
			if !e.doPing(ipStr, conf.Timeout, conf.Retry) {
				return
			}

			dent := &discoverInfoEnt{
				IP:         ipStr,
				IfMap:      make(map[string]string),
				ServerList: make(map[string]bool),
			}

			// 3. ARP cache / Vendor lookup
			arps, _ := e.store.LoadArpTable(ctx)
			for _, a := range arps {
				if a.IP == ipStr {
					dent.MAC = a.MAC
					dent.Vendor = a.Vendor
					break
				}
			}
			if dent.Vendor == "" && dent.MAC != "" {
				dent.Vendor = datastore.FindVendor(dent.MAC)
			}
			if existingNode != nil {
				if dent.MAC == "" && existingNode.MAC != "" {
					dent.MAC = existingNode.MAC
				}
				if (dent.Vendor == "" || dent.Vendor == "Unknown") && existingNode.Vendor != "" {
					dent.Vendor = existingNode.Vendor
				}
			}

			// 4. DNS Reverse lookup
			r := &net.Resolver{}
			dnsCtx, dnsCancel := context.WithTimeout(ctx, 2*time.Second)
			names, err := r.LookupAddr(dnsCtx, ipStr)
			dnsCancel()
			if err == nil && len(names) > 0 {
				dent.HostName = strings.TrimRight(names[0], ".")
			}

			// 5. SNMP scan
			e.getSnmpInfo(ipStr, conf, mapConf, dent)

			// 6. Port scan
			if conf.PortScan {
				select {
				case <-ctx.Done():
					return
				case portScanSem <- struct{}{}:
					e.checkServer(dent, conf.Timeout)
					<-portScanSem
				}
			}

			// 7. Update Datastore & Stats
			e.mu.Lock()
			dent.X = e.posX
			dent.Y = e.posY
			e.stat.Found++

			if existingNode == nil {
				e.posX += GRID
				if e.posX > GRID*10 {
					e.posX = GRID
					e.posY += GRID
				}
			}

			if conf.AddNetwork && (dent.ServerList["lldp"] || dent.ServerList["bridge"]) {
				nets, _ := e.store.ListNetworks(ctx)
				hasNet := false
				for _, netw := range nets {
					if netw.IP == ipStr {
						hasNet = true
						break
					}
				}
				if !hasNet {
					e.posX = GRID
					e.posY += GRID
				}
			}

			if dent.SysName != "" || dent.SysObjectID != "" {
				e.stat.Snmp++
			}
			if dent.ServerList["http"] || dent.ServerList["https"] {
				e.stat.Web++
			}
			if dent.ServerList["cifs"] || dent.ServerList["nfs"] {
				e.stat.File++
			}
			if dent.ServerList["rdp"] || dent.ServerList["vnc"] {
				e.stat.RDP++
			}
			if dent.ServerList["ldap"] || dent.ServerList["ldaps"] || dent.ServerList["kerberos"] {
				e.stat.LDAP++
			}
			if dent.ServerList["smtp"] || dent.ServerList["imap"] || dent.ServerList["pop3"] {
				e.stat.Mail++
			}
			if dent.ServerList["ssh"] {
				e.stat.SSH++
			}

			if existingNode == nil {
				e.addFoundNode(ctx, dent, conf, mapConf)
			} else {
				e.updateNode(ctx, existingNode, dent, conf, mapConf)
			}
			e.mu.Unlock()
		}(cur)
	}

	// Wait for ongoing tasks
	for len(sem) > 0 {
		time.Sleep(10 * time.Millisecond)
		e.mu.Lock()
		e.stat.Wait = len(sem)
		e.stat.Now = time.Now().Unix()
		e.mu.Unlock()
	}

	// Auto Line Connection
	if conf.AutoLine > datastore.AutoLineNone {
		_, _, _ = topology.AutoConnectLines(ctx, e.store, conf.AutoLine)
	}

	// Auto Layout
	if conf.AutoLayout > datastore.AutoLayoutNone {
		_, _ = layout.OptimizeLayout(ctx, e.store, conf.AutoLayout)
	}
}

// doPing checks connectivity with timeout and retries.
func (e *Engine) doPing(ipStr string, timeoutSec, retry int) bool {
	if timeoutSec <= 0 {
		timeoutSec = 1
	}

	pe := ping.DoPing(ipStr, timeoutSec, retry, 64, 0)
	return pe.Stat == ping.PingOK
}

// getSnmpConfigs creates candidate SNMP settings ordered by preference.
func getSnmpConfigs(conf *datastore.DiscoverConfEnt, mapConf *datastore.MapConfEnt) []datastore.SnmpConfEnt {
	var list []datastore.SnmpConfEnt
	if mapConf != nil && mapConf.SnmpMode != "" && mapConf.SnmpMode != "none" {
		list = append(list, datastore.SnmpConfEnt{
			SnmpMode:     mapConf.SnmpMode,
			Community:    mapConf.Community,
			SnmpUser:     mapConf.SnmpUser,
			SnmpPassword: mapConf.SnmpPassword,
		})
	}
	for _, sc := range conf.SnmpConfigs {
		if sc.SnmpMode == "" {
			continue
		}
		if len(list) > 0 &&
			sc.SnmpMode == list[0].SnmpMode &&
			sc.Community == list[0].Community &&
			sc.SnmpUser == list[0].SnmpUser &&
			sc.SnmpPassword == list[0].SnmpPassword {
			continue
		}
		list = append(list, sc)
	}
	if len(list) == 0 {
		list = append(list, datastore.SnmpConfEnt{
			SnmpMode:  "v2c",
			Community: "public",
		})
	}
	return list
}

func (e *Engine) getSnmpInfo(target string, conf *datastore.DiscoverConfEnt, mapConf *datastore.MapConfEnt, dent *discoverInfoEnt) {
	for _, sc := range getSnmpConfigs(conf, mapConf) {
		scCopy := sc
		if e.trySnmp(target, &scCopy, conf.Timeout, conf.Retry, dent) {
			break
		}
	}
}

func (e *Engine) trySnmp(target string, snmpConf *datastore.SnmpConfEnt, timeoutSec, retry int, dent *discoverInfoEnt) bool {
	if timeoutSec <= 0 {
		timeoutSec = 1
	}
	agent := &gosnmp.GoSNMP{
		Target:    target,
		Port:      161,
		Transport: "udp",
		Community: snmpConf.Community,
		Version:   gosnmp.Version2c,
		Timeout:   time.Duration(timeoutSec) * time.Second,
		Retries:   retry,
		MaxOids:   gosnmp.MaxOids,
	}

	switch snmpConf.SnmpMode {
	case "v1":
		agent.Version = gosnmp.Version1
	case "v3auth":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthNoPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 snmpConf.SnmpUser,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: snmpConf.SnmpPassword,
		}
	case "v3authpriv":
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		agent.MsgFlags = gosnmp.AuthPriv
		agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
			UserName:                 snmpConf.SnmpUser,
			AuthenticationProtocol:   gosnmp.SHA,
			AuthenticationPassphrase: snmpConf.SnmpPassword,
			PrivacyProtocol:          gosnmp.AES,
			PrivacyPassphrase:        snmpConf.SnmpPassword,
		}
	}

	err := agent.Connect()
	if err != nil {
		return false
	}
	defer agent.Conn.Close()

	// Get basic MIB sys info
	oids := []string{
		".1.3.6.1.2.1.1.5.0", // sysName.0
		".1.3.6.1.2.1.1.2.0", // sysObjectID.0
		".1.3.6.1.2.1.1.1.0", // sysDescr.0
	}
	result, err := agent.Get(oids)
	if err != nil || len(result.Variables) == 0 {
		return false
	}

	hasInfo := false
	for _, variable := range result.Variables {
		switch variable.Name {
		case ".1.3.6.1.2.1.1.5.0":
			dent.SysName = mibToString(variable.Value)
			if dent.SysName != "" {
				hasInfo = true
			}
		case ".1.3.6.1.2.1.1.2.0":
			dent.SysObjectID = mibToString(variable.Value)
			if dent.SysObjectID != "" {
				hasInfo = true
			}
		case ".1.3.6.1.2.1.1.1.0":
			dent.SysDescr = mibToString(variable.Value)
			if dent.SysDescr != "" {
				hasInfo = true
			}
		}
	}

	if !hasInfo {
		return false
	}

	dent.SnmpConf = snmpConf

	// Collect interfaces via ifType (.1.3.6.1.2.1.2.2.1.3)
	_ = agent.Walk(".1.3.6.1.2.1.2.2.1.3", func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.Split(name, ".")
		idxStr := ""
		if len(a) == 2 && a[0] == "ifType" {
			idxStr = a[1]
		} else {
			parts := strings.Split(variable.Name, ".")
			idxStr = parts[len(parts)-1]
		}
		if gosnmp.ToBigInt(variable.Value).Int64() == 6 { // ethernetCsmacd
			dent.IfMap[idxStr] = fmt.Sprintf("#%s", idxStr)
		}
		return nil
	})

	// Collect interface names via ifName (.1.3.6.1.2.1.31.1.1.1.1)
	_ = agent.Walk(".1.3.6.1.2.1.31.1.1.1.1", func(variable gosnmp.SnmpPDU) error {
		name := mib.OIDToName(variable.Name)
		a := strings.Split(name, ".")
		idxStr := ""
		if len(a) == 2 {
			idxStr = a[1]
		} else {
			parts := strings.Split(variable.Name, ".")
			idxStr = parts[len(parts)-1]
		}
		if _, ok := dent.IfMap[idxStr]; ok {
			dent.IfMap[idxStr] = mibToString(variable.Value)
		}
		return nil
	})

	// If any interface names in IfMap are still "#...", try ifDescr (.1.3.6.1.2.1.2.2.1.2)
	needDescr := false
	for _, v := range dent.IfMap {
		if strings.HasPrefix(v, "#") {
			needDescr = true
			break
		}
	}
	if needDescr {
		_ = agent.Walk(".1.3.6.1.2.1.2.2.1.2", func(variable gosnmp.SnmpPDU) error {
			parts := strings.Split(variable.Name, ".")
			idxStr := parts[len(parts)-1]
			if v, ok := dent.IfMap[idxStr]; ok && strings.HasPrefix(v, "#") {
				dent.IfMap[idxStr] = mibToString(variable.Value)
			}
			return nil
		})
	}

	// Check if switch/bridge
	_ = agent.Walk(".1.0.8802.1.1.2", func(pdu gosnmp.SnmpPDU) error {
		dent.ServerList["lldp"] = true
		return fmt.Errorf("found")
	})
	if !dent.ServerList["lldp"] {
		_ = agent.Walk(".1.3.6.1.2.1.17.1.1", func(pdu gosnmp.SnmpPDU) error {
			dent.ServerList["bridge"] = true
			return fmt.Errorf("found")
		})
	}

	return true
}

func mibToString(val interface{}) string {
	switch v := val.(type) {
	case string:
		return v
	case []byte:
		return string(v)
	default:
		return fmt.Sprintf("%v", v)
	}
}

func (e *Engine) checkServer(dent *discoverInfoEnt, timeoutSec int) {
	if timeoutSec <= 0 {
		timeoutSec = 1
	}
	timeout := time.Duration(timeoutSec) * time.Second

	checkList := map[string]string{
		"http":     "80",
		"https":    "443",
		"pop3":     "110",
		"imap":     "143",
		"smtp":     "25",
		"ssh":      "22",
		"cifs":     "445",
		"nfs":      "2049",
		"vnc":      "5900",
		"rdp":      "3389",
		"ldap":     "389",
		"ldaps":    "636",
		"kerberos": "88",
	}

	for s, p := range checkList {
		e.mu.Lock()
		if e.stop {
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()

		d := net.Dialer{Timeout: timeout}
		conn, err := d.Dial("tcp", net.JoinHostPort(dent.IP, p))
		if err == nil {
			_ = conn.Close()
			dent.ServerList[s] = true
		}
	}

	if dent.ServerList["http"] || dent.ServerList["https"] {
		e.checkWebInfo(dent, timeoutSec)
	}
}

func (e *Engine) checkWebInfo(dent *discoverInfoEnt, timeoutSec int) {
	schemes := []string{}
	if dent.ServerList["http"] {
		schemes = append(schemes, "http")
	}
	if dent.ServerList["https"] {
		schemes = append(schemes, "https")
	}
	if len(schemes) == 0 {
		return
	}

	client := &http.Client{
		Timeout: time.Duration(timeoutSec) * time.Second,
		Transport: &http.Transport{
			TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
		},
	}

	for _, scheme := range schemes {
		e.mu.Lock()
		if e.stop {
			e.mu.Unlock()
			return
		}
		e.mu.Unlock()

		url := fmt.Sprintf("%s://%s", scheme, dent.IP)
		req, err := http.NewRequest("GET", url, nil)
		if err != nil {
			continue
		}
		req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; TWSNMP-NEO/1.0)")
		resp, err := client.Do(req)
		if err != nil {
			continue
		}
		dent.HTTPServer = resp.Header.Get("Server")
		bodyBytes, _ := io.ReadAll(io.LimitReader(resp.Body, 16384))
		_ = resp.Body.Close()
		dent.HTTPBody = string(bodyBytes)

		matches := titleRegex.FindStringSubmatch(dent.HTTPBody)
		if len(matches) > 1 {
			dent.HTTPTitle = strings.TrimSpace(matches[1])
		}
		break
	}
}

func (e *Engine) addFoundNode(ctx context.Context, dent *discoverInfoEnt, conf *datastore.DiscoverConfEnt, mapConf *datastore.MapConfEnt) {
	if (dent.Vendor == "" || dent.Vendor == "Unknown") && dent.MAC != "" {
		dent.Vendor = datastore.FindVendor(dent.MAC)
	}

	node := datastore.NodeEnt{
		Name:   dent.HostName,
		IP:     dent.IP,
		MAC:    dent.MAC,
		Vendor: dent.Vendor,
		Icon:   "desktop",
		X:      dent.X,
		Y:      dent.Y,
		Descr:  fmt.Sprintf("Found at %s", time.Now().Format("2006/01/02")),
	}

	if node.Name == "" {
		if dent.SysName != "" {
			node.Name = dent.SysName
		} else {
			node.Name = dent.IP
		}
	}

	if dent.SysObjectID != "" {
		if dent.SnmpConf != nil {
			node.SnmpMode = dent.SnmpConf.SnmpMode
			node.User = dent.SnmpConf.SnmpUser
			node.Password = dent.SnmpConf.SnmpPassword
			node.Community = dent.SnmpConf.Community
		} else if mapConf != nil {
			node.SnmpMode = mapConf.SnmpMode
			node.User = mapConf.SnmpUser
			node.Password = mapConf.SnmpPassword
			node.Community = mapConf.Community
		}
		node.Icon = "hdd"
	}

	var detectRes *datastore.DetectResult
	if conf.AutoDetect {
		input := &datastore.DetectInput{
			IP:          dent.IP,
			Name:        dent.SysName,
			HostName:    dent.HostName,
			SysObjectID: dent.SysObjectID,
			SysDescr:    dent.SysDescr,
			HTTPTitle:   dent.HTTPTitle,
			HTTPServer:  dent.HTTPServer,
			HTTPBody:    dent.HTTPBody,
			Vendor:      dent.Vendor,
		}
		detectRes = datastore.DetectNode(input)
		if detectRes != nil && detectRes.Icon != "" && detectRes.RuleID != "unknown" {
			node.Icon = detectRes.Icon
			if detectRes.Name != "" {
				node.Descr += fmt.Sprintf(" [%s]", detectRes.Name)
			}
		}
	}

	var protoList []string
	for _, s := range []string{"http", "https", "ssh", "cifs", "rdp", "smtp", "imap", "pop3", "ldap"} {
		if dent.ServerList[s] {
			protoList = append(protoList, s)
		}
	}
	if len(protoList) > 0 {
		node.Descr += " Protocol: " + strings.Join(protoList, ",")
	}

	if err := e.store.SaveNode(ctx, &node); err != nil {
		log.Printf("save node error: %v", err)
		return
	}

	_ = e.store.AddEventLog(ctx, &datastore.EventLogEnt{
		Type:     "discover",
		Level:    "info",
		NodeID:   node.ID,
		NodeName: node.Name,
		Event:    "Add by discover",
	})

	// Add Network container if switch/router
	hasSnmp := dent.SysObjectID != ""
	isSwitch := hasSnmp && (dent.ServerList["lldp"] || dent.ServerList["bridge"] ||
		(detectRes != nil && (detectRes.Category == "switch" || detectRes.Icon == "switch")))

	if conf.AddNetwork && isSwitch {
		nw := &datastore.NetworkEnt{
			Name:      node.Name,
			IP:        node.IP,
			X:         node.X + GRID,
			Y:         node.Y,
			SnmpMode:  node.SnmpMode,
			Community: node.Community,
			User:      node.User,
			Password:  node.Password,
			HPorts:    24,
			Ports:     []datastore.PortEnt{},
		}
		if dent.SnmpConf != nil {
			nw.SnmpMode = dent.SnmpConf.SnmpMode
			nw.Community = dent.SnmpConf.Community
			nw.User = dent.SnmpConf.SnmpUser
			nw.Password = dent.SnmpConf.SnmpPassword
		}
		// Query ports via SNMP (LLDP and IF-MIB)
		ports, err := topology.FetchNetworkPorts(ctx, nw, conf.Timeout, conf.Retry)
		if err == nil && len(ports) > 0 {
			nw.Ports = ports
			nw.Error = ""
		} else if len(dent.IfMap) > 0 {
			var ifIndices []int
			for idxStr := range dent.IfMap {
				if n, e := strconv.Atoi(idxStr); e == nil {
					ifIndices = append(ifIndices, n)
				}
			}
			sort.Ints(ifIndices)
			for i, idx := range ifIndices {
				idxStr := strconv.Itoa(idx)
				pName := dent.IfMap[idxStr]
				nw.Ports = append(nw.Ports, datastore.PortEnt{
					ID:      idxStr,
					Name:    pName,
					Index:   idxStr,
					X:       i % 24,
					Y:       i / 24,
					Polling: fmt.Sprintf("ifOperStatus.%s", idxStr),
					State:   "unknown",
				})
			}
		}
		_ = e.store.SaveNetwork(ctx, nw)
	}

	if conf.AddPolling {
		e.addPolling(ctx, dent, &node, mapConf)
	}
}

func (e *Engine) updateNode(ctx context.Context, n *datastore.NodeEnt, dent *discoverInfoEnt, conf *datastore.DiscoverConfEnt, mapConf *datastore.MapConfEnt) {
	if n.Name == n.IP && dent.SysName != "" {
		n.Name = dent.SysName
	}
	if n.MAC == "" && dent.MAC != "" {
		n.MAC = dent.MAC
	}
	if (n.Vendor == "" || n.Vendor == "Unknown") && dent.Vendor != "" {
		n.Vendor = dent.Vendor
	}

	if dent.SysObjectID != "" {
		if dent.SnmpConf != nil {
			n.SnmpMode = dent.SnmpConf.SnmpMode
			n.User = dent.SnmpConf.SnmpUser
			n.Password = dent.SnmpConf.SnmpPassword
			n.Community = dent.SnmpConf.Community
		} else if n.User == "" && n.Community == "" && mapConf != nil {
			n.SnmpMode = mapConf.SnmpMode
			n.User = mapConf.SnmpUser
			n.Password = mapConf.SnmpPassword
			n.Community = mapConf.Community
		}
		if n.Icon == "desktop" {
			n.Icon = "hdd"
		}
	}

	var detectRes *datastore.DetectResult
	if conf.AutoDetect {
		input := &datastore.DetectInput{
			IP:          dent.IP,
			Name:        n.Name,
			HostName:    dent.HostName,
			SysObjectID: dent.SysObjectID,
			SysDescr:    dent.SysDescr,
			HTTPTitle:   dent.HTTPTitle,
			HTTPServer:  dent.HTTPServer,
			HTTPBody:    dent.HTTPBody,
			Vendor:      dent.Vendor,
		}
		detectRes = datastore.DetectNode(input)
		if detectRes != nil && detectRes.Icon != "" {
			if n.Icon == "desktop" || n.Icon == "hdd" || n.Icon == "" || conf.ReCheck {
				n.Icon = detectRes.Icon
			}
		}
	}

	_ = e.store.SaveNode(ctx, n)

	// Check/update Network container if switch/router
	hasSnmp := dent.SysObjectID != ""
	isSwitch := hasSnmp && (dent.ServerList["lldp"] || dent.ServerList["bridge"] ||
		(detectRes != nil && (detectRes.Category == "switch" || detectRes.Icon == "switch")))
	if conf.AddNetwork && isSwitch {
		allNets, _ := e.store.ListNetworks(ctx)
		var existingNet *datastore.NetworkEnt
		for _, netEnt := range allNets {
			if netEnt.IP == n.IP {
				existingNet = netEnt
				break
			}
		}
		if existingNet == nil {
			newNet := &datastore.NetworkEnt{
				Name:      n.Name,
				IP:        n.IP,
				X:         n.X + GRID,
				Y:         n.Y,
				SnmpMode:  n.SnmpMode,
				Community: n.Community,
				User:      n.User,
				Password:  n.Password,
				HPorts:    24,
				Ports:     []datastore.PortEnt{},
			}
			if dent.SnmpConf != nil {
				newNet.SnmpMode = dent.SnmpConf.SnmpMode
				newNet.Community = dent.SnmpConf.Community
				newNet.User = dent.SnmpConf.SnmpUser
				newNet.Password = dent.SnmpConf.SnmpPassword
			}
			ports, err := topology.FetchNetworkPorts(ctx, newNet, conf.Timeout, conf.Retry)
			if err == nil && len(ports) > 0 {
				newNet.Ports = ports
				newNet.Error = ""
			}
			_ = e.store.SaveNetwork(ctx, newNet)
		} else if len(existingNet.Ports) < 1 || existingNet.Error != "" {
			if dent.SnmpConf != nil {
				existingNet.SnmpMode = dent.SnmpConf.SnmpMode
				existingNet.Community = dent.SnmpConf.Community
				existingNet.User = dent.SnmpConf.SnmpUser
				existingNet.Password = dent.SnmpConf.SnmpPassword
			}
			ports, err := topology.FetchNetworkPorts(ctx, existingNet, conf.Timeout, conf.Retry)
			if err == nil && len(ports) > 0 {
				existingNet.Ports = ports
				existingNet.Error = ""
			}
			_ = e.store.SaveNetwork(ctx, existingNet)
		}
	}

	if conf.AddPolling {
		e.addPolling(ctx, dent, n, mapConf)
	}
}

func (e *Engine) addPolling(ctx context.Context, dent *discoverInfoEnt, n *datastore.NodeEnt, mapConf *datastore.MapConfEnt) {
	pollInt := 60
	timeout := 2
	retry := 1
	if mapConf != nil {
		if mapConf.PollInt > 0 {
			pollInt = mapConf.PollInt
		}
		if mapConf.Timeout > 0 {
			timeout = mapConf.Timeout
		}
		if mapConf.Retry > 0 {
			retry = mapConf.Retry
		}
	}

	offset := 5 + rand.Intn(15)
	nextTime := time.Now().UnixNano() + int64(offset)*1e9

	// 1. PING Polling
	pPing := &datastore.PollingEnt{
		NodeID:   n.ID,
		Name:     "PING",
		Type:     "ping",
		Level:    "low",
		State:    "unknown",
		PollInt:  pollInt,
		Timeout:  timeout,
		Retry:    retry,
		NextTime: nextTime,
	}
	_ = e.savePollingUnique(ctx, pPing)

	// 2. SNMP Polling if SNMP is enabled
	if dent.SysObjectID != "" || n.SnmpMode != "" {
		pUptime := &datastore.PollingEnt{
			NodeID:   n.ID,
			Name:     "sysUpTime",
			Type:     "snmp",
			Params:   mib.NameToOID("sysUpTime.0"),
			Level:    "low",
			State:    "unknown",
			PollInt:  pollInt * 5,
			Timeout:  timeout,
			Retry:    retry,
			NextTime: nextTime + int64(time.Second),
		}
		_ = e.savePollingUnique(ctx, pUptime)
	}

	// 3. Port scan pollings
	for s := range dent.ServerList {
		name := ""
		ptype := ""
		params := ""
		switch s {
		case "http":
			name = "HTTP"
			ptype = "http"
			params = fmt.Sprintf("http://%s", n.IP)
		case "https":
			name = "HTTPS"
			ptype = "http"
			params = fmt.Sprintf("https://%s", n.IP)
		case "ssh":
			name = "SSH"
			ptype = "tcp"
			params = "22"
		case "rdp":
			name = "RDP"
			ptype = "tcp"
			params = "3389"
		}
		if name != "" {
			p := &datastore.PollingEnt{
				NodeID:   n.ID,
				Name:     name,
				Type:     ptype,
				Params:   params,
				Level:    "off",
				State:    "unknown",
				PollInt:  pollInt * 5,
				Timeout:  timeout,
				Retry:    retry,
				NextTime: nextTime + int64(2*time.Second),
			}
			_ = e.savePollingUnique(ctx, p)
		}
	}
}

func (e *Engine) savePollingUnique(ctx context.Context, p *datastore.PollingEnt) error {
	pollings, err := e.store.ListPollings(ctx)
	if err == nil {
		for _, ep := range pollings {
			if ep.NodeID == p.NodeID && ep.Type == p.Type && ep.Name == p.Name {
				return nil // already exists
			}
		}
	}
	return e.store.SavePolling(ctx, p)
}
