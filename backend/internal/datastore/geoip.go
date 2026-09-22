package datastore

import (
	"fmt"
	"io"
	"log/slog"
	"net"
	"os"
	"path/filepath"
	"sync"

	"github.com/oschwald/geoip2-golang"
)

var (
	geoipMu      sync.RWMutex
	geoipReader  *geoip2.Reader
	geoipMap     sync.Map
	geoipVersion string

	privateIPBlocks []*net.IPNet
	privateIPOnce   sync.Once
)

func initPrivateIPBlocks() {
	cidrs := []string{
		"10.0.0.0/8",     // RFC1918
		"172.16.0.0/12",  // RFC1918
		"192.168.0.0/16", // RFC1918
		"127.0.0.0/8",     // Loopback
		"169.254.0.0/16", // Link-local
		"::1/128",        // IPv6 Loopback
		"fe80::/10",      // IPv6 Link-local
		"fc00::/7",       // IPv6 Unique Local
	}
	for _, cidr := range cidrs {
		_, block, err := net.ParseCIDR(cidr)
		if err == nil {
			privateIPBlocks = append(privateIPBlocks, block)
		}
	}
}

// IsPrivateIP checks if an IP address belongs to private/link-local/loopback spaces.
func IsPrivateIP(ip net.IP) bool {
	if ip == nil {
		return false
	}
	if !ip.IsGlobalUnicast() {
		return true
	}
	privateIPOnce.Do(initPrivateIPBlocks)
	for _, block := range privateIPBlocks {
		if block.Contains(ip) {
			return true
		}
	}
	return false
}

// InitGeoIP initializes GeoIP database from data directory if geoip.mmdb exists.
func InitGeoIP(dataDir string) error {
	p := filepath.Join(dataDir, "geoip.mmdb")
	if _, err := os.Stat(p); err != nil {
		return nil
	}
	return openGeoIP(p)
}

func openGeoIP(path string) error {
	geoipMu.Lock()
	defer geoipMu.Unlock()

	if geoipReader != nil {
		_ = geoipReader.Close()
		geoipReader = nil
	}

	reader, err := geoip2.Open(path)
	if err != nil {
		slog.Warn("Failed to open GeoIP database", "path", path, "error", err)
		geoipVersion = ""
		return err
	}

	geoipReader = reader
	md := reader.Metadata()
	geoipVersion = fmt.Sprintf("%d.%d", md.BinaryFormatMajorVersion, md.BinaryFormatMinorVersion)
	// Clear cache on DB change
	geoipMap.Range(func(key, value any) bool {
		geoipMap.Delete(key)
		return true
	})
	slog.Info("GeoIP database opened successfully", "version", geoipVersion, "path", path)
	return nil
}

// UpdateGeoIP replaces current GeoIP database with uploaded content and reloads it.
func UpdateGeoIP(dataDir string, r io.Reader) error {
	tmpPath := filepath.Join(dataDir, "geoip.mmdb.tmp")
	dstPath := filepath.Join(dataDir, "geoip.mmdb")

	f, err := os.Create(tmpPath)
	if err != nil {
		return fmt.Errorf("create temp geoip file: %w", err)
	}

	if _, err := io.Copy(f, r); err != nil {
		_ = f.Close()
		_ = os.Remove(tmpPath)
		return fmt.Errorf("write geoip file: %w", err)
	}
	if err := f.Close(); err != nil {
		_ = os.Remove(tmpPath)
		return err
	}

	// Verify reader can open the temp file
	testReader, err := geoip2.Open(tmpPath)
	if err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("invalid geoip mmdb file: %w", err)
	}
	_ = testReader.Close()

	// Close existing reader before replacing file
	CloseGeoIP()

	if err := os.Rename(tmpPath, dstPath); err != nil {
		_ = os.Remove(tmpPath)
		return fmt.Errorf("rename geoip file: %w", err)
	}

	return openGeoIP(dstPath)
}

// DeleteGeoIP removes the GeoIP database and closes the reader.
func DeleteGeoIP(dataDir string) error {
	CloseGeoIP()
	dstPath := filepath.Join(dataDir, "geoip.mmdb")
	if err := os.Remove(dstPath); err != nil && !os.IsNotExist(err) {
		return err
	}
	return nil
}

// CloseGeoIP closes the currently open GeoIP reader.
func CloseGeoIP() {
	geoipMu.Lock()
	defer geoipMu.Unlock()

	if geoipReader != nil {
		_ = geoipReader.Close()
		geoipReader = nil
	}
	geoipVersion = ""
	geoipMap.Range(func(key, value any) bool {
		geoipMap.Delete(key)
		return true
	})
}

// GetGeoIPInfo returns GeoIP database version or empty string if not loaded.
func GetGeoIPInfo() string {
	geoipMu.RLock()
	defer geoipMu.RUnlock()
	return geoipVersion
}

// GetLoc resolves location string for a given IP address.
// Matches TWSNMP FC / FK output format.
func GetLoc(sip string) string {
	if sip == "" || sip == "-" {
		return ""
	}
	if v, ok := geoipMap.Load(sip); ok {
		if l, ok := v.(string); ok {
			return l
		}
	}

	loc := ""
	ip := net.ParseIP(sip)
	if ip == nil {
		return ""
	}

	if IsPrivateIP(ip) {
		loc = "LOCAL,0,0,"
	} else {
		geoipMu.RLock()
		reader := geoipReader
		geoipMu.RUnlock()

		if reader != nil {
			record, err := reader.City(ip)
			if err == nil && record.Country.IsoCode != "" {
				loc = fmt.Sprintf("%s,%f,%f,%s", record.Country.IsoCode, record.Location.Latitude, record.Location.Longitude, record.City.Names["en"])
			} else {
				loc = "LOCAL,0,0,"
			}
		} else {
			// No GeoIP DB loaded, but is global unicast
			loc = ""
		}
	}

	geoipMap.Store(sip, loc)
	return loc
}
