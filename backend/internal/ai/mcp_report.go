package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/logreport"
)

type mcpSensorMonitorEnt struct {
	CPU     float64 `json:"cpu"`
	Memory  float64 `json:"memory"`
	Load    float64 `json:"load"`
	Process int64   `json:"process"`
}

type mcpSensorEnt struct {
	Host      string              `json:"host"`
	Type      string              `json:"type"`
	Total     int64               `json:"total"`
	Send      int64               `json:"send"`
	State     string              `json:"state"`
	Monitor   mcpSensorMonitorEnt `json:"monitor"`
	FirstTime string              `json:"first_time"`
	LastTime  string              `json:"last_time"`
}

type mcpGetSensorListParams struct {
	StateFilter string `json:"state_filter,omitempty" jsonschema:"state_filter uses a regular expression to specify criteria for sensor state (normal,warn,low,high,unknown)."`
}

type mcpMACEnt struct {
	MAC       string  `json:"mac"`
	Name      string  `json:"name"`
	IP        string  `json:"ip"`
	Vendor    string  `json:"vendor"`
	Score     float64 `json:"score"`
	Penalty   int64   `json:"penalty"`
	FirstTime string  `json:"first_time"`
	LastTime  string  `json:"last_time"`
}

type mcpIPEnt struct {
	IP        string  `json:"ip"`
	MAC       string  `json:"mac"`
	Name      string  `json:"name"`
	Location  string  `json:"location"`
	Vendor    string  `json:"vendor"`
	Count     int64   `json:"count"`
	Change    int64   `json:"change"`
	Score     float64 `json:"score"`
	Penalty   int64   `json:"penalty"`
	FirstTime string  `json:"first_time"`
	LastTime  string  `json:"last_time"`
}

type mcpWifiAPEnt struct {
	Host      string `json:"host"`
	BSSID     string `json:"bssid"`
	SSID      string `json:"ssid"`
	RSSI      int    `json:"rssi"`
	Channel   string `json:"channel"`
	Vendor    string `json:"vendor"`
	Info      string `json:"info"`
	Count     int    `json:"count"`
	Change    int    `json:"change"`
	FirstTime string `json:"first_time"`
	LastTime  string `json:"last_time"`
}

type mcpBluetoothDeviceEnt struct {
	Host        string `json:"host"`
	Address     string `json:"address"`
	Name        string `json:"name"`
	AddressType string `json:"address_type"`
	RSSI        int    `json:"rssi"`
	Info        string `json:"info"`
	Vendor      string `json:"vendor"`
	Count       int64  `json:"count"`
	FirstTime   string `json:"first_time"`
	LastTime    string `json:"last_time"`
}

type mcpGetBluetoothDeviceListParams struct {
	PublicAddressOnly bool `json:"public_address_only,omitempty" jsonschema:"List only devices with Bluetooth address type Public"`
}

type mcpServerCertificateEnt struct {
	Server       string  `json:"server"`
	Port         int     `json:"port"`
	Subject      string  `json:"subject"`
	Issuer       string  `json:"issuer"`
	SerialNumber string  `json:"serial_number"`
	Verify       bool    `json:"verify"`
	NotAfter     string  `json:"not_after"`
	NotBefore    string  `json:"not_before"`
	Error        string  `json:"error"`
	Score        float64 `json:"score"`
	Penalty      int64   `json:"penalty"`
	FirstTime    string  `json:"first_time"`
	LastTime     string  `json:"last_time"`
}

type mcpResourceMonitorEnt struct {
	Time   string `json:"time"`
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Swap   string `json:"swap"`
	Disk   string `json:"disk"`
	Load   string `json:"load"`
}

func (s *MCPServer) registerReportTools(server *mcp.Server) {
	// 1. get_sensor_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sensor_list",
		Description: "Get list of remote monitoring sensors reporting to TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetSensorListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpSensorEnt{})
		}
		sensors, err := s.store.ListSensors(ctx)
		if err != nil {
			return nil, nil, err
		}
		stateFilter := makeRegexFilter(args.StateFilter)
		list := make([]mcpSensorEnt, 0)
		for _, sn := range sensors {
			if sn.Ignore {
				continue
			}
			if stateFilter != nil && !stateFilter.MatchString(sn.State) {
				continue
			}
			mon := mcpSensorMonitorEnt{}
			if len(sn.Monitors) > 0 {
				last := sn.Monitors[len(sn.Monitors)-1]
				mon.CPU = last.CPU
				mon.Memory = last.Mem
				mon.Load = last.Load
				mon.Process = last.Process
			}
			list = append(list, mcpSensorEnt{
				Host:      sn.Host,
				Type:      sn.Type,
				Total:     sn.Total,
				Send:      sn.Send,
				State:     sn.State,
				Monitor:   mon,
				FirstTime: time.Unix(0, sn.FirstTime).Format(time.RFC3339Nano),
				LastTime:  time.Unix(0, sn.LastTime).Format(time.RFC3339Nano),
			})
		}
		return jsonResult(list)
	})

	// 2. get_mac_address_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_mac_address_list",
		Description: "Get list of MAC addresses and LAN devices tracked by TWSNMP NEO",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpMACEnt{})
		}
		arps, _ := s.store.LoadArpTable(ctx)
		nodes, _ := s.store.ListNodes(ctx)
		nodeMap := make(map[string]*datastore.NodeEnt)
		for _, n := range nodes {
			if n.MAC != "" {
				nodeMap[strings.ToLower(n.MAC)] = n
			}
		}

		list := make([]mcpMACEnt, 0)
		for _, a := range arps {
			name := a.IP
			vendor := ""
			normMAC := strings.ToLower(a.MAC)
			if n, ok := nodeMap[normMAC]; ok {
				name = n.Name
				vendor = n.Vendor
			}
			firstTime := ""
			lastTime := ""
			if a.FirstTime > 0 {
				firstTime = time.Unix(0, a.FirstTime).Format(time.RFC3339Nano)
			}
			if a.LastTime > 0 {
				lastTime = time.Unix(0, a.LastTime).Format(time.RFC3339Nano)
			}
			list = append(list, mcpMACEnt{
				MAC:       a.MAC,
				Name:      name,
				IP:        a.IP,
				Vendor:    vendor,
				FirstTime: firstTime,
				LastTime:  lastTime,
			})
		}
		return jsonResult(list)
	})

	// 3. get_ip_address_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_ip_address_list",
		Description: "Get list of IP addresses discovered on the managed network",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpIPEnt{})
		}
		arps, _ := s.store.LoadArpTable(ctx)
		nodes, _ := s.store.ListNodes(ctx)
		nodeMap := make(map[string]*datastore.NodeEnt)
		for _, n := range nodes {
			nodeMap[n.IP] = n
		}

		list := make([]mcpIPEnt, 0)
		for _, a := range arps {
			name := a.IP
			vendor := ""
			if n, ok := nodeMap[a.IP]; ok {
				name = n.Name
				vendor = n.Vendor
			}
			firstTime := ""
			lastTime := ""
			if a.FirstTime > 0 {
				firstTime = time.Unix(0, a.FirstTime).Format(time.RFC3339Nano)
			}
			if a.LastTime > 0 {
				lastTime = time.Unix(0, a.LastTime).Format(time.RFC3339Nano)
			}
			list = append(list, mcpIPEnt{
				IP:        a.IP,
				MAC:       a.MAC,
				Name:      name,
				Vendor:    vendor,
				FirstTime: firstTime,
				LastTime:  lastTime,
			})
		}
		return jsonResult(list)
	})

	// 4. get_wifi_ap_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_wifi_ap_list",
		Description: "Get list of Wi-Fi access points detected by wireless sensors",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpWifiAPEnt{})
		}
		rawMap, err := s.store.ListLogReportData(ctx, "wifi")
		if err != nil {
			return nil, nil, err
		}
		list := make([]mcpWifiAPEnt, 0)
		for _, b := range rawMap {
			var ap logreport.WifiAPEnt
			if err := json.Unmarshal(b, &ap); err == nil {
				rssi := 0
				if len(ap.RSSI) > 0 {
					rssi = ap.RSSI[len(ap.RSSI)-1].Value
				}
				list = append(list, mcpWifiAPEnt{
					Host:      ap.Host,
					BSSID:     ap.BSSID,
					SSID:      ap.SSID,
					Channel:   ap.Channel,
					Vendor:    ap.Vendor,
					Info:      ap.Info,
					RSSI:      rssi,
					Count:     ap.Count,
					Change:    ap.Change,
					FirstTime: time.Unix(0, ap.FirstTime).Format(time.RFC3339Nano),
					LastTime:  time.Unix(0, ap.LastTime).Format(time.RFC3339Nano),
				})
			}
		}
		return jsonResult(list)
	})

	// 5. get_bluetooth_device_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_bluetooth_device_list",
		Description: "Get list of Bluetooth devices detected by BLE scan sensors",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args mcpGetBluetoothDeviceListParams) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpBluetoothDeviceEnt{})
		}
		rawMap, err := s.store.ListLogReportData(ctx, "blueDevice")
		if err != nil {
			return nil, nil, err
		}
		list := make([]mcpBluetoothDeviceEnt, 0)
		for _, b := range rawMap {
			var dev logreport.BlueDeviceEnt
			if err := json.Unmarshal(b, &dev); err == nil {
				if args.PublicAddressOnly && !strings.Contains(dev.AddressType, "Public") {
					continue
				}
				rssi := 0
				if len(dev.RSSI) > 0 {
					rssi = dev.RSSI[len(dev.RSSI)-1].Value
				}
				list = append(list, mcpBluetoothDeviceEnt{
					Host:        dev.Host,
					Address:     dev.Address,
					Name:        dev.Name,
					AddressType: dev.AddressType,
					RSSI:        rssi,
					Info:        dev.Info,
					Vendor:      dev.Vendor,
					Count:       int64(dev.Count),
					FirstTime:   time.Unix(0, dev.FirstTime).Format(time.RFC3339Nano),
					LastTime:    time.Unix(0, dev.LastTime).Format(time.RFC3339Nano),
				})
			}
		}
		return jsonResult(list)
	})

	// 6. get_server_certificate_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_server_certificate_list",
		Description: "Get list of TLS/SSL server certificates and validity periods",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		if s.store == nil {
			return jsonResult([]mcpServerCertificateEnt{})
		}
		certs, err := s.store.ListCertMonitors(ctx)
		if err != nil {
			return nil, nil, err
		}
		list := make([]mcpServerCertificateEnt, 0)
		for _, c := range certs {
			nb := ""
			na := ""
			if c.NotBefore > 0 {
				nb = time.Unix(c.NotBefore, 0).Format(time.RFC3339)
			}
			if c.NotAfter > 0 {
				na = time.Unix(c.NotAfter, 0).Format(time.RFC3339)
			}
			list = append(list, mcpServerCertificateEnt{
				Server:       c.Target,
				Port:         c.Port,
				Subject:      c.Subject,
				Issuer:       c.Issuer,
				SerialNumber: c.SerialNumber,
				Verify:       c.Verify,
				NotBefore:    nb,
				NotAfter:     na,
				Error:        c.Error,
			})
		}
		return jsonResult(list)
	})

	// 7. get_resource_monitor_list
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_resource_monitor_list",
		Description: "Get historical system resource metrics (CPU, Memory, Swap, Disk, Load)",
	}, func(ctx context.Context, req *mcp.CallToolRequest, _ any) (*mcp.CallToolResult, any, error) {
		list := make([]mcpResourceMonitorEnt, 0)
		if s.monitor != nil {
			history := s.monitor.GetData()
			skip := 1
			if len(history) > 120 {
				skip = len(history) / 60
			}
			for i, m := range history {
				if i%skip != 0 {
					continue
				}
				list = append(list, mcpResourceMonitorEnt{
					Time:   time.Unix(m.Time, 0).Format(time.RFC3339),
					CPU:    fmt.Sprintf("%.02f%%", m.CPU),
					Memory: fmt.Sprintf("%.02f%%", m.Mem),
					Swap:   fmt.Sprintf("%.02f%%", m.Swap),
					Disk:   fmt.Sprintf("%.02f%%", m.Disk),
					Load:   fmt.Sprintf("%.02f", m.Load),
				})
			}
		}
		return jsonResult(list)
	})
}
