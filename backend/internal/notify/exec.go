// Package notify : 通知処理 - コマンド実行
package notify

import (
	"context"
	"fmt"
	"log/slog"
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

// ExecNotifyCmdWithDir executes the notification command resolving files strictly in dataDir/cmd.
func ExecNotifyCmdWithDir(dataDir string, c string, level int) error {
	cl := strings.Fields(c)
	if len(cl) == 0 {
		return fmt.Errorf("notify ExecCmd is empty")
	}
	if filepath.Base(cl[0]) != cl[0] {
		return fmt.Errorf("notify ExecCmd has path")
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

	cmdDir := "cmd"
	if dataDir != "" {
		cmdDir = filepath.Join(dataDir, "cmd")
	}
	targetCmd := filepath.Join(cmdDir, filepath.Base(cl[0]))

	var cmd *exec.Cmd
	if filepath.Ext(cl[0]) == ".sh" {
		cl[0] = targetCmd
		cmd = exec.CommandContext(ctx, "/bin/sh", "-c", strings.Join(cl, " "))
	} else {
		if len(cl) == 1 {
			cmd = exec.CommandContext(ctx, targetCmd)
		} else {
			cmd = exec.CommandContext(ctx, targetCmd, cl[1:]...)
		}
	}

	cmd.WaitDelay = 5 * time.Second
	return cmd.Run()
}
