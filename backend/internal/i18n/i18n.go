package i18n

import (
	"strings"
	"sync"

	"github.com/jeandeaual/go-locale"
)

var (
	mu   sync.RWMutex
	lang = "en" // 取得失敗時・初期値としてのフォールバック言語
)

// 翻訳マップ
var transMap = map[string]map[string]string{
	"Add": {
		"ja": "追加",
	},
	"Update": {
		"ja": "更新",
	},
	"Added node %s (%s)": {
		"ja": "ノード %s (%s) を追加しました",
	},
	"Updated node %s (%s)": {
		"ja": "ノード %s (%s) を更新しました",
	},
	"TWSNMP NEO %s started (Web port: %d)": {
		"ja": "TWSNMP NEO %s サービスを起動しました (Webポート: %d)",
	},
	"Start polling node %s (%s)": {
		"ja": "ノード %s (%s) のポーリングを開始しました",
	},
	"Delete node %s": {
		"ja": "ノード %s を削除しました",
	},
	"Polling %s: %s -> %s (%s)": {
		"ja": "ポーリング %s: %s -> %s (%s)",
	},
	"Saved polling %s": {
		"ja": "ポーリング %s を保存しました",
	},
	"Delete polling %s": {
		"ja": "ポーリング %s を削除しました",
	},
	"Connected %d lines automatically": {
		"ja": "自動トポロジー探索により %d 本のラインを結線しました",
	},
	"Update line %s - %s": {
		"ja": "ラインを結線/更新しました (%s - %s)",
	},
	"Delete line %s": {
		"ja": "ラインを切断/削除しました (%s)",
	},
	"Added network %s (%s)": {
		"ja": "ネットワーク %s (%s) を追加しました",
	},
	"Updated network %s (%s)": {
		"ja": "ネットワーク %s (%s) を更新しました",
	},
	"Delete network %s": {
		"ja": "ネットワーク %s を削除しました",
	},
	"Added drawitem %s": {
		"ja": "描画アイテム %s を追加しました",
	},
	"Updated drawitem %s": {
		"ja": "描画アイテム %s を更新しました",
	},
	"Copy DrawItem %s": {
		"ja": "描画アイテム %s をコピーしました",
	},
	"Delete drawitem": {
		"ja": "描画アイテムを削除しました",
	},
	"Updated GeoIP database": {
		"ja": "IP位置情報DBを更新しました",
	},
	"Delete geoip database": {
		"ja": "IP位置情報DBを削除しました",
	},
	"Delete all event logs": {
		"ja": "すべてのイベントログを消去しました",
	},
	"Delete logs %s": {
		"ja": "%s ログを消去しました",
	},
	"Reset ARP table": {
		"ja": "ARP監視テーブルを全消去しました",
	},
	"Delete arp entry (IP: %s)": {
		"ja": "ARP監視エントリーを削除しました (IP: %s)",
	},
	"Delete mqtt stats (%d items)": {
		"ja": "MQTT統計を削除しました (%d件)",
	},
	"Delete all mqtt stats": {
		"ja": "すべてのMQTT統計を削除しました",
	},
	"Delete all OpenTelemetry data": {
		"ja": "全OpenTelemetryデータを消去しました",
	},
	"Memory usage warning (Host: %.1f%%, Process: %.1f%%)": {
		"ja": "メモリ使用量警告 (Host: %.1f%%, Process: %.1f%%)",
	},
	"Storage usage warning (Disk: %.1f%%)": {
		"ja": "ストレージ容量逼迫警告 (Disk: %.1f%%)",
	},
	"CPU high load warning (Load Avg: %.2f / CPU count: %d)": {
		"ja": "CPU高負荷警告 (Load Avg: %.2f / CPU数: %d)",
	},
	"Starting ARP Watch Engine...": {
		"ja": "ARP監視を開始しました",
	},
	"Stopping ARP Watch Engine...": {
		"ja": "ARP監視を停止しました",
	},
	"ARP monitoring range %s utilization: %d/%d (%.2f%%)": {
		"ja": "ARP監視範囲 %s 利用率: %d/%d (%.2f%%)",
	},
	"New MAC address detected %s (%s - %s)": {
		"ja": "新規MACアドレス検知 %s (%s - %s)",
	},
	"MAC address change detected %s (%s -> %s)": {
		"ja": "MACアドレス変更検知 %s (%s -> %s)",
	},
	"Node %s add MAC address %s (%s)": {
		"ja": "ノード %s のMACアドレスを自動登録しました: %s (%s)",
	},
	"Node %s change MAC address: %s -> %s": {
		"ja": "ノード %s のMACアドレス変更を検知・更新しました: %s -> %s",
	},
}

func init() {
	if loc, err := locale.GetLocale(); err == nil {
		lang = parseLanguage(loc)
	}
}

// parseLanguage はロケール文字列（例: "ja-JP", "ja_JP", "ja"）から言語コード部（"ja"）を取り出します
func parseLanguage(loc string) string {
	loc = strings.ReplaceAll(loc, "_", "-")
	parts := strings.Split(loc, "-")
	if len(parts) > 0 && parts[0] != "" {
		return strings.ToLower(parts[0])
	}
	return "en"
}

// SetLang は使用する言語を設定します（スレッドセーフ）
func SetLang(l string) {
	mu.Lock()
	defer mu.Unlock()
	lang = strings.ToLower(l)
}

// GetLang は現在設定されている言語を返します（スレッドセーフ）
func GetLang() string {
	mu.RLock()
	defer mu.RUnlock()
	return lang
}

// Trans は指定された文字列の翻訳を返します。
// 登録がない場合や英語環境の場合はそのまま元の文字列を返します。
func Trans(s string) string {
	mu.RLock()
	currentLang := lang
	mu.RUnlock()

	if m, ok := transMap[s]; ok {
		if t, ok := m[currentLang]; ok {
			return t
		}
	}
	return s
}
