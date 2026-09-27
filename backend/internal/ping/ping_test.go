package ping

import (
	"testing"
	"time"
)

func TestDoPing_Loopback(t *testing.T) {
	pe := DoPing("127.0.0.1", 1, 1, 64, 64)
	if pe.Stat != PingOK {
		t.Logf("Ping to 127.0.0.1 failed (might require root/privilege in container/env): stat=%v, err=%v", pe.Stat, pe.Error)
	} else {
		if pe.Time <= 0 {
			t.Errorf("expected positive RTT, got %d", pe.Time)
		}
	}
}

func TestDoPing_NonExistent(t *testing.T) {
	// 192.0.2.1 is TEST-NET-1 (RFC 5737), guaranteed to not respond
	start := time.Now()
	pe := DoPing("192.0.2.1", 1, 0, 64, 64)
	elapsed := time.Since(start)
	_ = elapsed

	if pe.Stat == PingOK {
		t.Errorf("expected non-existent IP to fail ping, but got PingOK!")
	}
	if pe.Stat != PingTimeout && pe.Stat != PingOtherError {
		t.Errorf("unexpected stat for non-existent IP: %v", pe.Stat)
	}
}

func TestResolveHostname(t *testing.T) {
	pe := DoPing("localhost", 1, 1, 64, 64)
	if pe.Stat != PingOK {
		t.Logf("Ping to localhost (may require root/permission): Stat=%v, Error=%v", pe.Stat, pe.Error)
	} else if pe.Time <= 0 {
		t.Errorf("expected positive RTT for localhost, got %d", pe.Time)
	}
}
