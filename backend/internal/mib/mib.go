package mib

import (
	"bytes"
	"embed"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/gosnmp/gosnmp"
	"github.com/sleepinggenius2/gosmi/parser"
	gomibdb "github.com/twsnmp/go-mibdb"
	"golang.org/x/text/encoding/japanese"
	"golang.org/x/text/transform"
)

//go:embed conf/mib.txt conf/mibs/* conf/mib2descr_*.txt
var conf embed.FS

// MIBInfo stores metadata parsed from MIB definition files.
type MIBInfo struct {
	OID         string         `json:"oid"`
	Status      string         `json:"status"`
	Type        string         `json:"type"`
	Enum        string         `json:"enum"`
	Defval      string         `json:"defval"`
	Units       string         `json:"units"`
	Index       string         `json:"index"`
	Description string         `json:"description"`
	EnumMap     map[int]string `json:"enumMap,omitempty"`
	Hint        string         `json:"hint"`
}

// MIBTreeEnt represents a hierarchical node in the MIB Tree.
type MIBTreeEnt struct {
	OID      string        `json:"oid"`
	Name     string        `json:"name"`
	MIBInfo  *MIBInfo      `json:"mibInfo,omitempty"`
	Children []*MIBTreeEnt `json:"children,omitempty"`
}

// MIBModuleEnt represents a loaded MIB module entry.
type MIBModuleEnt struct {
	Type  string `json:"type"`
	File  string `json:"file"`
	Name  string `json:"name"`
	Error string `json:"error,omitempty"`
}

// MIBTypeEnt represents a textual convention type definition.
type MIBTypeEnt struct {
	Enum    string
	EnumMap map[int]string
	Hint    string
}

var (
	mu           sync.RWMutex
	MIBDB        *gomibdb.MIBDB
	MIBTree      = []*MIBTreeEnt{}
	MIBInfoMap   = make(map[string]*MIBInfo)
	MIBTypeMap   = make(map[string]MIBTypeEnt)
	MIBModules   = []*MIBModuleEnt{}
	AutoCharCode = true
	initialized  bool
)

const mibFileList = `RFC-1215.txt
RFC1155-SMI.txt
RFC1213-MIB.txt
AGENTX-MIB.txt
BRIDGE-MIB.txt
DISMAN-EVENT-MIB.txt
DISMAN-SCHEDULE-MIB.txt
DISMAN-SCRIPT-MIB.txt
EtherLike-MIB.txt
HCNUM-TC.txt
HOST-RESOURCES-MIB.txt
HOST-RESOURCES-TYPES.txt
IANA-ADDRESS-FAMILY-NUMBERS-MIB.txt
IANA-LANGUAGE-MIB.txt
IANA-RTPROTO-MIB.txt
IANAifType-MIB.txt
IF-INVERTED-STACK-MIB.txt
IF-MIB.txt
INET-ADDRESS-MIB.txt
IP-FORWARD-MIB.txt
IP-MIB.txt
IPV6-FLOW-LABEL-MIB.txt
IPV6-ICMP-MIB.txt
IPV6-MIB.txt
IPV6-TC.txt
IPV6-TCP-MIB.txt
IPV6-UDP-MIB.txt
NET-SNMP-AGENT-MIB.txt
NET-SNMP-EXAMPLES-MIB.txt
NET-SNMP-EXTEND-MIB.txt
NET-SNMP-MIB.txt
NET-SNMP-PASS-MIB.txt
NET-SNMP-TC.txt
NET-SNMP-VACM-MIB.txt
NOTIFICATION-LOG-MIB.txt
RMON-MIB.txt
TOKEN-RING-RMON-MIB.txt
RMON2.txt
HC-RMON-MIB.txt
SCTP-MIB.txt
SMUX-MIB.txt
SNMP-COMMUNITY-MIB.txt
SNMP-FRAMEWORK-MIB.txt
SNMP-MPD-MIB.txt
SNMP-NOTIFICATION-MIB.txt
SNMP-PROXY-MIB.txt
SNMP-TARGET-MIB.txt
SNMP-USER-BASED-SM-MIB.txt
SNMP-USM-AES-MIB.txt
SNMP-USM-DH-OBJECTS-MIB.txt
SNMP-VIEW-BASED-ACM-MIB.txt
SNMPv2-CONF.txt
SNMPv2-MIB.txt
SNMPv2-SMI.txt
SNMPv2-TC.txt
SNMPv2-TM.txt
TCP-MIB.txt
TRANSPORT-ADDRESS-MIB.txt
TUNNEL-MIB.txt
UCD-DEMO-MIB.txt
UCD-DISKIO-MIB.txt
UCD-DLMOD-MIB.txt
UCD-IPFWACC-MIB.txt
UCD-SNMP-MIB.txt
UDP-MIB.txt
ENTITY-MIB.mib
ENTITY-STATE-MIB.mib
IPMCAST-MIB.mib
IPMROUTE-STD-MIB.mib
VRRP-MIB.mib
ATM-MIB.mib
DISMAN-PING-MIB.mib
DISMAN-TRACEROUTE-MIB.mib
OSPF-MIB.mib
OSPFV3-MIB.mib
PTOPO-MIB.mib
RADIUS-ACC-CLIENT-MIB.mib
RADIUS-ACCT-SERVER-MIB.mib
RADIUS-STAT-MIB.mib
SYSAPPL-MIB.mib
LLDP-MIB.mib
P-BRIDGE-MIB.mib
Q-BRIDGE-MIB.mib
U-BRIDGE-MIB.mib
`

// Init initializes the MIB database, loading embedded definitions and extmibs directory.
func Init(dataPath string) error {
	mu.Lock()
	defer mu.Unlock()

	MIBInfoMap = make(map[string]*MIBInfo)
	MIBTypeMap = make(map[string]MIBTypeEnt)
	MIBModules = []*MIBModuleEnt{}
	MIBTree = []*MIBTreeEnt{}

	// 1. Load mib.txt
	if r, err := os.Open(filepath.Join(dataPath, "mib.txt")); err == nil {
		loadMIBDBNameOnly(r)
	} else if r, err := conf.Open("conf/mib.txt"); err == nil {
		loadMIBDBNameOnly(r)
	}

	// 2. Load embedded MIB files
	loadMIBsFromFS()

	// 3. Load user-added extended MIBs from dataPath/extmibs
	extDir := filepath.Join(dataPath, "extmibs")
	_ = os.MkdirAll(extDir, 0755)
	loadExtMIBs(extDir)

	// 4. Resolve MIB info numeric OIDs
	checkMIBInfoMap()

	// 5. Apply localized MIB-2 descriptions
	setMIB2Descr("ja")

	// 6. Build MIB Tree for browser
	makeMibTreeList()

	initialized = true
	slog.Info("MIB subsystem initialized", "modules", len(MIBModules), "infoEntries", len(MIBInfoMap))
	return nil
}

// ReloadExtMIBs scans and reloads extended MIBs from dataPath/extmibs.
func ReloadExtMIBs(dataPath string) {
	mu.Lock()
	defer mu.Unlock()
	extDir := filepath.Join(dataPath, "extmibs")
	loadExtMIBs(extDir)
	checkMIBInfoMap()
	makeMibTreeList()
}

func loadMIBDBNameOnly(f io.Reader) {
	if f == nil {
		return
	}
	if closer, ok := f.(io.Closer); ok {
		defer closer.Close()
	}
	s, err := io.ReadAll(f)
	if err != nil {
		slog.Warn("Failed to read mib.txt", "error", err)
		return
	}
	mdb, err := gomibdb.NewMIBDBFromStr(string(s), "")
	if err != nil {
		slog.Warn("Failed to parse mib.txt", "error", err)
		return
	}
	MIBDB = mdb
}

func loadMIBsFromFS() {
	var skipList []string
	for _, m := range strings.Split(mibFileList, "\n") {
		m = strings.TrimSpace(m)
		if m == "" {
			continue
		}
		path := "conf/mibs/" + m
		r, err := conf.Open(path)
		if err != nil {
			continue
		}
		asn1, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			continue
		}
		if loadExtMIB(asn1, "int", path, false) {
			skipList = append(skipList, path)
		}
	}

	for _, path := range skipList {
		r, err := conf.Open(path)
		if err != nil {
			continue
		}
		asn1, err := io.ReadAll(r)
		_ = r.Close()
		if err != nil {
			continue
		}
		_ = loadExtMIB(asn1, "int", path, true)
	}
}

func loadExtMIBs(root string) {
	if MIBDB == nil {
		return
	}
	skipMap := make(map[string]bool)
	hasHit := false
	cleanedRoot := filepath.Clean(root)

	_ = filepath.Walk(cleanedRoot, func(path string, info os.FileInfo, err error) error {
		if err != nil || info.IsDir() {
			return nil
		}
		cleanedPath := filepath.Clean(path)
		if !strings.HasPrefix(cleanedPath, cleanedRoot) || info.Mode()&os.ModeSymlink != 0 {
			return nil
		}
		if asn1, err := os.ReadFile(cleanedPath); err == nil {
			if loadExtMIB(asn1, "ext", cleanedPath, false) {
				skipMap[cleanedPath] = true
			} else {
				hasHit = true
			}
		}
		return nil
	})

	r := 1
	for hasHit && len(skipMap) > 0 && r < 10 {
		hasHit = false
		for path := range skipMap {
			if asn1, err := os.ReadFile(path); err == nil {
				if !loadExtMIB(asn1, "ext", path, false) {
					delete(skipMap, path)
					hasHit = true
				}
			}
		}
		r++
	}

	for path := range skipMap {
		if asn1, err := os.ReadFile(path); err == nil {
			_ = loadExtMIB(asn1, "ext", path, true)
		}
	}
}

func loadExtMIB(asn1 []byte, fileType, file string, retry bool) bool {
	var nameList []string
	mapNameToOID := make(map[string]string)
	if MIBDB != nil {
		for _, name := range MIBDB.GetNameList() {
			mapNameToOID[name] = MIBDB.NameToOID(name)
		}
	}

	module, err := parser.Parse(bytes.NewReader(asn1))
	if err != nil || module == nil {
		modErr := err
		mod := "Unknown"
		if module != nil {
			mod = string(module.Name)
		}
		asn1 = rfc2mib(asn1)
		module, err = parser.Parse(bytes.NewReader(asn1))
		if err != nil || module == nil {
			errMsg := ""
			if modErr != nil {
				errMsg = modErr.Error()
			}
			if err != nil {
				errMsg += "\t" + err.Error()
			}
			MIBModules = append(MIBModules, &MIBModuleEnt{
				File:  file,
				Type:  fileType,
				Name:  mod,
				Error: errMsg,
			})
			return false
		}
	}

	if module.Body.Identity != nil {
		name := module.Body.Identity.Name.String()
		oid := getOid(&module.Body.Identity.Oid)
		mapNameToOID[name] = oid
		nameList = append(nameList, name)
	}

	if module.Body.Types != nil {
		for _, t := range module.Body.Types {
			if t.TextualConvention != nil {
				if t.TextualConvention.Syntax.Enum != nil {
					var enum []string
					enumMap := make(map[int]string)
					for _, e := range t.TextualConvention.Syntax.Enum {
						enum = append(enum, fmt.Sprintf("%s:%s ", e.Value, e.Name))
						if i, err := strconv.Atoi(e.Value); err == nil {
							enumMap[i] = string(e.Name)
						}
					}
					MIBTypeMap[t.Name.String()] = MIBTypeEnt{
						Hint:    t.TextualConvention.DisplayHint,
						Enum:    strings.Join(enum, ","),
						EnumMap: enumMap,
					}
				} else if t.TextualConvention.DisplayHint != "" {
					MIBTypeMap[t.Name.String()] = MIBTypeEnt{
						Hint: t.TextualConvention.DisplayHint,
					}
				}
			}
		}
	}

	for _, n := range module.Body.Nodes {
		if n.Name.String() == "" || n.Oid == nil {
			continue
		}
		name := n.Name.String()
		mapNameToOID[name] = getOid(n.Oid)
		nameList = append(nameList, name)
		setMIBInfo(mapNameToOID[name], &n)
	}

	for _, name := range nameList {
		oid, ok := mapNameToOID[name]
		if !ok {
			continue
		}
		a := strings.SplitN(oid, ".", 2)
		if len(a) < 2 {
			continue
		}
		noid, ok := mapNameToOID[a[0]]
		if !ok {
			continue
		}
		mapNameToOID[name] = noid + "." + a[1]
	}

	hasSkip := false
	noParent := ""
	oidReg := regexp.MustCompile(`^[.0-9]+$`)
	for _, name := range nameList {
		oid := mapNameToOID[name]
		if !oidReg.MatchString(oid) && MIBDB != nil {
			noid := MIBDB.NameToOID(oid)
			if noid == ".0.0" {
				hasSkip = true
				if retry && noParent == "" {
					noParent = fmt.Sprintf("no parent name=%s,oid=%s", name, oid)
				}
				continue
			}
			oid = noid
		}
		if MIBDB != nil {
			_ = MIBDB.Add(name, oid)
		}
	}

	if !hasSkip {
		MIBModules = append(MIBModules, &MIBModuleEnt{
			File: file,
			Type: fileType,
			Name: string(module.Name),
		})
	} else if retry {
		MIBModules = append(MIBModules, &MIBModuleEnt{
			File:  file,
			Type:  fileType,
			Name:  string(module.Name),
			Error: noParent,
		})
	}
	return hasSkip
}

func getOid(oid *parser.Oid) string {
	ret := ""
	for _, o := range oid.SubIdentifiers {
		if o.Name != nil {
			ret += o.Name.String()
		}
		if o.Number != nil {
			ret += fmt.Sprintf(".%d", int(*o.Number))
		}
	}
	return ret
}

func setMIBInfo(oid string, n *parser.Node) {
	if n == nil {
		return
	}
	if n.NotificationType != nil {
		MIBInfoMap[oid] = &MIBInfo{
			OID:         oid,
			Status:      n.NotificationType.Status.ToSmi().String(),
			Type:        "Notification",
			Description: n.NotificationType.Description,
		}
		return
	}
	if n.TrapType != nil {
		MIBInfoMap[oid] = &MIBInfo{
			OID:         oid,
			Status:      "current",
			Type:        "Notification",
			Description: n.TrapType.Description,
		}
		return
	}
	if n.ObjectType == nil {
		return
	}
	if n.ObjectType.Syntax.Sequence != nil {
		MIBInfoMap[oid] = &MIBInfo{
			OID:         oid,
			Status:      "current",
			Type:        "Sequence",
			Description: n.ObjectType.Description,
		}
		return
	}
	if n.ObjectType.Syntax.Type == nil {
		return
	}
	var enum []string
	enumMap := make(map[int]string)
	for _, e := range n.ObjectType.Syntax.Type.Enum {
		enum = append(enum, fmt.Sprintf("%s:%s ", e.Value, e.Name))
		if i, err := strconv.Atoi(e.Value); err == nil {
			enumMap[i] = string(e.Name)
		}
	}
	defval := ""
	if n.ObjectType.Defval != nil {
		defval = *n.ObjectType.Defval
	}
	var index []string
	for _, i := range n.ObjectType.Index {
		index = append(index, i.Name.String())
	}

	MIBInfoMap[oid] = &MIBInfo{
		OID:         oid,
		Status:      n.ObjectType.Status.ToSmi().String(),
		Type:        n.ObjectType.Syntax.Type.Name.String(),
		Enum:        strings.Join(enum, ","),
		Defval:      defval,
		Units:       n.ObjectType.Units,
		Index:       strings.Join(index, ","),
		Description: n.ObjectType.Description,
		EnumMap:     enumMap,
	}
}

func checkMIBInfoMap() {
	if MIBDB == nil {
		return
	}
	var delList []string
	var addList []*MIBInfo
	for oid, info := range MIBInfoMap {
		noid := MIBDB.NameToOID(oid)
		if noid != oid {
			delList = append(delList, oid)
			info.OID = noid
			addList = append(addList, info)
		}
		if e, ok := MIBTypeMap[info.Type]; ok {
			if info.Enum == "" {
				info.Enum = e.Enum
				info.EnumMap = e.EnumMap
			}
			if info.Hint == "" {
				info.Hint = e.Hint
			}
		}
	}
	for _, d := range delList {
		delete(MIBInfoMap, d)
	}
	for _, a := range addList {
		MIBInfoMap[a.OID] = a
	}
}

func setMIB2Descr(lang string) {
	if lang != "ja" {
		lang = "en"
	}
	r, err := conf.Open("conf/mib2descr_" + lang + ".txt")
	if err != nil {
		return
	}
	defer r.Close()

	rg := regexp.MustCompile(`^#(\S+)`)
	all, err := io.ReadAll(r)
	if err != nil {
		return
	}
	name := ""
	var descr []string
	for _, l := range strings.Split(string(all), "\n") {
		m := rg.FindStringSubmatch(l)
		if len(m) > 1 {
			if name != "" && len(descr) > 0 {
				replaceMIBDescr(name, descr)
			}
			name = m[1]
			descr = []string{}
		} else {
			l = strings.ReplaceAll(l, `"`, "")
			descr = append(descr, l)
		}
	}
	if name != "" && len(descr) > 0 {
		replaceMIBDescr(name, descr)
	}
}

func replaceMIBDescr(name string, descr []string) {
	if MIBDB == nil {
		return
	}
	oid := MIBDB.NameToOID(name)
	if e, ok := MIBInfoMap[oid]; ok {
		e.Description = strings.Join(descr, "\n")
		MIBInfoMap[oid] = e
	}
}

var (
	mibTreeMAP  = map[string]*MIBTreeEnt{}
	mibTreeRoot *MIBTreeEnt
)

func addToMibTree(oid, name, poid string) {
	n := &MIBTreeEnt{Name: name, OID: oid, Children: []*MIBTreeEnt{}}
	if i, ok := MIBInfoMap[oid]; ok {
		n.MIBInfo = i
	}
	if poid == "" {
		mibTreeRoot = n
	} else {
		p, ok := mibTreeMAP[poid]
		if !ok {
			return
		}
		p.Children = append(p.Children, n)
	}
	mibTreeMAP[oid] = n
}

func makeMibTreeList() {
	if MIBDB == nil {
		return
	}
	mibTreeMAP = map[string]*MIBTreeEnt{}
	mibTreeRoot = nil
	MIBTree = []*MIBTreeEnt{}

	var oids []string
	for _, n := range MIBDB.GetNameList() {
		oid := MIBDB.NameToOID(n)
		oids = append(oids, oid)
	}
	sort.Slice(oids, func(i, j int) bool {
		a := strings.Split(oids[i], ".")
		b := strings.Split(oids[j], ".")
		for k := 0; k < len(a) && k < len(b); k++ {
			l, _ := strconv.Atoi(a[k])
			m, _ := strconv.Atoi(b[k])
			if l == m {
				continue
			}
			return l < m
		}
		return len(a) < len(b)
	})
	addToMibTree(".1", "iso", "")
	for _, oid := range oids {
		name := MIBDB.OIDToName(oid)
		if name == "" {
			continue
		}
		lastDot := strings.LastIndex(oid, ".")
		if lastDot < 0 {
			continue
		}
		poid := oid[:lastDot]
		addToMibTree(oid, name, poid)
	}
	if mibTreeRoot != nil {
		MIBTree = append(MIBTree, mibTreeRoot.Children...)
	}
}

func rfc2mib(b []byte) []byte {
	rp := strings.NewReplacer("\r", "")
	all := rp.Replace(string(b))

	regPageBreak := regexp.MustCompile(`[^\n]*\n+\f\n+[^\n]*`)
	all = regPageBreak.ReplaceAllString(all, "\n\n")

	regOver3nl := regexp.MustCompile(`\n{3,}`)
	all = regOver3nl.ReplaceAllString(all, "\n\n")

	regMODULEStart := regexp.MustCompile(`\s*([A-Z]+[-A-Za-z0-9]+)+\s+DEFINITIONS\s+\w*\s*::=\s+BEGIN\s*`)
	regMACROStart := regexp.MustCompile(`\s*([A-Z]+[-A-Za-z0-9]+)+\s+MACRO\s+::=\s+BEGIN\s*`)
	regEnd := regexp.MustCompile(`\s*END\s*`)
	regComment := regexp.MustCompile(`\s*(--\s+.*)$`)
	lines := strings.Split(all, "\n")
	depth := 0
	quoted := false
	var mibLines []string
	for _, l := range lines {
		if depth == 0 {
			if regMODULEStart.MatchString(l) {
				mibLines = append(mibLines, l)
				depth = 1
			}
			continue
		}
		mibLines = append(mibLines, l)
		if quoted {
			a := strings.Split(l, `"`)
			if len(a) == 1 {
				continue
			}
			if len(a)%2 == 0 {
				quoted = false
			}
			continue
		} else {
			a := strings.Split(l, `"`)
			if len(a) == 2 {
				quoted = true
				continue
			}
			if len(a) != 1 {
				continue
			}
		}
		l = regComment.ReplaceAllString(l, "")
		if regMACROStart.MatchString(l) {
			depth++
			continue
		}
		if regEnd.MatchString(l) {
			depth--
			if depth == 0 {
				return []byte(strings.Join(mibLines, "\n") + "\n")
			}
		}
	}
	return []byte("")
}

// OIDToName resolves numeric OID to MIB object name, including index suffix.
func OIDToName(oid string) string {
	mu.RLock()
	defer mu.RUnlock()
	if MIBDB == nil {
		return strings.TrimPrefix(oid, ".")
	}
	norm := oid
	if !strings.HasPrefix(norm, ".") {
		norm = "." + norm
	}
	name := MIBDB.OIDToName(norm)
	if name != "" && name != norm {
		return name
	}
	// Fallback right-to-left suffix peeling
	parts := strings.Split(strings.TrimPrefix(norm, "."), ".")
	for i := len(parts) - 1; i >= 1; i-- {
		prefix := "." + strings.Join(parts[:i], ".")
		subName := MIBDB.OIDToName(prefix)
		if subName != "" && subName != prefix {
			suffix := strings.Join(parts[i:], ".")
			return subName + "." + suffix
		}
	}
	return strings.TrimPrefix(oid, ".")
}

// NameToOID converts a MIB object name to numeric OID with leading dot.
func NameToOID(name string) string {
	mu.RLock()
	defer mu.RUnlock()
	if MIBDB == nil {
		if strings.HasPrefix(name, ".") {
			return name
		}
		return "." + name
	}
	clean := strings.TrimPrefix(name, ".")
	a := strings.Split(clean, ".")
	if len(a) > 1 {
		baseOID := MIBDB.NameToOID(a[0])
		if baseOID != ".0.0" && baseOID != "" {
			return baseOID + "." + strings.Join(a[1:], ".")
		}
	}
	oid := MIBDB.NameToOID(clean)
	if oid != ".0.0" && oid != "" {
		return oid
	}
	if strings.HasPrefix(name, ".") {
		return name
	}
	return "." + name
}

// FindMIBInfo looks up MIB metadata by object name or OID.
func FindMIBInfo(name string) *MIBInfo {
	mu.RLock()
	defer mu.RUnlock()
	a := strings.SplitN(name, ".", 2)
	cleanName := a[0]
	oid := cleanName
	if MIBDB != nil {
		oid = MIBDB.NameToOID(cleanName)
	}
	if i, ok := MIBInfoMap[oid]; ok {
		return i
	}
	if i, ok := MIBInfoMap["."+oid]; ok {
		return i
	}
	return nil
}

// GetMIBTree returns the loaded hierarchical MIB tree.
func GetMIBTree() []*MIBTreeEnt {
	mu.RLock()
	defer mu.RUnlock()
	return MIBTree
}

// GetMIBModules returns all loaded MIB modules.
func GetMIBModules() []*MIBModuleEnt {
	mu.RLock()
	defer mu.RUnlock()
	return MIBModules
}

// FormatMIBValue formats an SNMP PDU value based on MIB type definitions.
func GetMIBValueString(name string, variable *gosnmp.SnmpPDU, raw bool) string {
	if variable == nil {
		return ""
	}
	mu.RLock()
	defer mu.RUnlock()

	value := ""
	switch variable.Type {
	case gosnmp.OctetString:
		mi := findMIBInfoUnlocked(name)
		if mi != nil {
			switch mi.Type {
			case "PhysAddress", "OctetString", "MacAddress":
				a, ok := variable.Value.([]uint8)
				if !ok {
					a = []uint8(PrintMIBStringVal(variable.Value))
				}
				if len(a) == 6 {
					var mac []string
					for _, m := range a {
						mac = append(mac, fmt.Sprintf("%02X", m&0x00ff))
					}
					value = strings.Join(mac, ":")
				} else {
					value = PrintMIBStringVal(variable.Value)
				}
			case "PtopoChassisId", "PtopoGenAddr", "LldpChassisId", "LldpPortId":
				value = printLLDPID(variable.Value)
			case "LldpManAddress":
				value = PrintIPAddress(variable.Value)
			case "DateAndTime":
				value = PrintDateAndTime(variable.Value)
			default:
				value = PrintMIBStringVal(variable.Value)
			}
		} else {
			a, ok := variable.Value.([]uint8)
			if ok && len(a) == 6 {
				var mac []string
				for _, m := range a {
					mac = append(mac, fmt.Sprintf("%02X", m&0x00ff))
				}
				value = strings.Join(mac, ":")
			} else {
				value = PrintMIBStringVal(variable.Value)
			}
		}
	case gosnmp.ObjectIdentifier:
		rawOid := PrintMIBStringVal(variable.Value)
		if MIBDB != nil {
			value = MIBDB.OIDToName(rawOid)
			if value == "" || value == rawOid {
				value = OIDToNameUnlocked(rawOid)
			}
		} else {
			value = rawOid
		}
	case gosnmp.TimeTicks:
		t := gosnmp.ToBigInt(variable.Value).Uint64()
		if raw {
			value = fmt.Sprintf("%d", t)
		} else {
			if t > (24 * 3600 * 100) {
				d := t / (24 * 3600 * 100)
				r := t - d*(24*3600*100)
				value = fmt.Sprintf("%d(%d days, %v)", t, d, time.Duration(r*10*uint64(time.Millisecond)))
			} else {
				value = fmt.Sprintf("%d(%v)", t, time.Duration(t*10*uint64(time.Millisecond)))
			}
		}
	case gosnmp.IPAddress:
		value = PrintIPAddress(variable.Value)
	default:
		if variable.Type == gosnmp.Integer {
			value = fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64())
		} else {
			value = fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Uint64())
		}
		if !raw {
			mi := findMIBInfoUnlocked(name)
			if mi != nil {
				v := int(gosnmp.ToBigInt(variable.Value).Int64())
				if mi.Enum != "" && mi.EnumMap != nil {
					if vn, ok := mi.EnumMap[v]; ok {
						value += "(" + vn + ")"
					}
				} else if mi.Hint != "" {
					value = PrintHintedMIBIntVal(int32(v), mi.Hint, variable.Type != gosnmp.Integer)
				}
				if mi.Units != "" {
					value += " " + mi.Units
				}
			} else {
				// Built-in standard enums fallback (ifAdminStatus, ifOperStatus, etc.)
				v := int(gosnmp.ToBigInt(variable.Value).Int64())
				baseName := name
				if dot := strings.Index(baseName, "."); dot >= 0 {
					baseName = baseName[:dot]
				}
				switch baseName {
				case "ifAdminStatus", "ifOperStatus":
					switch v {
					case 1:
						value += "(up)"
					case 2:
						value += "(down)"
					case 3:
						value += "(testing)"
					case 4:
						value += "(unknown)"
					case 5:
						value += "(dormant)"
					case 6:
						value += "(notPresent)"
					case 7:
						value += "(lowerLayerDown)"
					}
				}
			}
		}
	}
	return value
}

func findMIBInfoUnlocked(name string) *MIBInfo {
	a := strings.SplitN(name, ".", 2)
	cleanName := a[0]
	oid := cleanName
	if MIBDB != nil {
		oid = MIBDB.NameToOID(cleanName)
	}
	if i, ok := MIBInfoMap[oid]; ok {
		return i
	}
	if i, ok := MIBInfoMap["."+oid]; ok {
		return i
	}
	return nil
}

func OIDToNameUnlocked(oid string) string {
	if MIBDB == nil {
		return strings.TrimPrefix(oid, ".")
	}
	norm := oid
	if !strings.HasPrefix(norm, ".") {
		norm = "." + norm
	}
	name := MIBDB.OIDToName(norm)
	if name != "" && name != norm {
		return name
	}
	parts := strings.Split(strings.TrimPrefix(norm, "."), ".")
	for i := len(parts) - 1; i >= 1; i-- {
		prefix := "." + strings.Join(parts[:i], ".")
		subName := MIBDB.OIDToName(prefix)
		if subName != "" && subName != prefix {
			suffix := strings.Join(parts[i:], ".")
			return subName + "." + suffix
		}
	}
	return strings.TrimPrefix(oid, ".")
}

// DecodeTrap decodes an SNMP Trap packet into TrapType, formatted Variables string, and FromAddress.
func DecodeTrap(packet *gosnmp.SnmpPacket, srcIP string, nodeName string) (trapType string, variables string, fromAddress string) {
	if packet == nil {
		return "", "", srcIP
	}

	fromAddress = srcIP
	if nodeName != "" {
		fromAddress = fmt.Sprintf("%s(%s)", srcIP, nodeName)
	}

	var sb strings.Builder
	var trapOidVal string

	for _, vb := range packet.Variables {
		key := OIDToName(vb.Name)
		val := GetMIBValueString(key, &vb, false)
		if sb.Len() > 0 {
			sb.WriteString(" ")
		}
		sb.WriteString(fmt.Sprintf("%s=%s", key, val))

		if key == "snmpTrapOID.0" {
			trapOidVal = val
		}
		if strings.HasPrefix(key, "sysName") && nodeName == "" {
			fromAddress = fmt.Sprintf("%s(%s)", srcIP, val)
		}
	}

	// TrapType determination matching twsnmpfk
	isV1 := packet.Version == gosnmp.Version1 || packet.Enterprise != ""
	if isV1 {
		switch packet.GenericTrap {
		case 0:
			trapType = "coldStart(v1)"
		case 1:
			trapType = "warmStart(v1)"
		case 2:
			trapType = "linkDown(v1)"
		case 3:
			trapType = "linkUp(v1)"
		case 4:
			trapType = "authenticationFailure(v1)"
		case 5:
			trapType = "egpNeighborLoss(v1)"
		default:
			trapType = fmt.Sprintf("enterpriseSpecific(%d)", packet.SpecificTrap)
		}
		ent := OIDToName(packet.Enterprise)
		if ent != "" {
			if sb.Len() > 0 {
				sb.WriteString(" ")
			}
			sb.WriteString("Enterprise=" + ent)
		}
	} else {
		// SNMPv2c / SNMPv3
		if trapOidVal != "" {
			trapType = trapOidVal
		} else {
			// Try regex match from variables string
			re := regexp.MustCompile(`snmpTrapOID\.0=(\S+)`)
			matches := re.FindStringSubmatch(sb.String())
			if len(matches) > 1 {
				trapType = matches[1]
			}
		}
	}

	variables = sb.String()
	return trapType, variables, fromAddress
}

// PrintHintedMIBIntVal formats integer based on display hint.
func PrintHintedMIBIntVal(val int32, hint string, us bool) string {
	if hint == "" {
		if us {
			return fmt.Sprintf("%d", uint32(val))
		}
		return fmt.Sprintf("%d", val)
	}
	h := hint[0:1]
	switch h {
	case "d":
		r := ""
		n := false
		if us {
			r = fmt.Sprintf("%d", uint32(val))
		} else {
			if val < 0 {
				n = true
				val = -val
			}
			r = fmt.Sprintf("%d", val)
		}
		if len(hint) > 2 && hint[1:2] == "-" {
			s, err := strconv.Atoi(hint[2:])
			if err == nil && s != 0 {
				if s < len(r) {
					pos := len(r) - s
					r = r[0:pos] + "." + r[pos:]
				} else {
					tmp := "0."
					for i := 0; i < s-len(r); i++ {
						tmp += "0"
					}
					r = tmp + r
				}
			}
		}
		if n {
			r = "-" + r
		}
		return r
	case "x":
		return fmt.Sprintf("%x", val)
	case "o":
		return fmt.Sprintf("%o", val)
	case "b":
		r := ""
		for b := 0x80000000; b != 0; b >>= 1 {
			if int32(b)&val != 0 {
				r += "1"
			} else {
				r += "0"
			}
		}
		return r
	}
	return ""
}

// PrintIPAddress formats IP address value.
func PrintIPAddress(i interface{}) string {
	switch v := i.(type) {
	case string:
		return v
	case []uint8:
		if len(v) == 16 {
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
				v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10], v[11], v[12], v[13], v[14], v[15])
		} else if len(v) == 4 {
			return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
		}
	case int, int64, uint, uint64:
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("Invalid IP Address %v", i)
}

// PrintMIBStringVal formats string value with auto charset checking.
func PrintMIBStringVal(i interface{}) string {
	r := ""
	switch v := i.(type) {
	case string:
		r = v
	case []uint8:
		r = string(v)
	case int, int64, uint, uint64:
		return fmt.Sprintf("%d", v)
	}
	if AutoCharCode {
		r = CheckCharCode(r)
	}
	return r
}

func printLLDPID(i interface{}) string {
	r := ""
	switch v := i.(type) {
	case string:
		r = v
	case []uint8:
		if len(v) == 6 {
			return fmt.Sprintf("%02X:%02X:%02X:%02X:%02X:%02X", v[0], v[1], v[2], v[3], v[4], v[5])
		} else if len(v) == 4 {
			return fmt.Sprintf("%d.%d.%d.%d", v[0], v[1], v[2], v[3])
		} else if len(v) == 16 {
			return fmt.Sprintf("%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x:%02x",
				v[0], v[1], v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10], v[11], v[12], v[13], v[14], v[15])
		}
		r = string(v)
	case int, int64, uint, uint64:
		return fmt.Sprintf("%d", v)
	default:
		return fmt.Sprintf("%v", i)
	}
	for _, a := range r {
		if !unicode.IsPrint(a) {
			return fmt.Sprintf("%x", r)
		}
	}
	return r
}

// PrintDateAndTime formats DateAndTime textual convention.
func PrintDateAndTime(i interface{}) string {
	switch v := i.(type) {
	case string:
		return v
	case []uint8:
		if len(v) == 11 {
			return fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d.%02d%c%02d%02d",
				(int(v[0])*256 + int(v[1])), v[2], v[3], v[4], v[5], v[6], v[7], v[8], v[9], v[10])
		} else if len(v) == 8 {
			return fmt.Sprintf("%04d/%02d/%02d %02d:%02d:%02d.%02d",
				(int(v[0])*256 + int(v[1])), v[2], v[3], v[4], v[5], v[6], v[7])
		}
	case int, int64, uint, uint64:
		return fmt.Sprintf("%d", v)
	}
	return fmt.Sprintf("Invalid Date And Time %v", i)
}

// CheckCharCode converts Shift-JIS to UTF-8 if detected.
func CheckCharCode(s string) string {
	if !AutoCharCode {
		return s
	}
	if isSjis([]byte(s)) {
		dec := japanese.ShiftJIS.NewDecoder()
		if b, _, err := transform.Bytes(dec, []byte(s)); err == nil {
			return string(b)
		}
	}
	return s
}

func isSjis(p []byte) bool {
	f := false
	for _, c := range p {
		if f {
			if c < 0x0040 || c > 0x00fc {
				return false
			}
			f = false
			continue
		}
		if c < 0x007f {
			continue
		}
		if (c >= 0x0081 && c <= 0x9f) || (c >= 0x00e0 && c <= 0x00ef) {
			f = true
		} else {
			return false
		}
	}
	return true
}
