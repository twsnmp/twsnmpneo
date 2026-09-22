package receiver

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"time"

	"github.com/Cistern/sflow"
	"github.com/google/gopacket"
	"github.com/google/gopacket/layers"
	"github.com/tehmaze/netflow/read"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/parquet"
)

// SFlowConfig holds options for sFlow receiver.
type SFlowConfig struct {
	Port     int
	LogStore *parquet.Store
}

// SFlowServer ingests sFlow v5 flow and counter samples over UDP.
type SFlowServer struct {
	port     int
	logStore *parquet.Store
}

// NewSFlowServer creates a new sFlow receiver instance.
func NewSFlowServer(cfg SFlowConfig) *SFlowServer {
	return &SFlowServer{
		port:     cfg.Port,
		logStore: cfg.LogStore,
	}
}

// Start launches the UDP listener on the configured sFlow port.
func (s *SFlowServer) Start(ctx context.Context) error {
	if s.port <= 0 {
		return nil
	}

	addr := fmt.Sprintf(":%d", s.port)
	slog.Info("Starting sFlow receiver", "addr", addr)

	conn, err := net.ListenPacket("udp", addr)
	if err != nil {
		slog.Warn("Failed to start sFlow receiver", "addr", addr, "error", err)
		return fmt.Errorf("listen sflow udp: %w", err)
	}
	defer conn.Close()

	slog.Info("Started sFlow receiver", "addr", addr)
	buf := make([]byte, 65535)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	for {
		n, remoteAddr, err := conn.ReadFrom(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			continue
		}

		fromIP := ""
		if udpAddr, ok := remoteAddr.(*net.UDPAddr); ok {
			fromIP = udpAddr.IP.String()
		}

		s.handlePacket(buf[:n], fromIP)
	}
}

func (s *SFlowServer) handlePacket(data []byte, fromIP string) {
	r := bytes.NewReader(data)
	d := sflow.NewDecoder(r)
	dg, err := d.Decode()
	if err != nil {
		slog.Debug("Failed to decode sflow datagram", "error", err, "from", fromIP)
		return
	}

	for _, sample := range dg.Samples {
		switch smp := sample.(type) {
		case *sflow.CounterSample:
			for _, record := range smp.Records {
				switch record.(type) {
				case sflow.HostDiskCounters:
					s.saveCounter("HostDiskCounter", fromIP, record)
				case sflow.HostCPUCounters:
					s.saveCounter("HostCPUCounter", fromIP, record)
				case sflow.HostMemoryCounters:
					s.saveCounter("HostMemoryCounter", fromIP, record)
				case sflow.HostNetCounters:
					s.saveCounter("HostNetCounter", fromIP, record)
				case sflow.GenericInterfaceCounters:
					s.saveCounter("GenericInterfaceCounter", fromIP, record)
				default:
					slog.Debug("Unknown sflow counter sample", "record", record)
				}
			}
		case *sflow.FlowSample:
			for _, record := range smp.Records {
				switch fsr := record.(type) {
				case sflow.RawPacketFlow:
					s.saveRawPacketFlow(&fsr, 0)
				}
			}
		case *sflow.EventDiscardedPacket:
			for _, record := range smp.Records {
				switch fsr := record.(type) {
				case sflow.RawPacketFlow:
					s.saveRawPacketFlow(&fsr, int(smp.Reason))
				}
			}
		}
	}
}

func (s *SFlowServer) saveRawPacketFlow(r *sflow.RawPacketFlow, reason int) {
	var e datastore.SFlowEnt
	e.Time = time.Now().UnixNano()

	packet := gopacket.NewPacket(r.Header, layers.LayerTypeEthernet, gopacket.Default)
	ethernetLayer := packet.Layer(layers.LayerTypeEthernet)
	if ethernetLayer != nil {
		if eth, ok := ethernetLayer.(*layers.Ethernet); ok {
			e.SrcMAC = eth.SrcMAC.String()
			e.DstMAC = eth.DstMAC.String()
		}
	}

	ipv4Layer := packet.Layer(layers.LayerTypeIPv4)
	if ipv4Layer != nil {
		if ip, ok := ipv4Layer.(*layers.IPv4); ok {
			e.SrcAddr = ip.SrcIP.String()
			e.DstAddr = ip.DstIP.String()
			e.Bytes = int(ip.Length)
		}
	} else {
		ipv6Layer := packet.Layer(layers.LayerTypeIPv6)
		if ipv6Layer != nil {
			if ip6, ok := ipv6Layer.(*layers.IPv6); ok {
				e.SrcAddr = ip6.SrcIP.String()
				e.DstAddr = ip6.DstIP.String()
				e.Bytes = int(ip6.Length)
			}
		}
	}

	// Protocol and Ports
	udpLayer := packet.Layer(layers.LayerTypeUDP)
	if udpLayer != nil {
		if udp, ok := udpLayer.(*layers.UDP); ok {
			e.SrcPort = int(udp.SrcPort)
			e.DstPort = int(udp.DstPort)
			e.Protocol = "udp"
		}
	} else {
		tcpLayer := packet.Layer(layers.LayerTypeTCP)
		if tcpLayer != nil {
			if tcp, ok := tcpLayer.(*layers.TCP); ok {
				e.SrcPort = int(tcp.SrcPort)
				e.DstPort = int(tcp.DstPort)
				var flag uint8
				if tcp.FIN {
					flag |= 0x01
				}
				if tcp.SYN {
					flag |= 0x02
				}
				if tcp.RST {
					flag |= 0x04
				}
				if tcp.PSH {
					flag |= 0x08
				}
				if tcp.ACK {
					flag |= 0x10
				}
				if tcp.URG {
					flag |= 0x20
				}
				if tcp.ECE {
					flag |= 0x40
				}
				if tcp.CWR {
					flag |= 0x80
				}
				e.TCPFlags = read.TCPFlags(flag)
				e.Protocol = "tcp"
			}
		} else {
			icmpV4Layer := packet.Layer(layers.LayerTypeICMPv4)
			if icmpV4Layer != nil {
				if icmp, ok := icmpV4Layer.(*layers.ICMPv4); ok {
					e.Protocol = "icmp"
					e.DstPort = int(icmp.TypeCode)
				}
			} else {
				icmpV6Layer := packet.Layer(layers.LayerTypeICMPv6)
				if icmpV6Layer != nil {
					if icmp6, ok := icmpV6Layer.(*layers.ICMPv6); ok {
						e.Protocol = "icmpv6"
						e.DstPort = int(icmp6.TypeCode)
					}
				}
			}
		}
	}

	e.Reason = reason
	e.SrcLoc = datastore.GetLoc(e.SrcAddr)
	e.DstLoc = datastore.GetLoc(e.DstAddr)

	if s.logStore != nil {
		rawJSON, err := json.Marshal(&e)
		if err != nil {
			slog.Debug("Failed to marshal sflow entry", "error", err)
			return
		}
		_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time: e.Time,
			Type: "sflow",
			Src:  e.SrcAddr,
			Log:  string(rawJSON),
		})
	}
}

func (s *SFlowServer) saveCounter(counterType, remoteIP string, record any) {
	dataBytes, err := json.Marshal(record)
	if err != nil {
		slog.Debug("Failed to marshal sflow counter record", "error", err)
		return
	}

	ent := datastore.SFlowCounterEnt{
		Type:   counterType,
		Remote: remoteIP,
		Data:   string(dataBytes),
	}

	if s.logStore != nil {
		rawJSON, err := json.Marshal(&ent)
		if err != nil {
			slog.Debug("Failed to marshal sflow counter entry", "error", err)
			return
		}
		_ = s.logStore.WriteLog(&parquet.ParquetLogRecord{
			Time: time.Now().UnixNano(),
			Type: "sflowCounter",
			Src:  remoteIP,
			Log:  string(rawJSON),
		})
	}
}
