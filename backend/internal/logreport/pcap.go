package logreport

import (
	"fmt"
	"strings"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// twpcap record types handled: EtherType, DNS, RADIUS, TLSFlow.
// IPToMAC, DHCP and NTP are not handled yet because they feed the
// device / IP / server reports, which are built by other means in this project.
func (s *Session) processPcap(r Record, t string, m map[string]string) bool {
	switch t {
	case "EtherType":
		return s.pcapEtherType(r, m)
	case "DNS":
		return s.pcapDNS(r, m)
	case "RADIUS":
		return s.pcapRADIUS(r, m)
	case "TLSFlow":
		return s.pcapTLSFlow(r, m)
	}
	return false
}

var etherTypeMap = map[string]string{
	"0x0000": "LLC",
	"0x0800": "IPv4",
	"0x0806": "ARP",
	"0x0842": "WakeOnLAN",
	"0x8035": "RARP",
	"0x86dd": "IPv6",
	"0x8899": "RRCP/Buffalo Loop Detect",
	"0x88cc": "LLDP",
	"0x8100": "VLAN",
	"0x9100": "VLAN DT",
	"0x8847": "MPLS Unicat",
	"0x8848": "MPLS Multicast",
	"0x8863": "PPPoE Discovery",
	"0x8864": "PPPoE Session",
	"0x888e": "802.1X",
	"0x88a2": "ATAoE",
	"0x9000": "Ethernet Conf Test",
	"0x87d2": "Aironet DDP",
	"0x890d": "802.11MP",
	"0x2000": "Cisco Discovery",
	"0x01a2": "Nortel Discovery",
	"0x6558": "Transparent Ethernet Bridging",
	"0x880b": "PPP",
	"0x88be": "ERSPAN",
	"0x88a8": "QinQ",
}

func etherTypeName(t string) string {
	if n, ok := etherTypeMap[t]; ok {
		return n
	}
	return fmt.Sprintf("Other(%s)", t)
}

// type=EtherType,0x0800=123,0x0806=4,...
func (s *Session) pcapEtherType(r Record, m map[string]string) bool {
	hit := false
	for k, v := range m {
		if !strings.HasPrefix(k, "0x") {
			continue
		}
		c := atoi(v)
		id := r.Host + ":" + k
		if e := getEnt[EtherTypeEnt](s, KindEtherType, id); e != nil {
			e.Count += c
			e.LastTime = r.Time
			putEnt(s, KindEtherType, id, e)
		} else {
			putEnt(s, KindEtherType, id, &EtherTypeEnt{
				ID:        id,
				Host:      r.Host,
				Type:      k,
				Name:      etherTypeName(k),
				Count:     c,
				FirstTime: r.Time,
				LastTime:  r.Time,
			})
		}
		hit = true
	}
	return hit
}

// type=DNS,DNSType=%s,Name=%s,sv=%s,count=%d,change=%d,lcl=%s,lMAC=%s,ft=%s,lt=%s
func (s *Session) pcapDNS(r Record, m map[string]string) bool {
	t, n, sv := m["DNSType"], m["Name"], m["sv"]
	if t == "" || n == "" || sv == "" {
		return false
	}
	id := makeID(r.Host + ":" + sv + ":" + t + ":" + n)
	lt := parseTime(m["lt"], r.Time)
	e := getEnt[DNSQEnt](s, KindDNSQ, id)
	if e == nil {
		e = &DNSQEnt{ID: id, Host: r.Host, Type: t, Server: sv, Name: n}
	}
	// twpcap reports cumulative counts per reporting period of its own.
	e.Count = atoi(m["count"])
	e.Change = atoi(m["change"])
	e.LastClient = m["lcl"]
	e.LastMAC = m["lMAC"]
	e.LastTime = lt
	e.FirstTime = parseTime(m["ft"], lt)
	putEnt(s, KindDNSQ, id, e)
	return true
}

// type=RADIUS,cl=%s,sv=%s,count=%d,req=%d,accept=%d,reject=%d,challenge=%d,ft=%s,lt=%s
func (s *Session) pcapRADIUS(r Record, m map[string]string) bool {
	sv, cl := m["sv"], m["cl"]
	if sv == "" || cl == "" {
		return false
	}
	id := cl + ":" + sv
	lt := parseTime(m["lt"], r.Time)
	e := getEnt[RADIUSFlowEnt](s, KindRADIUSFlow, id)
	if e == nil {
		e = &RADIUSFlowEnt{ID: id, Client: cl, Server: sv}
	}
	e.Accept = atoi(m["accept"])
	e.Reject = atoi(m["reject"])
	e.Request = atoi(m["req"])
	e.Challenge = atoi(m["challenge"])
	e.Count = atoi(m["count"])
	e.LastTime = lt
	e.FirstTime = parseTime(m["ft"], lt)
	e.UpdateTime = r.Time
	ci, si := s.lookupNode(cl), s.lookupNode(sv)
	e.ClientName, e.ClientNodeID = ci.Name, ci.ID
	e.ServerName, e.ServerNodeID = si.Name, si.ID
	e.Penalty = 0
	if e.Reject > 0 {
		e.Penalty++
		if e.Accept < e.Reject {
			e.Penalty++
		}
	}
	// Server name could not be resolved.
	if e.ServerName == e.Server {
		e.Penalty++
	}
	putEnt(s, KindRADIUSFlow, id, e)
	return true
}

// type=TLSFlow,cl=%s,sv=%s,serv=%s,count=%d,maxver=%s,cipher=%s,ft=%s,lt=%s
func (s *Session) pcapTLSFlow(r Record, m map[string]string) bool {
	sv, cl, service := m["sv"], m["cl"], m["serv"]
	if sv == "" || cl == "" || service == "" {
		return false
	}
	id := cl + ":" + sv + ":" + service
	lt := parseTime(m["lt"], r.Time)
	e := getEnt[TLSFlowEnt](s, KindTLSFlow, id)
	if e == nil {
		e = &TLSFlowEnt{ID: id, Client: cl, Server: sv, Service: service}
	}
	if e.ServerLoc == "" {
		e.ServerLoc = datastore.GetLoc(sv)
	}
	if e.ClientLoc == "" {
		e.ClientLoc = datastore.GetLoc(cl)
	}
	e.Cipher = m["cipher"]
	e.Version = m["maxver"]
	e.Count = atoi(m["count"])
	e.LastTime = lt
	e.FirstTime = parseTime(m["ft"], lt)
	e.UpdateTime = r.Time
	ci, si := s.lookupNode(cl), s.lookupNode(sv)
	e.ClientName, e.ClientNodeID = ci.Name, ci.ID
	e.ServerName, e.ServerNodeID = si.Name, si.ID
	e.Penalty = tlsPenalty(e)
	putEnt(s, KindTLSFlow, id, e)
	return true
}

func tlsPenalty(f *TLSFlowEnt) int {
	p := 0
	switch {
	case strings.Contains(f.Version, "1.2"):
		p++
	case strings.Contains(f.Version, "1.1"):
		p += 2
	case strings.Contains(f.Version, "1.0"):
		p += 3
	case strings.Contains(f.Version, "SSL"):
		p += 4
	}
	// Names that could not be resolved.
	if f.ServerName == f.Server {
		p++
	}
	if f.ClientName == f.Client {
		p++
	}
	return p
}
