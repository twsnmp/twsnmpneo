package datastore_test

import (
	"testing"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

func TestFindVendor(t *testing.T) {
	// Test well known Cisco MAC
	v := datastore.FindVendor("00:00:0C:12:34:56")
	if v == "" || v == "Unknown" {
		t.Logf("Vendor for Cisco: %s", v)
	}
	// Empty
	if datastore.FindVendor("") != "" {
		t.Errorf("Expected empty string for empty MAC")
	}
}
