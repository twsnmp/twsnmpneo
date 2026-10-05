// Package logreport builds the "report" datasets (Wi-Fi APs, Bluetooth devices,
// packet-capture summaries and Windows event summaries) from protocol records
// emitted by twWifiScan, twBlueScan, twpcap, twwinlog, etc.
//
// Records are processed in real time when received by protocol servers (matching
// TWSNMP FC behavior) through the Engine / Reporter pipeline.
package logreport

import "strings"

// Polling types / sources handled by this package.
const (
	SourceWifiScan = "twwifiscan"
	SourceBlueScan = "twbluescan"
	SourcePcap     = "twpcap"
	SourceWinLog   = "twwinlog"
)

// Report kinds (storage buckets / API names).
const (
	KindWifiAP       = "wifiAP"
	KindBlueDevice   = "blueDevice"
	KindEnvMonitor   = "envMonitor"
	KindPowerMonitor = "powerMonitor"
	KindMotionSensor = "motionSensor"
	KindEtherType    = "etherType"
	KindDNSQ         = "dnsq"
	KindRADIUSFlow   = "radiusFlow"
	KindTLSFlow      = "tlsFlow"
	KindWinEventID   = "winEventID"
	KindWinLogon     = "winLogon"
	KindWinAccount   = "winAccount"
	KindWinKerberos  = "winKerberos"
	KindWinPrivilege = "winPrivilege"
	KindWinProcess   = "winProcess"
	KindWinTask      = "winTask"
)

// SyslogTag returns the syslog tag emitted by the source.
func SyslogTag(source string) string {
	switch strings.ToLower(source) {
	case SourceWifiScan:
		return "twWifiScan"
	case SourceBlueScan:
		return "twBlueScan"
	case SourcePcap:
		return "twpcap"
	case SourceWinLog:
		return "twwinlog"
	}
	return ""
}

// IsSource reports whether s is a known source / polling type.
func IsSource(s string) bool { return SyslogTag(s) != "" }

// KindsOf returns the report kinds fed by the source.
func KindsOf(source string) []string {
	switch strings.ToLower(source) {
	case SourceWifiScan:
		return []string{KindWifiAP}
	case SourceBlueScan:
		return []string{KindBlueDevice, KindEnvMonitor, KindPowerMonitor, KindMotionSensor}
	case SourcePcap:
		return []string{KindEtherType, KindDNSQ, KindRADIUSFlow, KindTLSFlow}
	case SourceWinLog:
		return []string{KindWinEventID, KindWinLogon, KindWinAccount, KindWinKerberos, KindWinPrivilege, KindWinProcess, KindWinTask}
	}
	return nil
}

// AllKinds lists every report kind.
func AllKinds() []string {
	var ret []string
	for _, s := range []string{SourceWifiScan, SourceBlueScan, SourcePcap, SourceWinLog} {
		ret = append(ret, KindsOf(s)...)
	}
	return ret
}

// IsKind reports whether k is a known report kind.
func IsKind(k string) bool {
	for _, v := range AllKinds() {
		if v == k {
			return true
		}
	}
	return false
}

// ScoreInfo is embedded by entities that carry a penalty and a deviation score.
type ScoreInfo struct {
	Penalty    int     `json:"Penalty"`
	Score      float64 `json:"Score"`
	ValidScore bool    `json:"ValidScore"`
}

func (x *ScoreInfo) scoreInfo() *ScoreInfo { return x }

// RSSIEnt is one RSSI sample.
type RSSIEnt struct {
	Value int   `json:"Value"`
	Time  int64 `json:"Time"`
}

// WifiAPEnt is a Wi-Fi access point seen by twWifiScan.
type WifiAPEnt struct {
	ID        string    `json:"ID"`
	Host      string    `json:"Host"`
	BSSID     string    `json:"BSSID"`
	SSID      string    `json:"SSID"`
	Channel   string    `json:"Channel"`
	Info      string    `json:"Info"`
	Vendor    string    `json:"Vendor"`
	Count     int       `json:"Count"`
	Change    int       `json:"Change"`
	RSSI      []RSSIEnt `json:"RSSI"`
	FirstTime int64     `json:"FirstTime"`
	LastTime  int64     `json:"LastTime"`
}

// BlueDeviceEnt is a Bluetooth device seen by twBlueScan.
type BlueDeviceEnt struct {
	ID          string    `json:"ID"`
	Host        string    `json:"Host"`
	Address     string    `json:"Address"`
	AddressType string    `json:"AddressType"`
	Name        string    `json:"Name"`
	Vendor      string    `json:"Vendor"`
	Info        string    `json:"Info"`
	Count       int       `json:"Count"`
	RSSI        []RSSIEnt `json:"RSSI"`
	FirstTime   int64     `json:"FirstTime"`
	LastTime    int64     `json:"LastTime"`
}

// EnvDataEnt is one environment sensor sample.
type EnvDataEnt struct {
	Time               int64   `json:"Time"`
	RSSI               int     `json:"RSSI"`
	Temp               float64 `json:"Temp"`
	Humidity           float64 `json:"Humidity"`
	Illuminance        float64 `json:"Illuminance"`
	BarometricPressure float64 `json:"BarometricPressure"`
	Sound              float64 `json:"Sound"`
	ETVOC              float64 `json:"ETVOC"`
	ECo2               float64 `json:"ECo2"`
	Battery            int     `json:"Battery"`
}

// EnvMonitorEnt is a Bluetooth environment sensor (OMRON / SwitchBot / Inkbird).
type EnvMonitorEnt struct {
	ID        string       `json:"ID"`
	Host      string       `json:"Host"`
	Address   string       `json:"Address"`
	Name      string       `json:"Name"`
	Count     int          `json:"Count"`
	EnvData   []EnvDataEnt `json:"EnvData"`
	FirstTime int64        `json:"FirstTime"`
	LastTime  int64        `json:"LastTime"`
}

// PowerMonitorDataEnt is one smart plug sample.
type PowerMonitorDataEnt struct {
	Time   int64   `json:"Time"`
	Load   float64 `json:"Load"`
	Switch bool    `json:"Switch"`
	Over   bool    `json:"Over"`
	RSSI   int     `json:"RSSI"`
}

// PowerMonitorEnt is a SwitchBot plug mini.
type PowerMonitorEnt struct {
	ID        string                `json:"ID"`
	Host      string                `json:"Host"`
	Address   string                `json:"Address"`
	Name      string                `json:"Name"`
	Count     int                   `json:"Count"`
	Data      []PowerMonitorDataEnt `json:"Data"`
	FirstTime int64                 `json:"FirstTime"`
	LastTime  int64                 `json:"LastTime"`
}

// MotionSensorDataEnt is one motion sensor sample.
type MotionSensorDataEnt struct {
	Time         int64  `json:"Time"`
	Event        string `json:"Event"`
	Moving       bool   `json:"Moving"`
	Light        bool   `json:"Light"`
	Battery      int    `json:"Battery"`
	LastMove     int64  `json:"LastMove"`
	LastMoveDiff int    `json:"LastMoveDiff"`
	RSSI         int    `json:"RSSI"`
}

// MotionSensorEnt is a SwitchBot motion sensor.
type MotionSensorEnt struct {
	ID        string                `json:"ID"`
	Host      string                `json:"Host"`
	Address   string                `json:"Address"`
	Name      string                `json:"Name"`
	Count     int                   `json:"Count"`
	Data      []MotionSensorDataEnt `json:"Data"`
	FirstTime int64                 `json:"FirstTime"`
	LastTime  int64                 `json:"LastTime"`
}

// EtherTypeEnt counts Ethernet frame types seen by twpcap.
type EtherTypeEnt struct {
	ID        string `json:"ID"`
	Host      string `json:"Host"`
	Type      string `json:"Type"`
	Name      string `json:"Name"`
	Count     int    `json:"Count"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}

// DNSQEnt is a DNS question/answer summary.
type DNSQEnt struct {
	ID         string `json:"ID"`
	Host       string `json:"Host"`
	Type       string `json:"Type"`
	Server     string `json:"Server"`
	Name       string `json:"Name"`
	Count      int    `json:"Count"`
	Change     int    `json:"Change"`
	LastClient string `json:"LastClient"`
	LastMAC    string `json:"LastMAC"`
	FirstTime  int64  `json:"FirstTime"`
	LastTime   int64  `json:"LastTime"`
}

// RADIUSFlowEnt is a RADIUS client/server flow.
type RADIUSFlowEnt struct {
	ID           string `json:"ID"`
	Client       string `json:"Client"`
	ClientName   string `json:"ClientName"`
	ClientNodeID string `json:"ClientNodeID"`
	Server       string `json:"Server"`
	ServerName   string `json:"ServerName"`
	ServerNodeID string `json:"ServerNodeID"`
	Accept       int    `json:"Accept"`
	Reject       int    `json:"Reject"`
	Request      int    `json:"Request"`
	Challenge    int    `json:"Challenge"`
	Count        int    `json:"Count"`
	ScoreInfo
	FirstTime  int64 `json:"FirstTime"`
	LastTime   int64 `json:"LastTime"`
	UpdateTime int64 `json:"UpdateTime"`
}

// TLSFlowEnt is a TLS client/server flow.
type TLSFlowEnt struct {
	ID           string `json:"ID"`
	Client       string `json:"Client"`
	ClientName   string `json:"ClientName"`
	ClientNodeID string `json:"ClientNodeID"`
	ClientLoc    string `json:"ClientLoc"`
	Server       string `json:"Server"`
	ServerName   string `json:"ServerName"`
	ServerNodeID string `json:"ServerNodeID"`
	ServerLoc    string `json:"ServerLoc"`
	Service      string `json:"Service"`
	Version      string `json:"Version"`
	Cipher       string `json:"Cipher"`
	Count        int    `json:"Count"`
	ScoreInfo
	FirstTime  int64 `json:"FirstTime"`
	LastTime   int64 `json:"LastTime"`
	UpdateTime int64 `json:"UpdateTime"`
}

// WinEventIDEnt summarises one Windows event ID.
type WinEventIDEnt struct {
	ID        string `json:"ID"`
	Level     string `json:"Level"`
	Provider  string `json:"Provider"`
	EventID   int    `json:"EventID"`
	Computer  string `json:"Computer"`
	Channel   string `json:"Channel"`
	Count     int    `json:"Count"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}

// WinLogonEnt summarises logons for a target/computer/IP.
type WinLogonEnt struct {
	ID         string         `json:"ID"`
	Target     string         `json:"Target"`
	Computer   string         `json:"Computer"`
	IP         string         `json:"IP"`
	Count      int            `json:"Count"`
	Logon      int            `json:"Logon"`
	Logoff     int            `json:"Logoff"`
	Failed     int            `json:"Failed"`
	LogonType  map[string]int `json:"LogonType"`
	FailedCode map[string]int `json:"FailedCode"`
	ScoreInfo
	FirstTime int64 `json:"FirstTime"`
	LastTime  int64 `json:"LastTime"`
}

// WinAccountEnt summarises account operations.
type WinAccountEnt struct {
	ID        string `json:"ID"`
	Subject   string `json:"Subject"`
	Target    string `json:"Target"`
	Computer  string `json:"Computer"`
	Count     int    `json:"Count"`
	Edit      int    `json:"Edit"`
	Password  int    `json:"Password"`
	Other     int    `json:"Other"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}

// WinKerberosEnt summarises Kerberos ticket requests.
type WinKerberosEnt struct {
	ID         string `json:"ID"`
	Target     string `json:"Target"`
	Computer   string `json:"Computer"`
	IP         string `json:"IP"`
	Service    string `json:"Service"`
	TicketType string `json:"TicketType"`
	Count      int    `json:"Count"`
	Failed     int    `json:"Failed"`
	ScoreInfo
	FirstTime int64 `json:"FirstTime"`
	LastTime  int64 `json:"LastTime"`
}

// WinPrivilegeEnt summarises special privilege use.
type WinPrivilegeEnt struct {
	ID        string `json:"ID"`
	Subject   string `json:"Subject"`
	Computer  string `json:"Computer"`
	Count     int    `json:"Count"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}

// WinProcessEnt summarises process start/exit events.
type WinProcessEnt struct {
	ID          string `json:"ID"`
	Process     string `json:"Process"`
	Computer    string `json:"Computer"`
	Count       int    `json:"Count"`
	Start       int    `json:"Start"`
	Exit        int    `json:"Exit"`
	LastSubject string `json:"LastSubject"`
	LastParent  string `json:"LastParent"`
	LastStatus  string `json:"LastStatus"`
	FirstTime   int64  `json:"FirstTime"`
	LastTime    int64  `json:"LastTime"`
}

// WinTaskEnt summarises scheduled task events.
type WinTaskEnt struct {
	ID        string `json:"ID"`
	TaskName  string `json:"TaskName"`
	Computer  string `json:"Computer"`
	Subject   string `json:"Subject"`
	Count     int    `json:"Count"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}
