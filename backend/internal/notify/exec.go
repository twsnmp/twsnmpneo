// Package notify : 通知処理 - コマンド実行
package notify

import (
	"context"
	"fmt"
	"log"
	"os/exec"
	"strconv"
	"strings"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

func (m *Manager) checkExecCmd() {
	conf := m.getNotifyConf()
	if conf.ExecCmd == "" {
		return
	}
	execLevel := 3
	m.store.ForEachNodes(func(n *datastore.NodeEnt) bool {
		ns := getLevelNum(n.State)
		if execLevel > ns {
			execLevel = ns
			if ns == 0 {
				return false
			}
		}
		return true
	})
	if execLevel != lastExecLevel {
		err := ExecNotifyCmd(conf.ExecCmd, execLevel)
		if err != nil {
			log.Printf("execNotifyCmd err=%v", err)
			m.addEventLog("low", fmt.Sprintf(i18n.Trans("Exec notify command err=%v"), err))
		}
		lastExecLevel = execLevel
	}
}

// ExecNotifyCmd executes the notification command with the given level.
func ExecNotifyCmd(c string, level int) error {
	// 連続する空白や前後の空白も適切に分割するために strings.Fields を推奨
	cl := strings.Fields(c)
	if len(cl) == 0 {
		return fmt.Errorf("notify ExecCmd is empty")
	}

	strLevel := strconv.Itoa(level)
	for i, v := range cl {
		if v == "$level" {
			cl[i] = strLevel
		}
	}

	// 60秒のタイムアウトコンテキストを作成
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	// 可変長引数（cl[1:]...）は要素が空でも安全に展開できるため分岐は不要
	cmd := exec.CommandContext(ctx, cl[0], cl[1:]...)

	// timeout.KillAfter (5秒) の代替:
	// コンテキストキャンセル後、あるいはプロセス終了後のI/Oパイプ待ちの上限時間
	cmd.WaitDelay = 5 * time.Second

	return cmd.Run()
}
