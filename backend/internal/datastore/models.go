package datastore

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
	MCPFrom        string `json:"MCPFrom"`
	IconSize       int    `json:"IconSize"`
	MapSize        int    `json:"MapSize"`
	ArpWatchRange  string `json:"ArpWatchRange"`
	ArpTimeout     int    `json:"ArpTimeout"`
	OTelRetention  int    `json:"OTelRetention"`
	OTelFrom       string `json:"OTelFrom"`
	LLMProvider    string `json:"LLMProvider"`
	LLMBaseURL     string `json:"LLMBaseURL"`
	LLMAPIKey      string `json:"LLMAPIKey"`
	LLMModel       string `json:"LLMModel"`
	LogFormat      string `json:"LogFormat"`
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
	Subject            string `json:"Subject"`
	Level              string `json:"Level"`
	Interval           int    `json:"Interval"`
	InsecureSkipVerify bool   `json:"InsecureSkipVerify"`
	MailServer         string `json:"MailServer"`
	MailUser           string `json:"MailUser"`
	MailPassword       string `json:"MailPassword"`
	MailTo             string `json:"MailTo"`
	MailFrom           string `json:"MailFrom"`
	WebhookURL         string `json:"WebhookURL"`
	ChatWebhookURL     string `json:"ChatWebhookURL"`
	LineToken          string `json:"LineToken"`
	SlackWebhookURL    string `json:"SlackWebhookURL"`
}

// DiscoverConfEnt holds auto-discovery parameters.
type DiscoverConfEnt struct {
	IPRange    string `json:"IPRange"`
	AddPolling bool   `json:"AddPolling"`
	AutoAck    bool   `json:"AutoAck"`
	Timeout    int    `json:"Timeout"`
	Retry      int    `json:"Retry"`
	X          int    `json:"X"`
	Y          int    `json:"Y"`
}
