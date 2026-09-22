package datastore

import (
	"net"
	"testing"
)

func TestIsPrivateIP(t *testing.T) {
	tests := []struct {
		ip       string
		expected bool
	}{
		{"192.168.1.1", true},
		{"10.0.0.1", true},
		{"172.16.0.1", true},
		{"127.0.0.1", true},
		{"169.254.1.1", true},
		{"8.8.8.8", false},
		{"1.1.1.1", false},
		{"::1", true},
		{"fe80::1", true},
		{"2001:4860:4847:400::", false},
	}

	for _, tt := range tests {
		ip := net.ParseIP(tt.ip)
		actual := IsPrivateIP(ip)
		if actual != tt.expected {
			t.Errorf("IsPrivateIP(%s) = %v, expected %v", tt.ip, actual, tt.expected)
		}
	}
}

func TestGetLocDefault(t *testing.T) {
	// Without GeoIP database loaded
	CloseGeoIP()

	loc := GetLoc("192.168.1.1")
	if loc != "LOCAL,0,0," {
		t.Errorf("GetLoc(192.168.1.1) = %q, expected 'LOCAL,0,0,'", loc)
	}

	locGlobal := GetLoc("8.8.8.8")
	if locGlobal != "" {
		t.Errorf("GetLoc(8.8.8.8) without DB = %q, expected empty string", locGlobal)
	}
}
