package polling

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
)

// CmdPoller executes local shell/OS commands and evaluates output with extractors/scripts.
type CmdPoller struct{}

func NewCmdPoller() *CmdPoller {
	return &CmdPoller{}
}

func (p *CmdPoller) Poll(ctx context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	cmdStr := pe.Params
	if cmdStr == "" {
		return &Result{State: StateHigh, Message: "missing command to execute"}, nil
	}

	if node != nil {
		cmdStr = strings.ReplaceAll(cmdStr, "$IP", node.IP)
		cmdStr = strings.ReplaceAll(cmdStr, "$NODE", strings.ReplaceAll(node.Name, " ", "_"))
	}

	timeoutSec := pe.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	execCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutSec)*time.Second)
	defer cancel()

	cmd := exec.CommandContext(execCtx, "/bin/sh", "-c", cmdStr)
	var stdoutBuf, stderrBuf bytes.Buffer
	cmd.Stdout = &stdoutBuf
	cmd.Stderr = &stderrBuf

	start := time.Now()
	err := cmd.Run()
	rtt := time.Since(start)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("cmd execution error: %v", err),
				Fields: map[string]interface{}{
					"rtt":      float64(rtt.Nanoseconds()),
					"exitCode": float64(-1),
					"stderr":   stderrBuf.String(),
					"error":    err.Error(),
				},
			}, nil
		}
	}

	stdoutStr := stdoutBuf.String()
	stderrStr := stderrBuf.String()

	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"exitCode": float64(exitCode),
		"stderr":   stderrStr,
		"lastTime": time.Now().Format("2006-01-02T15:04"),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(stdoutStr, vm)
	_ = vm.Set("exitCode", float64(exitCode))
	_ = vm.Set("interval", float64(pe.PollInt))
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	// Apply extractor (getBody, jsonpath, goquery, grok)
	if pe.Extractor != "" {
		if err := extractor.ApplyExtractor(pe.Extractor, stdoutStr, vm, fields); err != nil {
			fields["error"] = err.Error()
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("extractor error: %v", err),
				Fields:  fields,
			}, nil
		}
	}

	// No script: exitCode == 0 means normal
	if pe.Script == "" {
		if exitCode == 0 {
			return &Result{
				State:   StateNormal,
				RTT:     rtt,
				Message: fmt.Sprintf("cmd ok, exitCode=0, rtt=%v", rtt),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("cmd exitCode=%d, stderr=%s", exitCode, stderrStr),
			Fields:  fields,
		}, nil
	}

	// Evaluate script
	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	value, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("script error: %v", err),
			Fields:  fields,
		}, nil
	}

	pass, _ := value.ToBoolean()
	if pass {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("cmd script passed, exitCode=%d", exitCode),
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   failureState(pe.Level),
		RTT:     rtt,
		Message: fmt.Sprintf("cmd script returned false, exitCode=%d", exitCode),
		Fields:  fields,
	}, nil
}
