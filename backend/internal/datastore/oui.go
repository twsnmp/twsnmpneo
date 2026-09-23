package datastore

import (
	"bytes"
	_ "embed"
	"encoding/csv"
	"encoding/hex"
	"io"
	"strings"
	"sync"
)

//go:embed conf/mac-vendors-export.csv
var defaultOUICsv []byte

var (
	ouiMap  = make(map[string]string)
	ouiOnce sync.Once
)

func initOUI() {
	if len(defaultOUICsv) > 0 {
		loadOUIMap(bytes.NewReader(defaultOUICsv))
	}
}

// LoadOUIMap loads OUI data from an io.Reader.
func LoadOUIMap(r io.Reader) {
	loadOUIMap(r)
}

func loadOUIMap(f io.Reader) {
	r := csv.NewReader(f)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		if err != nil || len(record) < 2 {
			continue
		}
		oui := record[0]
		if !strings.Contains(oui, ":") {
			continue
		}
		oui = strings.TrimSpace(oui)
		oui = strings.ReplaceAll(oui, ":", "")
		ouiMap[strings.ToUpper(oui)] = record[1]
	}
}

// FindVendor finds vendor name from MAC address.
func FindVendor(mac string) string {
	ouiOnce.Do(initOUI)
	if mac == "" {
		return ""
	}
	mac = strings.TrimSpace(mac)
	mac = strings.ReplaceAll(mac, ":", "")
	mac = strings.ReplaceAll(mac, "-", "")
	if len(mac) >= 6 {
		mac = strings.ToUpper(mac)
		if n, ok := ouiMap[mac[:6]]; ok {
			return n
		}
		if len(mac) >= 7 {
			if n, ok := ouiMap[mac[:7]]; ok {
				return n
			}
		}
		if len(mac) >= 9 {
			if n, ok := ouiMap[mac[:9]]; ok {
				return n
			}
		}
		if h, err := hex.DecodeString(mac); err == nil && len(h) > 0 {
			if (h[0] & 0x02) == 0x02 {
				h[0] = h[0] & 0xfd
				mac = strings.ToUpper(hex.EncodeToString(h))
				if len(mac) >= 6 {
					if n, ok := ouiMap[mac[:6]]; ok {
						return n + "(Local)"
					}
				}
				if len(mac) >= 7 {
					if n, ok := ouiMap[mac[:7]]; ok {
						return n + "(Local)"
					}
				}
				if len(mac) >= 9 {
					if n, ok := ouiMap[mac[:9]]; ok {
						return n + "(Local)"
					}
				}
				return "Local"
			}
		}
	}
	return "Unknown"
}
