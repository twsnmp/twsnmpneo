package polling

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// NetFlowPoller monitors IP traffic flows from NetFlow / IPFIX.
type NetFlowPoller struct {
	store    datastore.DataStore
	logStore *parquet.Store
}

func NewNetFlowPoller(store datastore.DataStore, logStore *parquet.Store) *NetFlowPoller {
	return &NetFlowPoller{
		store:    store,
		logStore: logStore,
	}
}

func (p *NetFlowPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	start := time.Now()
	st := time.Now().Add(-time.Duration(pe.PollInt) * time.Second).UnixNano()
	if pe.Result != nil {
		if lt, ok := pe.Result["lastTime"].(float64); ok && int64(lt) > 0 {
			st = int64(lt)
		}
	}
	et := time.Now().UnixNano()

	// Parse filter key=val pairs (src, dst, ip, port, prot)
	var filterSrc, filterDst, filterIP, filterProt string
	var filterPort int
	if pe.Filter != "" {
		for _, part := range strings.Split(pe.Filter, ",") {
			kv := strings.Split(strings.TrimSpace(part), "=")
			if len(kv) == 2 {
				k := strings.ToLower(strings.TrimSpace(kv[0]))
				v := strings.TrimSpace(kv[1])
				switch k {
				case "src":
					filterSrc = v
				case "dst":
					filterDst = v
				case "ip":
					filterIP = v
				case "prot", "protocol":
					filterProt = strings.ToLower(v)
				case "port":
					filterPort, _ = strconv.Atoi(v)
				}
			}
		}
	}

	var logs []*parquet.ParquetLogRecord
	if p.logStore != nil {
		l, _ := p.logStore.Query(ctx, parquet.LogFilter{
			Type:      "netflow",
			StartTime: st,
			EndTime:   et,
			Limit:     10000,
		})
		logs = l
	}

	type miniFlow struct {
		SrcAddr  string `json:"src_addr"`
		DstAddr  string `json:"dst_addr"`
		SrcPort  int    `json:"src_port"`
		DstPort  int    `json:"dst_port"`
		Protocol string `json:"protocol"`
		Bytes    int64  `json:"bytes"`
		Packets  int64  `json:"packets"`
	}

	var totalBytes, totalPackets int64
	var flowCount int

	for _, rec := range logs {
		var f miniFlow
		_ = json.Unmarshal([]byte(rec.Log), &f)

		if filterSrc != "" && f.SrcAddr != filterSrc {
			continue
		}
		if filterDst != "" && f.DstAddr != filterDst {
			continue
		}
		if filterIP != "" && f.SrcAddr != filterIP && f.DstAddr != filterIP {
			continue
		}
		if filterPort > 0 && f.SrcPort != filterPort && f.DstPort != filterPort {
			continue
		}
		if filterProt != "" && strings.ToLower(f.Protocol) != filterProt {
			continue
		}

		flowCount++
		totalBytes += f.Bytes
		totalPackets += f.Packets
	}

	pollSec := float64(pe.PollInt)
	if pollSec <= 0 {
		pollSec = 60
	}
	bps := float64(totalBytes*8) / pollSec
	pps := float64(totalPackets) / pollSec

	fields := map[string]interface{}{
		"bps":      bps,
		"pps":      pps,
		"bytes":    float64(totalBytes),
		"packets":  float64(totalPackets),
		"count":    float64(flowCount),
		"flows":    float64(flowCount),
		"lastTime": float64(et),
		"rtt":      float64(time.Since(start).Nanoseconds()),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	_ = vm.Set("bps", bps)
	_ = vm.Set("pps", pps)
	_ = vm.Set("bytes", float64(totalBytes))
	_ = vm.Set("packets", float64(totalPackets))
	_ = vm.Set("count", float64(flowCount))
	_ = vm.Set("interval", pollSec)
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	if pe.Script == "" {
		return &Result{
			State:   StateNormal,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("netflow ok: %.1f bps, %.1f pps (%d flows)", bps, pps, flowCount),
			Fields:  fields,
		}, nil
	}

	val, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{State: StateHigh, Message: fmt.Sprintf("netflow script error: %v", err), Fields: fields}, nil
	}
	pass, _ := val.ToBoolean()
	if pass {
		return &Result{State: StateNormal, RTT: time.Since(start), Message: fmt.Sprintf("netflow script passed: %.1f bps", bps), Fields: fields}, nil
	}
	return &Result{State: failureState(pe.Level), RTT: time.Since(start), Message: fmt.Sprintf("netflow threshold breached: %.1f bps", bps), Fields: fields}, nil
}
