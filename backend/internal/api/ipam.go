package api

import (
	"context"
	"encoding/binary"
	"fmt"
	"net"
	"sort"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// IPAMSubnetBlock represents a /24 subnet block within a larger range.
type IPAMSubnetBlock struct {
	Subnet string  `json:"Subnet"`
	Size   int64   `json:"Size"`
	Used   int64   `json:"Used"`
	Usage  float64 `json:"Usage"`
}

// IPAMRangeEnt represents the IPAM utilization report for an address range.
type IPAMRangeEnt struct {
	Range   string             `json:"Range"`
	StartIP string             `json:"StartIP"`
	EndIP   string             `json:"EndIP"`
	Size    int64              `json:"Size"`
	Used    int64              `json:"Used"`
	Usage   float64            `json:"Usage"`
	UsedIP  []int              `json:"UsedIP"` // 100 slots for heatmap (0..99%)
	Subnets []*IPAMSubnetBlock `json:"Subnets,omitempty"`
}

// IPAMReportResp represents the aggregate IPAM response.
type IPAMReportResp struct {
	Ranges      []*IPAMRangeEnt `json:"Ranges"`
	TotalRanges int             `json:"TotalRanges"`
	TotalSize   int64           `json:"TotalSize"`
	TotalUsed   int64           `json:"TotalUsed"`
	TotalUsage  float64         `json:"TotalUsage"`
}

func ip2uint32(ip net.IP) uint32 {
	if len(ip) == 16 {
		return binary.BigEndian.Uint32(ip[12:16])
	}
	return binary.BigEndian.Uint32(ip)
}

func uint32toip(n uint32) net.IP {
	ip := make(net.IP, 4)
	binary.BigEndian.PutUint32(ip, n)
	return ip
}

// calculateIPAM generates the IPAM report given datastore and map configuration.
func calculateIPAM(ctx context.Context, store datastore.DataStore) (*IPAMReportResp, error) {
	mapConf, err := store.GetMapConf(ctx)
	if err != nil {
		mapConf = &datastore.MapConfEnt{}
	}

	arpEntries, _ := store.LoadArpTable(ctx)
	nodes, _ := store.ListNodes(ctx)

	now := time.Now().Unix()
	timeoutSec := int64(mapConf.ArpTimeout * 3600)

	// Collect used IPs: uint32 -> struct
	usedIPs := make(map[uint32]string) // uint32 -> node name or MAC

	for _, n := range nodes {
		if n.IP != "" {
			if ip := net.ParseIP(n.IP).To4(); ip != nil {
				usedIPs[ip2uint32(ip)] = n.Name
			}
		}
	}

	for _, a := range arpEntries {
		if timeoutSec > 0 && now-a.LastTime > timeoutSec {
			continue
		}
		if ip := net.ParseIP(a.IP).To4(); ip != nil {
			u := ip2uint32(ip)
			if _, exists := usedIPs[u]; !exists {
				usedIPs[u] = a.MAC
			}
		}
	}

	// Parse configured ranges
	rangeStrs := []string{}
	if mapConf.ArpWatchRange != "" {
		for _, r := range strings.Split(mapConf.ArpWatchRange, ",") {
			r = strings.TrimSpace(r)
			if r != "" {
				rangeStrs = append(rangeStrs, r)
			}
		}
	}

	// Fallback: If no range configured, infer /24 subnets from used IPs
	if len(rangeStrs) == 0 {
		subnetSet := make(map[string]bool)
		for u := range usedIPs {
			ip := uint32toip(u)
			subnet := fmt.Sprintf("%d.%d.%d.0/24", ip[0], ip[1], ip[2])
			subnetSet[subnet] = true
		}
		for s := range subnetSet {
			rangeStrs = append(rangeStrs, s)
		}
		sort.Strings(rangeStrs)
		if len(rangeStrs) == 0 {
			rangeStrs = append(rangeStrs, "192.168.1.0/24")
		}
	}

	resp := &IPAMReportResp{
		Ranges: []*IPAMRangeEnt{},
	}

	for _, rStr := range rangeStrs {
		var sIP, eIP uint32
		var validStart, validEnd uint32
		var isCIDR bool

		parts := strings.SplitN(rStr, "-", 2)
		if len(parts) == 1 {
			// CIDR format
			ip, ipnet, err := net.ParseCIDR(rStr)
			if err != nil {
				continue
			}
			ipv4 := ip.To4()
			if ipv4 == nil {
				continue
			}
			isCIDR = true
			mask := binary.BigEndian.Uint32(ipnet.Mask)
			sIP = ip2uint32(ipnet.IP.To4())
			eIP = sIP | (^mask)

			ones, _ := ipnet.Mask.Size()
			if ones < 31 {
				validStart = sIP + 1
				validEnd = eIP - 1
			} else {
				validStart = sIP
				validEnd = eIP
			}
		} else {
			// Range format: A.B.C.D-E.F.G.H
			s := net.ParseIP(strings.TrimSpace(parts[0])).To4()
			e := net.ParseIP(strings.TrimSpace(parts[1])).To4()
			if s == nil || e == nil {
				continue
			}
			sIP = ip2uint32(s)
			eIP = ip2uint32(e)
			if sIP > eIP {
				sIP, eIP = eIP, sIP
			}
			validStart = sIP
			validEnd = eIP
		}

		if validStart > validEnd {
			continue
		}

		size := int64(validEnd - validStart + 1)
		ent := &IPAMRangeEnt{
			Range:   rStr,
			StartIP: uint32toip(validStart).String(),
			EndIP:   uint32toip(validEnd).String(),
			Size:    size,
			UsedIP:  make([]int, 100),
		}

		// Count used IPs in this range
		var usedCount int64
		// Map for /24 block aggregation if size > 256
		blockMap := make(map[uint32]int64) // /24 network uint32 -> used count

		for u := range usedIPs {
			if u >= validStart && u <= validEnd {
				usedCount++
				// Calculate 0..99 percentile slot
				slot := 0
				if size > 1 {
					slot = int((int64(u-validStart) * 100) / size)
					if slot >= 100 {
						slot = 99
					}
					if slot < 0 {
						slot = 0
					}
				}
				ent.UsedIP[slot]++

				if size > 256 {
					blockNet := u & 0xffffff00
					blockMap[blockNet]++
				}
			}
		}

		ent.Used = usedCount
		if size > 0 {
			ent.Usage = (float64(usedCount) * 100.0) / float64(size)
		}

		// If large range, generate /24 subnet blocks
		if size > 256 {
			// Determine /24 blocks within validStart..validEnd
			firstBlock := validStart & 0xffffff00
			lastBlock := validEnd & 0xffffff00
			totalBlocks := int64((lastBlock - firstBlock)/256 + 1)

			// Limit generated blocks to avoid excessive payloads (e.g., max 256 blocks)
			maxBlocks := int64(256)
			step := int64(1)
			if totalBlocks > maxBlocks {
				step = (totalBlocks + maxBlocks - 1) / maxBlocks
			}

			for b := int64(0); b < totalBlocks; b += step {
				blockNet := firstBlock + uint32(b*256)
				blockIP := uint32toip(blockNet)
				bUsed := blockMap[blockNet]

				// Calculate actual size of this /24 within the range
				bStart := blockNet + 1
				bEnd := blockNet + 254
				if bStart < validStart {
					bStart = validStart
				}
				if bEnd > validEnd {
					bEnd = validEnd
				}
				bSize := int64(254)
				if bStart <= bEnd {
					bSize = int64(bEnd - bStart + 1)
				}

				bUsage := 0.0
				if bSize > 0 {
					bUsage = (float64(bUsed) * 100.0) / float64(bSize)
				}

				ent.Subnets = append(ent.Subnets, &IPAMSubnetBlock{
					Subnet: fmt.Sprintf("%d.%d.%d.0/24", blockIP[0], blockIP[1], blockIP[2]),
					Size:   bSize,
					Used:   bUsed,
					Usage:  bUsage,
				})
			}
		} else if isCIDR {
			// For /24 or smaller CIDR
			ent.Subnets = append(ent.Subnets, &IPAMSubnetBlock{
				Subnet: rStr,
				Size:   size,
				Used:   usedCount,
				Usage:  ent.Usage,
			})
		}

		resp.Ranges = append(resp.Ranges, ent)
		resp.TotalSize += size
		resp.TotalUsed += usedCount
	}

	resp.TotalRanges = len(resp.Ranges)
	if resp.TotalSize > 0 {
		resp.TotalUsage = (float64(resp.TotalUsed) * 100.0) / float64(resp.TotalSize)
	}

	return resp, nil
}
