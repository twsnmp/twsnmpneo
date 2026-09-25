package api

import (
	"fmt"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/gosnmp/gosnmp"
	"github.com/labstack/echo/v4"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

type HrSystem struct {
	Index int    `json:"Index"`
	Key   string `json:"Key"`
	Value string `json:"Value"`
}

type HrStorage struct {
	Index string  `json:"Index"`
	Type  string  `json:"Type"`
	Descr string  `json:"Descr"`
	Size  int64   `json:"Size"`
	Used  int64   `json:"Used"`
	Unit  int64   `json:"Unit"`
	Rate  float64 `json:"Rate"`
}

type HrDevice struct {
	Index  string `json:"Index"`
	Type   string `json:"Type"`
	Descr  string `json:"Descr"`
	Status string `json:"Status"`
	Errors string `json:"Errors"`
}

type HrFileSystem struct {
	Index    string `json:"Index"`
	Type     string `json:"Type"`
	Mount    string `json:"Mount"`
	Remote   string `json:"Remote"`
	Bootable int64  `json:"Bootable"`
	Access   int64  `json:"Access"`
}

type HrProcess struct {
	PID    string `json:"PID"`
	Name   string `json:"Name"`
	Type   string `json:"Type"`
	Status string `json:"Status"`
	Path   string `json:"Path"`
	Param  string `json:"Param"`
	CPU    int64  `json:"CPU"`
	Mem    int64  `json:"Mem"`
}

type HostResourceEnt struct {
	System     []*HrSystem     `json:"System"`
	Storage    []*HrStorage    `json:"Storage"`
	Device     []*HrDevice     `json:"Device"`
	FileSystem []*HrFileSystem `json:"FileSystem"`
	Process    []*HrProcess    `json:"Process"`
}

type VPanelPortEnt struct {
	Index    int64  `json:"Index"`
	Name     string `json:"Name"`
	State    string `json:"State"` // up, down, off
	Speed    int64  `json:"Speed"`
	InBytes  int64  `json:"InBytes"`
	OutBytes int64  `json:"OutBytes"`
	InError  int64  `json:"InError"`
	OutError int64  `json:"OutError"`
}

func getSNMPAgent(n *datastore.NodeEnt) *gosnmp.GoSNMP {
	if n == nil || n.IP == "" {
		return nil
	}
	snmpMode := strings.ToLower(n.SnmpMode)
	if snmpMode == "" || snmpMode == "none" {
		return nil
	}

	port := uint16(n.SnmpPort)
	if port == 0 {
		port = 161
	}

	community := n.Community
	if community == "" && !strings.HasPrefix(snmpMode, "v3") {
		community = "public"
	}

	agent := &gosnmp.GoSNMP{
		Target:    n.IP,
		Port:      port,
		Transport: "udp",
		Community: community,
		Version:   gosnmp.Version2c,
		Timeout:   2 * time.Second,
		Retries:   1,
		MaxOids:   gosnmp.MaxOids,
	}

	if snmpMode == "v1" {
		agent.Version = gosnmp.Version1
	} else if strings.HasPrefix(snmpMode, "v3") {
		agent.Version = gosnmp.Version3
		agent.SecurityModel = gosnmp.UserSecurityModel
		switch snmpMode {
		case "v3auth":
			agent.MsgFlags = gosnmp.AuthNoPriv
			agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
				UserName:                 n.User,
				AuthenticationProtocol:   gosnmp.SHA,
				AuthenticationPassphrase: n.Password,
			}
		case "v3authpriv", "v3authprivex":
			agent.MsgFlags = gosnmp.AuthPriv
			agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
				UserName:                 n.User,
				AuthenticationProtocol:   gosnmp.SHA,
				AuthenticationPassphrase: n.Password,
				PrivacyProtocol:          gosnmp.AES,
				PrivacyPassphrase:        n.Password,
			}
		default:
			agent.MsgFlags = gosnmp.NoAuthNoPriv
			agent.SecurityParameters = &gosnmp.UsmSecurityParameters{
				UserName: n.User,
			}
		}
	}
	return agent
}

func getMIBStringVal(v any) string {
	switch val := v.(type) {
	case string:
		return val
	case []byte:
		return string(val)
	default:
		return fmt.Sprintf("%v", val)
	}
}

func getDeviceStatusName(s int64) string {
	switch s {
	case 1:
		return "unknown"
	case 2:
		return "running"
	case 3:
		return "warning"
	case 4:
		return "testing"
	case 5:
		return "down"
	}
	return "unknown"
}

func getSWRunTypeName(t int64) string {
	switch t {
	case 1:
		return "unknown"
	case 2:
		return "operatingSystem"
	case 3:
		return "deviceDriver"
	case 4:
		return "application"
	}
	return "unknown"
}

func getSWRunStatusName(s int64) string {
	switch s {
	case 1:
		return "running"
	case 2:
		return "runnable"
	case 3:
		return "notRunnable"
	case 4:
		return "invalid"
	}
	return "unknown"
}

func queryNodeHostResource(n *datastore.NodeEnt) (*HostResourceEnt, error) {
	agent := getSNMPAgent(n)
	if agent == nil {
		return nil, fmt.Errorf("SNMP is not configured or disabled for this node")
	}

	err := agent.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SNMP agent: %w", err)
	}
	defer agent.Conn.Close()

	hr := &HostResourceEnt{
		System:     []*HrSystem{},
		Storage:    []*HrStorage{},
		Device:     []*HrDevice{},
		FileSystem: []*HrFileSystem{},
		Process:    []*HrProcess{},
	}

	storageMap := make(map[string]*HrStorage)
	deviceMap := make(map[string]*HrDevice)
	fsMap := make(map[string]*HrFileSystem)
	procMap := make(map[string]*HrProcess)

	var nCPU int
	var hrProcessorLoad int64

	hostOID := mib.NameToOID("host")
	if hostOID == "" {
		hostOID = ".1.3.6.1.2.1.25"
	}

	walkErr := agent.Walk(hostOID, func(variable gosnmp.SnmpPDU) error {
		rawName := mib.OIDToName(variable.Name)
		a := strings.SplitN(rawName, ".", 2)
		if len(a) != 2 {
			return nil
		}
		idx := a[1]

		switch a[0] {
		case "hrSystemUptime":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemUptime",
				Value: fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64()),
				Index: 1,
			})
		case "hrSystemDate":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemDate",
				Value: getMIBStringVal(variable.Value),
				Index: 2,
			})
		case "hrSystemInitialLoadDevice":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemInitialLoadDevice",
				Value: getMIBStringVal(variable.Value),
				Index: 3,
			})
		case "hrSystemInitialLoadParameters":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemInitialLoadParameters",
				Value: getMIBStringVal(variable.Value),
				Index: 4,
			})
		case "hrSystemNumUsers":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemNumUsers",
				Value: fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64()),
				Index: 5,
			})
		case "hrSystemProcesses":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemProcesses",
				Value: fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64()),
				Index: 6,
			})
		case "hrSystemMaxProcesses":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrSystemMaxProcesses",
				Value: fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64()),
				Index: 7,
			})
		case "hrMemorySize":
			hr.System = append(hr.System, &HrSystem{
				Key:   "hrMemorySize",
				Value: fmt.Sprintf("%d", gosnmp.ToBigInt(variable.Value).Int64()),
				Index: 8,
			})
		case "hrProcessorLoad":
			hrProcessorLoad += gosnmp.ToBigInt(variable.Value).Int64()
			nCPU++
		case "hrStorageType":
			s := storageMap[idx]
			if s == nil {
				s = &HrStorage{Index: idx}
				storageMap[idx] = s
			}
			s.Type = mib.OIDToName(getMIBStringVal(variable.Value))
		case "hrStorageDescr":
			s := storageMap[idx]
			if s == nil {
				s = &HrStorage{Index: idx}
				storageMap[idx] = s
			}
			s.Descr = getMIBStringVal(variable.Value)
		case "hrStorageSize":
			s := storageMap[idx]
			if s == nil {
				s = &HrStorage{Index: idx}
				storageMap[idx] = s
			}
			s.Size = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrStorageUsed":
			s := storageMap[idx]
			if s == nil {
				s = &HrStorage{Index: idx}
				storageMap[idx] = s
			}
			s.Used = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrStorageAllocationUnits":
			s := storageMap[idx]
			if s == nil {
				s = &HrStorage{Index: idx}
				storageMap[idx] = s
			}
			s.Unit = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrDeviceType":
			d := deviceMap[idx]
			if d == nil {
				d = &HrDevice{Index: idx}
				deviceMap[idx] = d
			}
			d.Type = mib.OIDToName(getMIBStringVal(variable.Value))
		case "hrDeviceDescr":
			d := deviceMap[idx]
			if d == nil {
				d = &HrDevice{Index: idx}
				deviceMap[idx] = d
			}
			d.Descr = getMIBStringVal(variable.Value)
		case "hrDeviceStatus":
			d := deviceMap[idx]
			if d == nil {
				d = &HrDevice{Index: idx}
				deviceMap[idx] = d
			}
			d.Status = getDeviceStatusName(gosnmp.ToBigInt(variable.Value).Int64())
		case "hrDeviceErrors":
			d := deviceMap[idx]
			if d == nil {
				d = &HrDevice{Index: idx}
				deviceMap[idx] = d
			}
			d.Errors = getMIBStringVal(variable.Value)
		case "hrFSMountPoint":
			f := fsMap[idx]
			if f == nil {
				f = &HrFileSystem{Index: idx}
				fsMap[idx] = f
			}
			f.Mount = getMIBStringVal(variable.Value)
		case "hrFSRemoteMountPoint":
			f := fsMap[idx]
			if f == nil {
				f = &HrFileSystem{Index: idx}
				fsMap[idx] = f
			}
			f.Remote = getMIBStringVal(variable.Value)
		case "hrFSType":
			f := fsMap[idx]
			if f == nil {
				f = &HrFileSystem{Index: idx}
				fsMap[idx] = f
			}
			f.Type = mib.OIDToName(getMIBStringVal(variable.Value))
		case "hrFSAccess":
			f := fsMap[idx]
			if f == nil {
				f = &HrFileSystem{Index: idx}
				fsMap[idx] = f
			}
			f.Access = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrFSBootable":
			f := fsMap[idx]
			if f == nil {
				f = &HrFileSystem{Index: idx}
				fsMap[idx] = f
			}
			f.Bootable = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrSWRunName":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Name = getMIBStringVal(variable.Value)
		case "hrSWRunType":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Type = getSWRunTypeName(gosnmp.ToBigInt(variable.Value).Int64())
		case "hrSWRunStatus":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Status = getSWRunStatusName(gosnmp.ToBigInt(variable.Value).Int64())
		case "hrSWRunPath":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Path = getMIBStringVal(variable.Value)
		case "hrSWRunParameters":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Param = getMIBStringVal(variable.Value)
		case "hrSWRunPerfCPU":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.CPU = gosnmp.ToBigInt(variable.Value).Int64()
		case "hrSWRunPerfMem":
			p := procMap[idx]
			if p == nil {
				p = &HrProcess{PID: idx}
				procMap[idx] = p
			}
			p.Mem = gosnmp.ToBigInt(variable.Value).Int64()
		}
		return nil
	})

	if walkErr != nil && len(hr.System) == 0 && len(storageMap) == 0 && len(procMap) == 0 {
		return nil, walkErr
	}

	if nCPU > 0 {
		hr.System = append(hr.System, &HrSystem{
			Key:   "hrProcessorLoad",
			Value: fmt.Sprintf("%.2f", float64(hrProcessorLoad)/float64(nCPU)),
			Index: 9,
		})
		hr.System = append(hr.System, &HrSystem{
			Key:   "hrProcessorCount",
			Value: fmt.Sprintf("%d", nCPU),
			Index: 10,
		})
	}

	for _, s := range storageMap {
		if s.Unit > 0 {
			s.Size *= s.Unit
			s.Used *= s.Unit
			if s.Size > 0 {
				s.Rate = 100.0 * float64(s.Used) / float64(s.Size)
			}
		}
		hr.Storage = append(hr.Storage, s)
	}
	for _, p := range procMap {
		hr.Process = append(hr.Process, p)
	}
	for _, d := range deviceMap {
		hr.Device = append(hr.Device, d)
	}
	for _, f := range fsMap {
		hr.FileSystem = append(hr.FileSystem, f)
	}

	return hr, nil
}

func queryNodePorts(n *datastore.NodeEnt) ([]*VPanelPortEnt, error) {
	agent := getSNMPAgent(n)
	if agent == nil {
		return nil, fmt.Errorf("SNMP is not configured or disabled for this node")
	}

	err := agent.Connect()
	if err != nil {
		return nil, fmt.Errorf("failed to connect to SNMP agent: %w", err)
	}
	defer agent.Conn.Close()

	ifMap := make(map[int64]*VPanelPortEnt)

	ifOID := mib.NameToOID("ifTable")
	if ifOID == "" {
		ifOID = ".1.3.6.1.2.1.2.2"
	}

	walkErr := agent.Walk(ifOID, func(variable gosnmp.SnmpPDU) error {
		rawName := mib.OIDToName(variable.Name)
		a := strings.SplitN(rawName, ".", 2)
		if len(a) != 2 {
			return nil
		}
		idx, err := strconv.ParseInt(a[1], 10, 64)
		if err != nil {
			return nil
		}
		e := ifMap[idx]
		if e == nil {
			e = &VPanelPortEnt{Index: idx, Name: fmt.Sprintf("Port %d", idx), State: "down"}
			ifMap[idx] = e
		}

		switch a[0] {
		case "ifDescr":
			e.Name = getMIBStringVal(variable.Value)
		case "ifOperStatus":
			st := gosnmp.ToBigInt(variable.Value).Int64()
			if st == 1 {
				e.State = "up"
			} else {
				e.State = "down"
			}
		case "ifSpeed":
			e.Speed = gosnmp.ToBigInt(variable.Value).Int64()
		case "ifInOctets":
			e.InBytes = gosnmp.ToBigInt(variable.Value).Int64()
		case "ifOutOctets":
			e.OutBytes = gosnmp.ToBigInt(variable.Value).Int64()
		case "ifInErrors":
			e.InError = gosnmp.ToBigInt(variable.Value).Int64()
		case "ifOutErrors":
			e.OutError = gosnmp.ToBigInt(variable.Value).Int64()
		}
		return nil
	})

	if walkErr != nil && len(ifMap) == 0 {
		return nil, walkErr
	}

	ports := make([]*VPanelPortEnt, 0, len(ifMap))
	for _, p := range ifMap {
		ports = append(ports, p)
	}
	sort.Slice(ports, func(i, j int) bool {
		return ports[i].Index < ports[j].Index
	})
	return ports, nil
}

func registerSNMPDetailEndpoints(apiGroup *echo.Group, store datastore.DataStore) {
	apiGroup.GET("/nodes/:id/hostresource", func(c echo.Context) error {
		id := c.Param("id")
		node, err := store.GetNode(c.Request().Context(), id)
		if err != nil || node == nil {
			return c.JSON(http.StatusNotFound, map[string]any{
				"supported": false,
				"error":     "node not found",
			})
		}

		snmpMode := strings.ToLower(node.SnmpMode)
		if snmpMode == "" || snmpMode == "none" {
			return c.JSON(http.StatusOK, map[string]any{
				"supported": false,
				"reason":    "snmp_not_configured",
				"message":   "SNMP is not configured for this node",
			})
		}

		hr, qErr := queryNodeHostResource(node)
		if qErr != nil {
			return c.JSON(http.StatusOK, map[string]any{
				"supported": true,
				"error":     qErr.Error(),
				"data":      nil,
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"supported": true,
			"data":      hr,
		})
	})

	apiGroup.GET("/nodes/:id/ports", func(c echo.Context) error {
		id := c.Param("id")
		node, err := store.GetNode(c.Request().Context(), id)
		if err != nil || node == nil {
			return c.JSON(http.StatusNotFound, map[string]any{
				"supported": false,
				"error":     "node not found",
			})
		}

		snmpMode := strings.ToLower(node.SnmpMode)
		if snmpMode == "" || snmpMode == "none" {
			return c.JSON(http.StatusOK, map[string]any{
				"supported": false,
				"reason":    "snmp_not_configured",
				"message":   "SNMP is not configured for this node",
				"ports":     []*VPanelPortEnt{},
			})
		}

		ports, qErr := queryNodePorts(node)
		if qErr != nil {
			return c.JSON(http.StatusOK, map[string]any{
				"supported": true,
				"error":     qErr.Error(),
				"ports":     []*VPanelPortEnt{},
			})
		}

		return c.JSON(http.StatusOK, map[string]any{
			"supported": true,
			"ports":     ports,
		})
	})
}
