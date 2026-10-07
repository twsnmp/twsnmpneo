package datastore

import "encoding/json"

// DrawItemType represents the shape/type of a draw item on the map.
type DrawItemType int

const (
	DrawItemTypeRect DrawItemType = iota
	DrawItemTypeEllipse
	DrawItemTypeText
	DrawItemTypeImage
	DrawItemTypePollingText
	DrawItemTypePollingGauge
	DrawItemTypePollingNewGauge
	DrawItemTypePollingBar
	DrawItemTypePollingLine
	DrawItemTypeGroupFrame
	DrawItemTypeGroupFill
	DrawItemTypePollingKPI
)

// LogMode defines the logging mode for polling operations.
const (
	LogModeNone = iota
	LogModeAlways
	LogModeOnChange
	LogModeAI
)

// NodeEnt represents a managed device/node in TWSNMP NEO.
// Data model is prioritized from twsnmpfk.
type NodeEnt struct {
	ID           string `json:"ID"`
	Name         string `json:"Name"`
	Descr        string `json:"Descr"`
	Icon         string `json:"Icon"`
	Image        string `json:"Image"`
	State        string `json:"State"`
	X            int    `json:"X"`
	Y            int    `json:"Y"`
	IP           string `json:"IP"`
	MAC          string `json:"MAC"`
	Vendor       string `json:"Vendor"`
	SnmpMode     string `json:"SnmpMode"`
	Community    string `json:"Community"`
	User         string `json:"User"`
	SSHUser      string `json:"SSHUser"`
	Password     string `json:"Password"`
	GNMIPort     string `json:"GNMIPort"`
	GNMIEncoding string `json:"GNMIEncoding"`
	GNMIUser     string `json:"GNMIUser"`
	GNMIPassword string `json:"GNMIPassword"`
	PublicKey    string `json:"PublicKey"`
	URL          string `json:"URL"`
	AddrMode     string `json:"AddrMode"`
	AutoAck      bool   `json:"AutoAck"`
	Loc          string `json:"Loc"`
	SnmpPort     int    `json:"SnmpPort"`
}

// UnmarshalJSON implements custom JSON unmarshaling to seamlessly handle both snake_case
// and PascalCase/camelCase field names from frontend API payloads.
func (n *NodeEnt) UnmarshalJSON(data []byte) error {
	type Alias NodeEnt
	aux := &struct {
		SnakeAddrMode     *string `json:"addr_mode"`
		SnakeAutoAck      *bool   `json:"auto_ack"`
		SnakeSnmpMode     *string `json:"snmp_mode"`
		SnakeSnmpPort     *int    `json:"snmp_port"`
		SnakeSSHUser      *string `json:"ssh_user"`
		SnakePublicKey    *string `json:"public_key"`
		SnakeGNMIPort     *string `json:"gnmi_port"`
		SnakeGNMIEncoding *string `json:"gnmi_encoding"`
		SnakeGNMIUser     *string `json:"gnmi_user"`
		SnakeGNMIPassword *string `json:"gnmi_password"`
		*Alias
	}{
		Alias: (*Alias)(n),
	}
	if err := json.Unmarshal(data, aux); err != nil {
		return err
	}
	if aux.SnakeAddrMode != nil && *aux.SnakeAddrMode != "" {
		n.AddrMode = *aux.SnakeAddrMode
	}
	if aux.SnakeAutoAck != nil {
		n.AutoAck = *aux.SnakeAutoAck
	}
	if aux.SnakeSnmpMode != nil && *aux.SnakeSnmpMode != "" {
		n.SnmpMode = *aux.SnakeSnmpMode
	}
	if aux.SnakeSnmpPort != nil && *aux.SnakeSnmpPort != 0 {
		n.SnmpPort = *aux.SnakeSnmpPort
	}
	if aux.SnakeSSHUser != nil && *aux.SnakeSSHUser != "" {
		n.SSHUser = *aux.SnakeSSHUser
	}
	if aux.SnakePublicKey != nil && *aux.SnakePublicKey != "" {
		n.PublicKey = *aux.SnakePublicKey
	}
	if aux.SnakeGNMIPort != nil && *aux.SnakeGNMIPort != "" {
		n.GNMIPort = *aux.SnakeGNMIPort
	}
	if aux.SnakeGNMIEncoding != nil && *aux.SnakeGNMIEncoding != "" {
		n.GNMIEncoding = *aux.SnakeGNMIEncoding
	}
	if aux.SnakeGNMIUser != nil && *aux.SnakeGNMIUser != "" {
		n.GNMIUser = *aux.SnakeGNMIUser
	}
	if aux.SnakeGNMIPassword != nil && *aux.SnakeGNMIPassword != "" {
		n.GNMIPassword = *aux.SnakeGNMIPassword
	}
	return nil
}

// LineEnt represents a connection between two nodes on the map.
type LineEnt struct {
	ID         string `json:"ID"`
	NodeID1    string `json:"NodeID1"`
	PollingID1 string `json:"PollingID1"`
	State1     string `json:"State1"`
	NodeID2    string `json:"NodeID2"`
	PollingID2 string `json:"PollingID2"`
	State2     string `json:"State2"`
	PollingID  string `json:"PollingID"`
	Width      int    `json:"Width"`
	State      string `json:"State"`
	Info       string `json:"Info"`
	Port       string `json:"Port"`
}

// PortEnt represents a port within a NetworkEnt.
type PortEnt struct {
	ID      string `json:"ID"`
	Name    string `json:"Name"`
	Polling string `json:"Polling"`
	Index   string `json:"Index"`
	X       int    `json:"X"`
	Y       int    `json:"Y"`
	State   string `json:"State"`
}

// NetworkEnt represents a network cloud or switch segment.
type NetworkEnt struct {
	ID        string    `json:"ID"`
	Name      string    `json:"Name"`
	Descr     string    `json:"Descr"`
	IP        string    `json:"IP"`
	SnmpMode  string    `json:"SnmpMode"`
	Community string    `json:"Community"`
	User      string    `json:"User"`
	Password  string    `json:"Password"`
	SnmpPort  int       `json:"SnmpPort"`
	URL       string    `json:"URL"`
	ArpWatch  bool      `json:"ArpWatch"`
	Unmanaged bool      `json:"Unmanaged"`
	HPorts    int       `json:"HPorts"`
	X         int       `json:"X"`
	Y         int       `json:"Y"`
	W         int       `json:"W"`
	H         int       `json:"H"`
	SystemID  string    `json:"SystemID"`
	Error     string    `json:"Error"`
	LLDP      bool      `json:"LLDP"`
	Ports     []PortEnt `json:"Ports"`
}

// DrawItemEnt represents drawing items (shapes, KPI cards, text) on the map.
type DrawItemEnt struct {
	ID            string       `json:"ID"`
	Type          DrawItemType `json:"Type"`
	X             int          `json:"X"`
	Y             int          `json:"Y"`
	W             int          `json:"W"`
	H             int          `json:"H"`
	Color         string       `json:"Color"`
	Path          string       `json:"Path"`
	Text          string       `json:"Text"`
	Size          int          `json:"Size"`
	PollingID     string       `json:"PollingID"`
	VarName       string       `json:"VarName"`
	Format        string       `json:"Format"`
	Value         float64      `json:"Value"`
	Scale         float64      `json:"Scale"`
	Cond          int          `json:"Cond"`
	Values        []float64    `json:"Values"`
	FormattedText string       `json:"FormattedText,omitempty"`
}

// PollingEnt defines a monitoring task for a node.
type PollingEnt struct {
	ID           string                 `json:"ID"`
	Name         string                 `json:"Name"`
	NodeID       string                 `json:"NodeID"`
	Type         string                 `json:"Type"`
	Mode         string                 `json:"Mode"`
	Params       string                 `json:"Params"`
	Filter       string                 `json:"Filter"`
	Extractor    string                 `json:"Extractor"`
	Script       string                 `json:"Script"`
	Level        string                 `json:"Level"`
	PollInt      int                    `json:"PollInt"`
	Timeout      int                    `json:"Timeout"`
	Retry        int                    `json:"Retry"`
	LogMode      int                    `json:"LogMode"`
	NextTime     int64                  `json:"NextTime"`
	LastTime     int64                  `json:"LastTime"`
	Result       map[string]interface{} `json:"Result"`
	State        string                 `json:"State"`
	FailAction   string                 `json:"FailAction"`
	RepairAction string                 `json:"RepairAction"`
	AIMode       string                 `json:"AIMode"`
	VectorCols   string                 `json:"VectorCols"`
	MqttURL      string                 `json:"MqttURL"`
	MqttTopic    string                 `json:"MqttTopic"`
	MqttCols     string                 `json:"MqttCols"`
	FailTime     int64                  `json:"FailTime"`
}

// PollingLogEnt records a historical polling result.
type PollingLogEnt struct {
	Time      int64                  `json:"Time"`
	PollingID string                 `json:"PollingID"`
	State     string                 `json:"State"`
	Result    map[string]interface{} `json:"Result"`
}

// EventLogEnt records an anomaly, alert, or system event.
type EventLogEnt struct {
	Time      int64  `json:"Time"`
	Type      string `json:"Type"`
	Level     string `json:"Level"`
	NodeName  string `json:"NodeName"`
	NodeID    string `json:"NodeID"`
	Event     string `json:"Event"`
	LastLevel string `json:"LastLevel"`
	Downtime  int64  `json:"Downtime,omitempty"`
}

// EventLogFilter defines query search options for event logs.
type EventLogFilter struct {
	StartTime int64  `json:"start"`
	EndTime   int64  `json:"end"`
	Level     string `json:"level"`
	Type      string `json:"type"`
	NodeID    string `json:"nodeId"`
	NodeName  string `json:"nodeName"`
	Filter    string `json:"filter"`
	Limit     int    `json:"limit"`
}

// BackImageEnt holds background image settings for map rendering.
type BackImageEnt struct {
	X      int    `json:"X"`
	Y      int    `json:"Y"`
	Width  int    `json:"Width"`
	Height int    `json:"Height"`
	Path   string `json:"Path"`
}

// MapConfEnt holds map and general monitoring configuration.
type MapConfEnt struct {
	MapName        string `json:"MapName"`
	PollInt        int    `json:"PollInt"`
	Timeout        int    `json:"Timeout"`
	Retry          int    `json:"Retry"`
	LogDays        int    `json:"LogDays"`
	SnmpMode       string `json:"SnmpMode"`
	Community      string `json:"Community"`
	SnmpUser       string `json:"SnmpUser"`
	SnmpPassword   string `json:"SnmpPassword"`
	EnableSyslogd  bool   `json:"EnableSyslogd"`
	EnableTrapd    bool   `json:"EnableTrapd"`
	EnableArpWatch bool   `json:"EnableArpWatch"`
	EnableNetflowd bool   `json:"EnableNetflowd"`
	EnableSshd     bool   `json:"EnableSshd"`
	EnableSFlowd   bool   `json:"EnableSFlowd"`
	EnableTcpd     bool   `json:"EnableTcpd"`
	EnableOTel     bool   `json:"EnableOTel"`
	EnableMqtt     bool   `json:"EnableMqtt"`
	MqttToSyslog   bool   `json:"Mqtt2Syslog"`
	MCPTransport   string `json:"MCPTransport"`
	MCPEndpoint    string `json:"MCPEndpoint"`
	MCPToken       string `json:"MCPToken"`
	MCPFrom        string  `json:"MCPFrom"`
	IconSize       int     `json:"IconSize"`
	MapSize        int     `json:"MapSize"`
	ArpWatchRange  string  `json:"ArpWatchRange"`
	ArpTimeout     int     `json:"ArpTimeout"`
	OTelRetention  int     `json:"OTelRetention"`
	OTelFrom       string  `json:"OTelFrom"`
	ReportDays     int     `json:"ReportDays"`
	ReportLimit    int     `json:"ReportLimit"`
	ScoreThreshold float64 `json:"ScoreThreshold"`
	FumbleThreshold int    `json:"FumbleThreshold"`
	LLMProvider    string  `json:"LLMProvider"`
	LLMBaseURL     string  `json:"LLMBaseURL"`
	LLMAPIKey      string  `json:"LLMAPIKey"`
	LLMModel       string  `json:"LLMModel"`
	LogFormat      string  `json:"LogFormat"`
	GeoIPInfo      string  `json:"GeoIPInfo,omitempty"`
}

// NetFlowEnt represents a decoded NetFlow / IPFIX flow log entry.
type NetFlowEnt struct {
	Time     int64   `json:"Time"`
	SrcAddr  string  `json:"SrcAddr"`
	SrcPort  int     `json:"SrcPort"`
	SrcLoc   string  `json:"SrcLoc"`
	SrcMAC   string  `json:"SrcMAC"`
	DstAddr  string  `json:"DstAddr"`
	DstPort  int     `json:"DstPort"`
	DstLoc   string  `json:"DstLoc"`
	DstMAC   string  `json:"DstMAC"`
	Bytes    int     `json:"Bytes"`
	Packets  int     `json:"Packets"`
	TCPFlags string  `json:"TCPFlags"`
	Protocol string  `json:"Protocol"`
	ToS      int     `json:"ToS"`
	Start    int64   `json:"Start"`
	End      int64   `json:"End"`
	Dur      float64 `json:"Dur"`
}

// SFlowEnt represents a decoded sFlow flow sample entry.
type SFlowEnt struct {
	Time     int64  `json:"Time"`
	SrcAddr  string `json:"SrcAddr"`
	SrcPort  int    `json:"SrcPort"`
	SrcLoc   string `json:"SrcLoc"`
	SrcMAC   string `json:"SrcMAC"`
	DstAddr  string `json:"DstAddr"`
	DstPort  int    `json:"DstPort"`
	DstLoc   string `json:"DstLoc"`
	DstMAC   string `json:"DstMAC"`
	Bytes    int    `json:"Bytes"`
	TCPFlags string `json:"TCPFlags"`
	Protocol string `json:"Protocol"`
	Reason   int    `json:"Reason"`
}

// SFlowCounterEnt represents an sFlow counter sample entry.
type SFlowCounterEnt struct {
	Type   string `json:"Type"`
	Remote string `json:"Remote"`
	Data   string `json:"Data"`
}

// LocConfEnt holds GIS/geographic map configuration.
type LocConfEnt struct {
	Style    string  `json:"Style"`
	Center   string  `json:"Center"`
	Zoom     float64 `json:"Zoom"`
	IconSize int     `json:"IconSize"`
}

// NotifyConfEnt holds alert notification configuration.
type NotifyConfEnt struct {
	Provider           string `json:"Provider"`
	MailServer         string `json:"MailServer"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"`
	User               string `json:"User"`
	Password           string `json:"Password"`
	MailTo             string `json:"MailTo"`
	MailFrom           string `json:"MailFrom"`
	Subject            string `json:"Subject"`
	Interval           int    `json:"Interval"`
	Level              string `json:"Level"`
	Report             bool   `json:"Report"`
	LLMSummary         bool   `json:"LLMSummary"`
	NotifyRepair       bool   `json:"NotifyRepair"`
	CheckDependency    bool   `json:"CheckDependency"`
	ExecCmd            string `json:"ExecCmd"`
	WebHookNotify      string `json:"WebHookNotify"`
	WebHookReport      string `json:"WebHookReport"`
	ClientID           string `json:"ClientID"`
	ClientSecret       string `json:"ClientSecret"`
	MSTenant           string `json:"MSTenant"`
}

func (n *NotifyConfEnt) UnmarshalJSON(data []byte) error {
	type notifyConfAlias NotifyConfEnt
	var current notifyConfAlias
	if err := json.Unmarshal(data, &current); err != nil {
		return err
	}
	var legacy struct {
		MailUser        string `json:"MailUser"`
		MailPassword    string `json:"MailPassword"`
		WebhookURL      string `json:"WebhookURL"`
		ChatWebhookURL  string `json:"ChatWebhookURL"`
		SlackWebhookURL string `json:"SlackWebhookURL"`
	}
	if err := json.Unmarshal(data, &legacy); err != nil {
		return err
	}
	*n = NotifyConfEnt(current)
	if n.User == "" {
		n.User = legacy.MailUser
	}
	if n.Password == "" {
		n.Password = legacy.MailPassword
	}
	if n.WebHookNotify == "" {
		switch {
		case legacy.WebhookURL != "":
			n.WebHookNotify = legacy.WebhookURL
		case legacy.SlackWebhookURL != "":
			n.WebHookNotify = legacy.SlackWebhookURL
		case legacy.ChatWebhookURL != "":
			n.WebHookNotify = legacy.ChatWebhookURL
		}
	}
	return nil
}

// AutoLine constants for discovery and topology connection
const (
	AutoLineNone        = 0
	AutoLineStrict      = 1
	AutoLineSpeculative = 2
)

// AutoLayout constants
const (
	AutoLayoutNone         = 0 // Sequential Grid (Default)
	AutoLayoutHierarchical = 1 // Hierarchical (Tree)
	AutoLayoutCluster      = 2 // Cluster (Hub & Spoke)
	AutoLayoutCategorized  = 3 // Categorized (By device type)
)

// SnmpConfEnt represents candidate SNMP configuration for discovery.
type SnmpConfEnt struct {
	SnmpMode     string `json:"SnmpMode"`
	Community    string `json:"Community"`
	SnmpUser     string `json:"SnmpUser"`
	SnmpPassword string `json:"SnmpPassword"`
}

// DiscoverConfEnt holds auto-discovery parameters.
type DiscoverConfEnt struct {
	StartIP      string        `json:"StartIP"`
	EndIP        string        `json:"EndIP"`
	Timeout      int           `json:"Timeout"`
	Retry        int           `json:"Retry"`
	X            int           `json:"X"`
	Y            int           `json:"Y"`
	AddPolling   bool          `json:"AddPolling"`
	PortScan     bool          `json:"PortScan"`
	ReCheck      bool          `json:"ReCheck"`
	AddNetwork   bool          `json:"AddNetwork"`
	AutoDetect   bool          `json:"AutoDetect"`
	AutoDetectAI bool          `json:"AutoDetectAI"`
	AutoLine     int           `json:"AutoLine"`
	AutoLayout   int           `json:"AutoLayout"`
	SnmpConfigs  []SnmpConfEnt `json:"SnmpConfigs"`

	// Legacy / Compatibility fields
	IPRange string `json:"IPRange,omitempty"`
	AutoAck bool   `json:"AutoAck,omitempty"`
}

// ArpEnt represents an entry in the ARP cache table.
type ArpEnt struct {
	IP        string `json:"IP"`
	MAC       string `json:"MAC"`
	NodeID    string `json:"NodeID,omitempty"`
	Vendor    string `json:"Vendor,omitempty"`
	FirstTime int64  `json:"FirstTime"`
	LastTime  int64  `json:"LastTime"`
}

// ArpLogEnt represents an ARP change or discovery log entry.
type ArpLogEnt struct {
	Time      int64  `json:"Time"`
	State     string `json:"State"`
	IP        string `json:"IP"`
	Node      string `json:"Node,omitempty"`
	NewMAC    string `json:"NewMAC"`
	NewVendor string `json:"NewVendor,omitempty"`
	OldMAC    string `json:"OldMAC,omitempty"`
	OldVendor string `json:"OldVendor,omitempty"`
}

// OTelMetricDataPointEnt represents a single data point within an OTel metric.
type OTelMetricDataPointEnt struct {
	Start          int64     `json:"Start"`
	Time           int64     `json:"Time"`
	Attributes     []string  `json:"Attributes"`
	Count          uint64    `json:"Count"`
	BucketCounts   []uint64  `json:"BucketCounts"`
	ExplicitBounds []float64 `json:"ExplicitBounds"`
	Sum            float64   `json:"Sum"`
	Min            float64   `json:"Min"`
	Max            float64   `json:"Max"`
	Gauge          float64   `json:"Gauge"`
	Positive       []uint64  `json:"Positive"`
	Negative       []uint64  `json:"Negative"`
	Scale          int64     `json:"Scale"`
	ZeroCount      int64     `json:"ZeroCount"`
	ZeroThreshold  float64   `json:"ZeroThreshold"`
	Index          int       `json:"Index"`
}

// OTelMetricEnt represents an aggregated OpenTelemetry metric series.
type OTelMetricEnt struct {
	Host        string                    `json:"Host"`
	Service     string                    `json:"Service"`
	Scope       string                    `json:"Scope"`
	Name        string                    `json:"Name"`
	Type        string                    `json:"Type"`
	Description string                    `json:"Description"`
	Unit        string                    `json:"Unit"`
	DataPoints  []*OTelMetricDataPointEnt `json:"DataPoints"`
	Count       int                       `json:"Count"`
	First       int64                     `json:"First"`
	Last        int64                     `json:"Last"`
}

// OTelTraceSpanEnt represents a single span within an OpenTelemetry trace.
type OTelTraceSpanEnt struct {
	SpanID       string   `json:"SpanID"`
	ParentSpanID string   `json:"ParentSpanID"`
	Host         string   `json:"Host"`
	Service      string   `json:"Service"`
	Scope        string   `json:"Scope"`
	Name         string   `json:"Name"`
	Start        int64    `json:"Start"`
	End          int64    `json:"End"`
	Dur          float64  `json:"Dur"`
	Attributes   []string `json:"Attributes"`
}

// OTelTraceEnt represents a distributed trace composed of spans.
type OTelTraceEnt struct {
	Bucket    string             `json:"Bucket"`
	TraceID   string             `json:"TraceID"`
	Start     int64              `json:"Start"`
	End       int64              `json:"End"`
	Dur       float64            `json:"Dur"`
	Spans     []OTelTraceSpanEnt `json:"Spans"`
	Last      int64              `json:"Last"`
	SavedLast int64              `json:"-"`
}

// OTelTraceSummaryEnt represents a summarized trace row for table display.
type OTelTraceSummaryEnt struct {
	Bucket   string  `json:"Bucket"`
	TraceID  string  `json:"TraceID"`
	Hosts    string  `json:"Hosts"`
	Services string  `json:"Services"`
	Scopes   string  `json:"Scopes"`
	Start    int64   `json:"Start"`
	End      int64   `json:"End"`
	Dur      float64 `json:"Dur"`
	NumSpan  int     `json:"NumSpan"`
}

// OTelTraceDAGNodeEnt represents a service node in an OTel trace DAG.
type OTelTraceDAGNodeEnt struct {
	Name  string `json:"Name"`
	Count int    `json:"Count"`
}

// OTelTraceDAGLinkEnt represents a directed call relationship between services.
type OTelTraceDAGLinkEnt struct {
	Src   string `json:"Src"`
	Dst   string `json:"Dst"`
	Count int    `json:"Count"`
}

// OTelTraceDAGEnt represents the service dependency graph derived from traces.
type OTelTraceDAGEnt struct {
	Nodes []OTelTraceDAGNodeEnt `json:"Nodes"`
	Links []OTelTraceDAGLinkEnt `json:"Links"`
}

// OTelLogEnt represents a structured OpenTelemetry log record.
type OTelLogEnt struct {
	Time         int64             `json:"time"`
	Host         string            `json:"host"`
	Service      string            `json:"service"`
	Scope        string            `json:"scope"`
	TraceID      string            `json:"traceId"`
	SpanID       string            `json:"spanId"`
	Severity     int               `json:"severity"`     // 1-7, syslog compatible
	SeverityText string            `json:"severityText"` // INFO, WARN, etc.
	Message      string            `json:"message"`
	Attributes   map[string]string `json:"attributes,omitempty"`
}

// MqttStatEnt represents statistical state for an MQTT client and topic.
type MqttStatEnt struct {
	ID       string `json:"ID"`
	State    string `json:"State"`
	ClientID string `json:"ClientID"`
	Topic    string `json:"Topic"`
	Remote   string `json:"Remote"`
	Count    int    `json:"Count"`
	Bytes    int64  `json:"Bytes"`
	First    int64  `json:"First"`
	Last     int64  `json:"Last"`
	Value    string `json:"Value"`
}

// MqttLogEnt represents a structured MQTT log stored in Parquet.
type MqttLogEnt struct {
	Time     int64  `json:"time"`
	Topic    string `json:"topic"`
	ClientID string `json:"clientID"`
	Remote   string `json:"remote"`
	Payload  string `json:"payload"`
}

// CertMonitorEnt represents an external TLS certificate monitor target matching twsnmpfk.
type CertMonitorEnt struct {
	ID           string `json:"id"`
	State        string `json:"state"` // normal, warn, error
	Target       string `json:"target"`
	Port         int    `json:"port"`
	Subject      string `json:"subject"`
	Issuer       string `json:"issuer"`
	SerialNumber string `json:"serialNumber"`
	Verify       bool   `json:"verify"`
	NotAfter     int64  `json:"notAfter"`   // unix seconds
	NotBefore    int64  `json:"notBefore"` // unix seconds
	Error        string `json:"error"`
	FirstTime    int64  `json:"firstTime"` // unix seconds
	LastTime     int64  `json:"lastTime"`  // unix seconds
}

// SensorEnt represents a remote sensor reporting to TWSNMP (syslog, twWifiScan, twBlueScan, sflow, netflow, mqtt, etc.).
type SensorEnt struct {
	ID          string             `json:"ID"`
	Host        string             `json:"Host"`
	Type        string             `json:"Type"`
	Param       string             `json:"Param"`
	Total       int64              `json:"Total"`
	Send        int64              `json:"Send"`
	State       string             `json:"State"`
	Ignore      bool               `json:"Ignore"`
	Stats       []SensorStatsEnt   `json:"Stats,omitempty"`
	Monitors    []SensorMonitorEnt `json:"Monitors,omitempty"`
	StatsLen    int                `json:"StatsLen"`
	MonitorsLen int                `json:"MonitorsLen"`
	FirstTime   int64              `json:"FirstTime"` // unix nano
	LastTime    int64              `json:"LastTime"`  // unix nano
}

// SensorStatsEnt represents periodic message rate / count telemetry for a sensor.
type SensorStatsEnt struct {
	Time     int64   `json:"Time"` // unix nano
	Total    int64   `json:"Total"`
	Count    int64   `json:"Count"`
	PS       float64 `json:"PS"`
	Send     int64   `json:"Send"`
	LastSend int64   `json:"LastSend"`
}

// SensorMonitorEnt represents periodic system resource monitoring telemetry for a sensor host.
type SensorMonitorEnt struct {
	Time    int64   `json:"Time"` // unix nano
	CPU     float64 `json:"CPU"`
	Mem     float64 `json:"Mem"`
	Load    float64 `json:"Load"`
	Process int64   `json:"Process"`
	Recv    int64   `json:"Recv"`
	Sent    int64   `json:"Sent"`
	TxSpeed float64 `json:"TxSpeed"`
	RxSpeed float64 `json:"RxSpeed"`
}

// IconEnt represents a custom icon definition (MDI unicode or custom image data).
type IconEnt struct {
	ID    string `json:"ID,omitempty"`
	Name  string `json:"Name"`
	Code  int    `json:"Code"`
	Type  string `json:"Type,omitempty"`  // "mdi" (default) or "image"
	Image string `json:"Image,omitempty"` // Base64 Data URL (data:image/...)
}


