// Package notify : 通知処理 - コマンド実行
package notify

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
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
		var dataDir string
		if m != nil && m.store != nil {
			dataDir = m.store.GetDataDir()
		}
		err := ExecNotifyCmdWithDir(dataDir, conf.ExecCmd, execLevel)
		if err != nil {
			slog.Error("exec notify command error", "error", err)
			m.addEventLog("low", fmt.Sprintf(i18n.Trans("Exec notify command err=%v"), err))
		}
		lastExecLevel = execLevel
	}
}

// ExecNotifyCmd executes the notification command with the given level using default manager's dir.
func ExecNotifyCmd(c string, level int) error {
	var dataDir string
	if defaultManager != nil && defaultManager.store != nil {
		dataDir = defaultManager.store.GetDataDir()
	}
	return ExecNotifyCmdWithDir(dataDir, c, level)
}

// ExecNotifyCmdWithDir executes the notification command resolving files in dataDir/cmd.
func ExecNotifyCmdWithDir(dataDir string, c string, level int) error {
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

	cmdName := cl[0]
	// Check if script or executable exists in dataDir/cmd or ./cmd
	var targetCmd string
	if dataDir != "" {
		p := filepath.Join(dataDir, "cmd", cmdName)
		if _, err := os.Stat(p); err == nil {
			targetCmd = p
		}
	}
	if targetCmd == "" {
		p := filepath.Join("cmd", cmdName)
		if _, err := os.Stat(p); err == nil {
			targetCmd = p
		}
	}

	var cmd *exec.Cmd
	if targetCmd != "" {
		if strings.HasSuffix(targetCmd, ".sh") {
			fullCmd := targetCmd + " " + strings.Join(cl[1:], " ")
			cmd = exec.CommandContext(ctx, "/bin/sh", "-c", fullCmd)
		} else {
			cmd = exec.CommandContext(ctx, targetCmd, cl[1:]...)
		}
	} else if strings.HasSuffix(cmdName, ".sh") {
		fullCmd := cmdName + " " + strings.Join(cl[1:], " ")
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", fullCmd)
	} else {
		cmd = exec.CommandContext(ctx, cmdName, cl[1:]...)
	}

	cmd.WaitDelay = 5 * time.Second
	return cmd.Run()
}
