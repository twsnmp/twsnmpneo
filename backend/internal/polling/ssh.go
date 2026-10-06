package polling

import (
	"context"
	"fmt"
	"net"
	"strconv"
	"time"

	"github.com/robertkrimen/otto"
	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/extractor"
	"golang.org/x/crypto/ssh"
)

// SSHPoller executes remote commands over SSH and evaluates output.
type SSHPoller struct{}

func NewSSHPoller() *SSHPoller {
	return &SSHPoller{}
}

func (p *SSHPoller) Poll(_ context.Context, pe *datastore.PollingEnt, node *datastore.NodeEnt) (*Result, error) {
	if node == nil || node.IP == "" {
		return &Result{State: StateHigh, Message: "missing node IP for SSH"}, nil
	}

	cmdStr := pe.Params
	if cmdStr == "" {
		return &Result{State: StateHigh, Message: "missing SSH command to execute"}, nil
	}

	port := "22"
	if pe.Mode != "" {
		if pNum, err := strconv.Atoi(pe.Mode); err != nil || pNum <= 0 || pNum > 65535 {
			return &Result{
				State:   StateUnknown,
				Message: fmt.Sprintf("unsupported ssh mode: %s", pe.Mode),
				Fields:  map[string]interface{}{"error": fmt.Sprintf("unsupported ssh mode: %s", pe.Mode)},
			}, nil
		}
		port = pe.Mode
	}

	timeoutSec := pe.Timeout
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	timeout := time.Duration(timeoutSec) * time.Second

	sshConfig := &ssh.ClientConfig{
		User:    node.SSHUser,
		Timeout: timeout,
		HostKeyCallback: func(hostname string, remote net.Addr, key ssh.PublicKey) error {
			// Auto-accept host key like FK
			return nil
		},
	}
	if sshConfig.User == "" {
		sshConfig.User = node.User
	}
	if sshConfig.User == "" {
		sshConfig.User = "root"
	}

	if node.Password != "" {
		sshConfig.Auth = append(sshConfig.Auth, ssh.Password(node.Password))
	}
	if node.PublicKey != "" {
		if signer, err := ssh.ParsePrivateKey([]byte(node.PublicKey)); err == nil {
			sshConfig.Auth = append(sshConfig.Auth, ssh.PublicKeys(signer))
		}
	}

	target := net.JoinHostPort(node.IP, port)
	start := time.Now()

	conn, err := net.DialTimeout("tcp", target, timeout)
	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("ssh connect to %s failed: %v", target, err),
			Fields: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, target, sshConfig)
	if err != nil {
		conn.Close()
		return &Result{
			State:   StateHigh,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("ssh handshake with %s failed: %v", target, err),
			Fields: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}
	client := ssh.NewClient(c, chans, reqs)
	defer client.Close()

	session, err := client.NewSession()
	if err != nil {
		return &Result{
			State:   StateHigh,
			RTT:     time.Since(start),
			Message: fmt.Sprintf("ssh new session failed: %v", err),
			Fields: map[string]interface{}{
				"error": err.Error(),
			},
		}, nil
	}
	defer session.Close()

	cmdStart := time.Now()
	out, err := session.CombinedOutput(cmdStr)
	rtt := time.Since(cmdStart)

	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*ssh.ExitError); ok {
			exitCode = exitErr.ExitStatus()
		} else {
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("ssh command run error: %v", err),
				Fields: map[string]interface{}{
					"rtt":      float64(rtt.Nanoseconds()),
					"exitCode": float64(-1),
					"error":    err.Error(),
				},
			}, nil
		}
	}

	outStr := string(out)
	fields := map[string]interface{}{
		"rtt":      float64(rtt.Nanoseconds()),
		"exitCode": float64(exitCode),
		"lastTime": time.Now().Format("2006-01-02T15:04"),
	}

	vm := otto.New()
	SetupOttoVM(pe, vm, fields)
	extractor.RegisterBodyHelpers(outStr, vm)
	_ = vm.Set("exitCode", float64(exitCode))
	_ = vm.Set("interval", float64(pe.PollInt))
	_ = vm.Set("rtt", float64(rtt.Nanoseconds()))

	if pe.Extractor != "" {
		if err := extractor.ApplyExtractor(pe.Extractor, outStr, vm, fields); err != nil {
			fields["error"] = err.Error()
			return &Result{
				State:   StateHigh,
				RTT:     rtt,
				Message: fmt.Sprintf("ssh extractor error: %v", err),
				Fields:  fields,
			}, nil
		}
	}

	if pe.Script == "" {
		if exitCode == 0 {
			return &Result{
				State:   StateNormal,
				RTT:     rtt,
				Message: fmt.Sprintf("ssh cmd ok, exitCode=0, rtt=%v", rtt),
				Fields:  fields,
			}, nil
		}
		return &Result{
			State:   failureState(pe.Level),
			RTT:     rtt,
			Message: fmt.Sprintf("ssh cmd exitCode=%d", exitCode),
			Fields:  fields,
		}, nil
	}

	for k, v := range fields {
		_ = vm.Set(k, v)
	}

	value, err := vm.Run(pe.Script)
	if err != nil {
		fields["error"] = err.Error()
		return &Result{
			State:   StateHigh,
			RTT:     rtt,
			Message: fmt.Sprintf("ssh script error: %v", err),
			Fields:  fields,
		}, nil
	}

	pass, _ := value.ToBoolean()
	if pass {
		return &Result{
			State:   StateNormal,
			RTT:     rtt,
			Message: fmt.Sprintf("ssh script passed, exitCode=%d", exitCode),
			Fields:  fields,
		}, nil
	}

	return &Result{
		State:   failureState(pe.Level),
		RTT:     rtt,
		Message: fmt.Sprintf("ssh script returned false, exitCode=%d", exitCode),
		Fields:  fields,
	}, nil
}
