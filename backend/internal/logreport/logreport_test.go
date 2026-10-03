package logreport

import (
	"context"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore/bbolt"
)

func newStore(t *testing.T) datastore.DataStore {
	t.Helper()
	s, err := bbolt.New(filepath.Join(t.TempDir(), "test.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.Close() })
	return s
}

func rec(tag, content string, ts int64) Record {
	return Record{Time: ts, Host: "host1", Tag: tag, Content: content, Severity: 6}
}

func load[T any](t *testing.T, st datastore.DataStore, kind string) map[string]*T {
	t.Helper()
	m, err := st.ListLogReportData(context.Background(), kind)
	if err != nil {
		t.Fatal(err)
	}
	ret := map[string]*T{}
	for id, b := range m {
		e := new(T)
		if err := json.Unmarshal(b, e); err != nil {
			t.Fatal(err)
		}
		ret[id] = e
	}
	return ret
}

func TestParseRecord(t *testing.T) {
	r, ok := ParseRecord(`{"hostname":"h","tag":"twWifiScan","content":"type=APInfo","severity":5}`, "src", 10)
	if !ok || r.Host != "h" || r.Tag != "twWifiScan" || r.Content != "type=APInfo" || r.Severity != 5 || r.Time != 10 {
		t.Fatalf("unexpected %+v ok=%v", r, ok)
	}
	// RFC5424 style: app_name / message, host from src
	r, ok = ParseRecord(`{"app_name":"twpcap","message":"type=DNS"}`, "1.2.3.4", 1)
	if !ok || r.Host != "1.2.3.4" || r.Tag != "twpcap" || r.Content != "type=DNS" {
		t.Fatalf("unexpected %+v ok=%v", r, ok)
	}
	if _, ok := ParseRecord(`not json`, "", 0); ok {
		t.Fatal("invalid json must fail")
	}
	if _, ok := ParseRecord(`{"hostname":"h"}`, "", 0); ok {
		t.Fatal("record without tag must fail")
	}
}

func TestProcessIgnoresOtherTag(t *testing.T) {
	st := newStore(t)
	s := NewSession(context.Background(), st)
	if s.Process(SourceWifiScan, rec("twpcap", "type=APInfo,bssid=aa:bb:cc:dd:ee:ff", 1)) {
		t.Fatal("tag mismatch must be ignored")
	}
	if s.Process(SourceWifiScan, rec("twWifiScan", "type=Stats", 1)) {
		t.Fatal("unsupported type must be ignored")
	}
	if s.Processed != 0 {
		t.Fatalf("processed=%d", s.Processed)
	}
}

func TestWifiAP(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	s := NewSession(ctx, st)
	c := "type=APInfo,ssid=net,bssid=aa:bb:cc:dd:ee:ff,rssi=-50,Channel=1,info=wpa"
	s.Process(SourceWifiScan, rec("twWifiScan", c, 100))
	// New session: entity must be loaded from the store and updated.
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	s = NewSession(ctx, st)
	s.Process(SourceWifiScan, rec("twWifiScan", "type=APInfo,ssid=net2,bssid=aa:bb:cc:dd:ee:ff,rssi=-60,Channel=1,info=wpa", 200))
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	aps := load[WifiAPEnt](t, st, KindWifiAP)
	if len(aps) != 1 {
		t.Fatalf("aps=%d", len(aps))
	}
	ap := aps["host1:aa:bb:cc:dd:ee:ff"]
	if ap == nil || ap.Count != 2 || ap.Change != 1 || ap.SSID != "net2" || ap.FirstTime != 100 || ap.LastTime != 200 || len(ap.RSSI) != 2 {
		t.Fatalf("unexpected ap %+v", ap)
	}
}

func TestRSSILimit(t *testing.T) {
	st := newStore(t)
	s := NewSession(context.Background(), st)
	for i := 0; i < MaxSeriesSize+10; i++ {
		s.Process(SourceWifiScan, rec("twWifiScan", "type=APInfo,bssid=aa,rssi=-1,Channel=1", int64(i+1)))
	}
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	for _, ap := range load[WifiAPEnt](t, st, KindWifiAP) {
		if len(ap.RSSI) != MaxSeriesSize || ap.Count != MaxSeriesSize+10 {
			t.Fatalf("rssi=%d count=%d", len(ap.RSSI), ap.Count)
		}
	}
}

func TestBlue(t *testing.T) {
	st := newStore(t)
	s := NewSession(context.Background(), st)
	lt := "2024-02-06T06:13:49+09:00"
	s.Process(SourceBlueScan, rec("twBlueScan", "type=Device,address=d7:bb,name=n,rssi=-64,addrType=Public,vendor=V,info=i,ft="+lt+",lt="+lt, 1))
	s.Process(SourceBlueScan, rec("twBlueScan", "type=OMRONEnv,address=e1,name=env,rssi=-60,temp=23.5,hum=40,lx=10,press=1000,sound=30,eTVOC=1,eCO2=400", 2))
	s.Process(SourceBlueScan, rec("twBlueScan", "type=SwitchBotEnv,address=e2,name=sb,rssi=-60,temp=20,hum=50,bat=90,co2=500", 3))
	s.Process(SourceBlueScan, rec("twBlueScan", "type=SwitchBotPlugMini,address=p1,name=plug,rssi=-70,sw=true,over=false,load=125", 4))
	s.Process(SourceBlueScan, rec("twBlueScan", "type=SwitchBotMotionSensor,address=m1,name=,rssi=-64,moving=true,event=report,lastMoveDiff=5,lastMove="+lt+",battery=100,light=false", 5))
	if s.Process(SourceBlueScan, rec("twBlueScan", "type=SwitchBotPlugMini,address=p2,rssi=-70", 6)) {
		t.Fatal("plug without load must be ignored")
	}
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	if s.Processed != 5 {
		t.Fatalf("processed=%d", s.Processed)
	}
	devs := load[BlueDeviceEnt](t, st, KindBlueDevice)
	if len(devs) != 1 {
		t.Fatal("device")
	}
	for _, d := range devs {
		if d.Address != "d7:bb" || d.Vendor != "V" || d.AddressType != "Public" || d.RSSI[0].Value != -64 {
			t.Fatalf("%+v", d)
		}
		if d.FirstTime != time.Date(2024, 2, 5, 21, 13, 49, 0, time.UTC).UnixNano() {
			t.Fatalf("first=%d", d.FirstTime)
		}
	}
	envs := load[EnvMonitorEnt](t, st, KindEnvMonitor)
	if len(envs) != 2 {
		t.Fatalf("env=%d", len(envs))
	}
	for _, e := range envs {
		switch e.Address {
		case "e1":
			if e.EnvData[0].Temp != 23.5 || e.EnvData[0].ECo2 != 400 || e.EnvData[0].Illuminance != 10 {
				t.Fatalf("%+v", e.EnvData[0])
			}
		case "e2":
			if e.EnvData[0].Battery != 90 || e.EnvData[0].ECo2 != 500 {
				t.Fatalf("%+v", e.EnvData[0])
			}
		}
	}
	for _, p := range load[PowerMonitorEnt](t, st, KindPowerMonitor) {
		if p.Data[0].Load != 12.5 || !p.Data[0].Switch || p.Data[0].Over {
			t.Fatalf("%+v", p.Data[0])
		}
	}
	for _, m := range load[MotionSensorEnt](t, st, KindMotionSensor) {
		if !m.Data[0].Moving || m.Data[0].Battery != 100 || m.Data[0].LastMove == 0 {
			t.Fatalf("%+v", m.Data[0])
		}
	}
}

func TestWinLogonAndScore(t *testing.T) {
	st := newStore(t)
	s := NewSession(context.Background(), st)
	tm := "2021-08-19T05:17:43+09:00"
	s.Process(SourceWinLog, rec("twwinlog", "type=Logon,subject=@,target=bob@PC,computer=PC,ip=1.1.1.1,logonType=Network,time="+tm, 1))
	s.Process(SourceWinLog, rec("twwinlog", "type=Logon,subject=@,target=bob@PC,computer=PC,ip=1.1.1.1,logonType=Network,time="+tm, 2))
	s.Process(SourceWinLog, rec("twwinlog", "type=LogonFailed,subject=@,target=bob@PC,computer=PC,ip=1.1.1.1,logonType=Network,failedCode=BadPassword,time="+tm, 3))
	s.Process(SourceWinLog, rec("twwinlog", "type=Logoff,subject=@,target=bob@PC,computer=PC,ip=,logonType=Network,time="+tm, 4))
	s.Process(SourceWinLog, rec("twwinlog", "type=Logon,subject=@,target=eve@PC,computer=PC,ip=2.2.2.2,logonType=Network,time="+tm, 5))
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	logons := load[WinLogonEnt](t, st, KindWinLogon)
	if len(logons) != 2 {
		t.Fatalf("logons=%d", len(logons))
	}
	var bob, eve *WinLogonEnt
	for _, l := range logons {
		if l.Target == "bob@PC" {
			bob = l
		} else {
			eve = l
		}
	}
	if bob.Count != 3 || bob.Logon != 2 || bob.Failed != 1 || bob.Logoff != 1 || bob.FailedCode["BadPassword"] != 1 || bob.LogonType["Network"] != 2 {
		t.Fatalf("bob %+v", bob)
	}
	// penalty = 1 + 10*1/3 = 4
	if bob.Penalty != 4 || eve.Penalty != 0 || !bob.ValidScore || !eve.ValidScore {
		t.Fatalf("penalty bob=%d eve=%d", bob.Penalty, eve.Penalty)
	}
	if !(bob.Score < 50 && eve.Score > 50) {
		t.Fatalf("score bob=%f eve=%f", bob.Score, eve.Score)
	}
}

func TestWinOthers(t *testing.T) {
	st := newStore(t)
	s := NewSession(context.Background(), st)
	lt := "2021-08-19T05:17:43+09:00"
	ev := "type=EventID,computer=PC,channel=Security,provider=P,eventID=4624,total=10,count=3,ft=" + lt + ",lt=" + lt
	r := rec("twwinlog", ev, 1)
	r.Severity = 3
	s.Process(SourceWinLog, r)
	r2 := rec("twwinlog", ev, 2)
	r2.Severity = 6
	s.Process(SourceWinLog, r2)
	s.Process(SourceWinLog, rec("twwinlog", "type=EventID,computer=PC,eventID=0,count=1", 1)) // ignored
	s.Process(SourceWinLog, rec("twwinlog", "type=Account,subject=a,target=t,computer=PC,count=2,edit=1,password=1,other=0,ft="+lt+",lt="+lt, 1))
	s.Process(SourceWinLog, rec("twwinlog", "type=Kerberos,target=t,computer=PC,ip=1.1.1.1,service=krbtgt,ticketType=TGT,count=4,failed=1,ft="+lt+",lt="+lt, 1))
	s.Process(SourceWinLog, rec("twwinlog", "type=Privilege,subject=a,computer=PC,count=5,ft="+lt+",lt="+lt, 1))
	s.Process(SourceWinLog, rec("twwinlog", "type=Process,computer=PC,process=cmd.exe,count=2,start=1,exit=1,subject=a,status=0,parent=explorer,ft="+lt+",lt="+lt, 1))
	s.Process(SourceWinLog, rec("twwinlog", "type=Task,subject=a,taskname=t1,computer=PC,count=1,ft="+lt+",lt="+lt, 1))
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	if s.Processed != 7 {
		t.Fatalf("processed=%d", s.Processed)
	}
	evs := load[WinEventIDEnt](t, st, KindWinEventID)
	if len(evs) != 1 {
		t.Fatal("event")
	}
	for _, e := range evs {
		// first record creates with total(10) > count(3); second adds 3
		if e.Count != 13 || e.Level != "error" || e.EventID != 4624 {
			t.Fatalf("%+v", e)
		}
	}
	for _, k := range load[WinKerberosEnt](t, st, KindWinKerberos) {
		if k.Count != 4 || k.Failed != 1 || k.Penalty != 3 || !k.ValidScore {
			t.Fatalf("%+v", k)
		}
	}
	for kind, n := range map[string]int{KindWinAccount: 1, KindWinPrivilege: 1, KindWinProcess: 1, KindWinTask: 1} {
		items, _ := st.ListLogReportData(context.Background(), kind)
		if len(items) != n {
			t.Fatalf("%s=%d", kind, len(items))
		}
	}
	for _, p := range load[WinProcessEnt](t, st, KindWinProcess) {
		if p.Start != 1 || p.LastParent != "explorer" {
			t.Fatalf("%+v", p)
		}
	}
}

func TestPcap(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	if err := st.SaveNode(ctx, &datastore.NodeEnt{ID: "n1", Name: "gw", IP: "192.168.1.1"}); err != nil {
		t.Fatal(err)
	}
	s := NewSession(ctx, st)
	lt := "2021-08-19T05:17:43+09:00"
	s.Process(SourcePcap, rec("twpcap", "type=EtherType,0x0800=100,0x1234=1", 1))
	s.Process(SourcePcap, rec("twpcap", "type=EtherType,0x0800=50", 2))
	s.Process(SourcePcap, rec("twpcap", "type=DNS,DNSType=A,Name=example.com,sv=8.8.8.8,count=5,change=1,lcl=1.1.1.1,lMAC=aa,ft="+lt+",lt="+lt, 1))
	s.Process(SourcePcap, rec("twpcap", "type=RADIUS,cl=192.168.1.1,sv=10.0.0.1,count=3,req=3,accept=1,reject=2,challenge=0,ft="+lt+",lt="+lt, 1))
	s.Process(SourcePcap, rec("twpcap", "type=TLSFlow,cl=192.168.1.1,sv=9.9.9.9,serv=HTTPS,count=2,maxver=TLS1.2,cipher=X,ft="+lt+",lt="+lt, 1))
	s.Process(SourcePcap, rec("twpcap", "type=IPToMAC,mac=aa,ip=1.1.1.1", 1)) // not handled
	if err := s.Commit(); err != nil {
		t.Fatal(err)
	}
	if s.Processed != 5 {
		t.Fatalf("processed=%d", s.Processed)
	}
	ethers := load[EtherTypeEnt](t, st, KindEtherType)
	if e := ethers["host1:0x0800"]; e == nil || e.Count != 150 || e.Name != "IPv4" {
		t.Fatalf("%+v", e)
	}
	if e := ethers["host1:0x1234"]; e == nil || e.Name != "Other(0x1234)" {
		t.Fatalf("%+v", e)
	}
	for _, d := range load[DNSQEnt](t, st, KindDNSQ) {
		if d.Count != 5 || d.Name != "example.com" || d.Server != "8.8.8.8" || d.LastClient != "1.1.1.1" {
			t.Fatalf("%+v", d)
		}
	}
	for _, r := range load[RADIUSFlowEnt](t, st, KindRADIUSFlow) {
		// reject>0 (+1), accept<reject (+1), server name unresolved (+1)
		if r.Reject != 2 || r.Penalty != 3 || r.ClientName != "gw" || r.ClientNodeID != "n1" || r.ServerName != "10.0.0.1" {
			t.Fatalf("%+v", r)
		}
	}
	for _, f := range load[TLSFlowEnt](t, st, KindTLSFlow) {
		// TLS1.2 (+1), server name unresolved (+1)
		if f.Penalty != 2 || f.Version != "TLS1.2" || f.ClientName != "gw" {
			t.Fatalf("%+v", f)
		}
	}
}

func TestCleanup(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	now := time.Now()
	put := func(kind, id string, e any) {
		b, _ := json.Marshal(e)
		if err := st.SaveLogReportData(ctx, kind, map[string][]byte{id: b}); err != nil {
			t.Fatal(err)
		}
	}
	put(KindWifiAP, "old", &WifiAPEnt{ID: "old", LastTime: now.AddDate(0, 0, -40).UnixNano()})
	for i := 0; i < 5; i++ {
		put(KindWifiAP, fmt.Sprintf("n%d", i), &WifiAPEnt{ID: fmt.Sprintf("n%d", i), Count: 1, LastTime: now.Add(-time.Duration(i) * time.Hour).UnixNano()})
	}
	put(KindBlueDevice, "rnd", &BlueDeviceEnt{ID: "rnd", AddressType: "BLE Random", LastTime: now.AddDate(0, 0, -2).UnixNano()})
	put(KindBlueDevice, "pub", &BlueDeviceEnt{ID: "pub", AddressType: "BLE Public", LastTime: now.AddDate(0, 0, -2).UnixNano()})

	if err := Cleanup(ctx, st, SourceWifiScan, 30, 3); err != nil {
		t.Fatal(err)
	}
	aps, _ := st.ListLogReportData(ctx, KindWifiAP)
	if len(aps) != 3 {
		t.Fatalf("aps=%d", len(aps))
	}
	// the three most recent entries are kept
	for _, id := range []string{"n0", "n1", "n2"} {
		if _, ok := aps[id]; !ok {
			t.Fatalf("%s must be kept", id)
		}
	}
	if err := Cleanup(ctx, st, SourceBlueScan, 30, 100); err != nil {
		t.Fatal(err)
	}
	devs, _ := st.ListLogReportData(ctx, KindBlueDevice)
	if _, ok := devs["rnd"]; ok || len(devs) != 1 {
		t.Fatalf("devs=%v", devs)
	}
}

func TestSourceHelpers(t *testing.T) {
	for _, s := range []string{SourceWifiScan, SourceBlueScan, SourcePcap, SourceWinLog} {
		if !IsSource(s) || SyslogTag(s) == "" || len(KindsOf(s)) == 0 {
			t.Fatalf("source %s", s)
		}
	}
	if IsSource("syslog") || IsKind("nope") || !IsKind(KindWinTask) {
		t.Fatal("helper mismatch")
	}
}
