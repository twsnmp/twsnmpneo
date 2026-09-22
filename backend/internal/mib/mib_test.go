package mib_test

import (
	"os"
	"strings"
	"testing"

	"github.com/gosnmp/gosnmp"
	"github.com/twsnmp/twsnmpneo/backend/internal/mib"
)

func TestMIB_InitAndResolutions(t *testing.T) {
	tempDir, err := os.MkdirTemp("", "mibtest_*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tempDir)

	if err := mib.Init(tempDir); err != nil {
		t.Fatalf("mib.Init failed: %v", err)
	}

	modules := mib.GetMIBModules()
	if len(modules) == 0 {
		t.Fatalf("expected loaded MIB modules, got 0")
	}

	tree := mib.GetMIBTree()
	if len(tree) == 0 {
		t.Fatalf("expected MIB tree entries, got 0")
	}

	// Test OIDToName
	tests := []struct {
		oid      string
		expected string
	}{
		{".1.3.6.1.2.1.1.3.0", "sysUpTimeInstance"},
		{".1.3.6.1.6.3.1.1.5.3", "linkDown"},
		{".1.3.6.1.6.3.1.1.5.1", "coldStart"},
		{".1.3.6.1.2.1.2.2.1.7.5", "ifAdminStatus.5"},
		{".1.3.6.1.2.1.2.2.1.8.5", "ifOperStatus.5"},
		{".1.3.6.1.6.3.1.1.4.1.0", "snmpTrapOID.0"},
	}

	for _, tc := range tests {
		name := mib.OIDToName(tc.oid)
		if name != tc.expected {
			t.Errorf("OIDToName(%s) = %s; want %s", tc.oid, name, tc.expected)
		}
	}

	// Test NameToOID
	ifAdminOID := mib.NameToOID("ifAdminStatus.5")
	if ifAdminOID != ".1.3.6.1.2.1.2.2.1.7.5" {
		t.Errorf("NameToOID(ifAdminStatus.5) = %s; want .1.3.6.1.2.1.2.2.1.7.5", ifAdminOID)
	}

	// Test GetMIBValueString
	pduTimeTicks := &gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.2.1.1.3.0",
		Type:  gosnmp.TimeTicks,
		Value: uint32(244552700),
	}
	valTimeTicks := mib.GetMIBValueString("sysUpTimeInstance", pduTimeTicks, false)
	if valTimeTicks != "244552700(28 days, 7h18m47s)" {
		t.Errorf("GetMIBValueString TimeTicks = %s; want 244552700(28 days, 7h18m47s)", valTimeTicks)
	}

	// Test Enum format
	pduAdmin := &gosnmp.SnmpPDU{
		Name:  ".1.3.6.1.2.1.2.2.1.7.5",
		Type:  gosnmp.Integer,
		Value: 1,
	}
	valAdmin := mib.GetMIBValueString("ifAdminStatus.5", pduAdmin, false)
	if valAdmin != "1(up)" {
		t.Errorf("GetMIBValueString ifAdminStatus.5 = %s; want 1(up)", valAdmin)
	}

	// Test DecodeTrap SNMPv2c
	trapPacket := &gosnmp.SnmpPacket{
		Version: gosnmp.Version2c,
		Variables: []gosnmp.SnmpPDU{
			{
				Name:  ".1.3.6.1.2.1.1.3.0",
				Type:  gosnmp.TimeTicks,
				Value: uint32(244552700),
			},
			{
				Name:  ".1.3.6.1.6.3.1.1.4.1.0",
				Type:  gosnmp.ObjectIdentifier,
				Value: ".1.3.6.1.6.3.1.1.5.3",
			},
			{
				Name:  ".1.3.6.1.2.1.2.2.1.1.5",
				Type:  gosnmp.Integer,
				Value: 5,
			},
			{
				Name:  ".1.3.6.1.2.1.2.2.1.7.5",
				Type:  gosnmp.Integer,
				Value: 1,
			},
			{
				Name:  ".1.3.6.1.2.1.2.2.1.8.5",
				Type:  gosnmp.Integer,
				Value: 2,
			},
		},
	}

	trapType, vars, from := mib.DecodeTrap(trapPacket, "192.168.1.240", "CoreSwitch")
	if trapType != "linkDown" {
		t.Errorf("DecodeTrap trapType = %s; want linkDown", trapType)
	}
	if from != "192.168.1.240(CoreSwitch)" {
		t.Errorf("DecodeTrap from = %s; want 192.168.1.240(CoreSwitch)", from)
	}
	expectedSub := "ifAdminStatus.5=1(up)"
	if !strings.Contains(vars, expectedSub) {
		t.Errorf("DecodeTrap vars = %s; want to contain %s", vars, expectedSub)
	}
	expectedSub2 := "ifOperStatus.5=2(down)"
	if !strings.Contains(vars, expectedSub2) {
		t.Errorf("DecodeTrap vars = %s; want to contain %s", vars, expectedSub2)
	}
}
