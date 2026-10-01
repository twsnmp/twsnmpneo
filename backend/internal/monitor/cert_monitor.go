package monitor

import (
	"context"
	"crypto/tls"
	"fmt"
	"log/slog"
	"net"
	"sync"
	"time"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
)

// CheckCert probes the TLS endpoint and updates the CertMonitorEnt.
func CheckCert(c *datastore.CertMonitorEnt, timeoutSec int) {
	if timeoutSec <= 0 {
		timeoutSec = 10
	}
	port := c.Port
	if port <= 0 {
		port = 443
	}
	target := fmt.Sprintf("%s:%d", c.Target, port)
	c.Verify = false
	c.Error = ""

	now := time.Now()
	if c.FirstTime == 0 {
		c.FirstTime = now.Unix()
	}
	c.LastTime = now.Unix()

	d := &net.Dialer{
		Timeout: time.Duration(timeoutSec) * time.Second,
	}

	// First attempt with verification
	conf := &tls.Config{
		ServerName:         c.Target,
		InsecureSkipVerify: false,
	}

	conn, err := tls.DialWithDialer(d, "tcp", target, conf)
	if err != nil {
		c.Error = err.Error()
		// Try again with InsecureSkipVerify to retrieve certificate details even if self-signed/expired
		conf.InsecureSkipVerify = true
		connInsecure, err2 := tls.DialWithDialer(d, "tcp", target, conf)
		if err2 != nil {
			c.State = "error"
			c.Error = fmt.Sprintf("TLS connection failed: %v", err)
			return
		}
		conn = connInsecure
	} else {
		c.Verify = true
	}
	defer conn.Close()

	cs := conn.ConnectionState()
	if len(cs.PeerCertificates) == 0 {
		c.State = "error"
		c.Error = "no TLS certificates presented"
		return
	}

	cert := cs.PeerCertificates[0]
	c.SerialNumber = cert.SerialNumber.String()
	c.Subject = cert.Subject.String()
	c.Issuer = cert.Issuer.String()
	c.NotBefore = cert.NotBefore.Unix()
	c.NotAfter = cert.NotAfter.Unix()

	// Evaluate expiration
	remaining := time.Until(cert.NotAfter)
	if remaining < 0 {
		c.State = "error"
		c.Error = "certificate has expired"
	} else if remaining < 30*24*time.Hour {
		c.State = "warn"
		if remaining < 7*24*time.Hour {
			c.Error = "certificate expires in less than 7 days"
		} else {
			c.Error = "certificate expires in less than 30 days"
		}
	} else {
		c.State = "normal"
		c.Error = ""
	}
}

// CheckAllCertMonitors queries all registered certificate monitors concurrently.
func CheckAllCertMonitors(ctx context.Context, store datastore.DataStore) error {
	if store == nil {
		return datastore.ErrDBNotOpen
	}
	list, err := store.ListCertMonitors(ctx)
	if err != nil {
		return err
	}
	if len(list) == 0 {
		return nil
	}

	var wg sync.WaitGroup
	sem := make(chan struct{}, 8) // Limit concurrent TLS dials

	for _, cm := range list {
		wg.Add(1)
		go func(ent *datastore.CertMonitorEnt) {
			defer wg.Done()
			select {
			case sem <- struct{}{}:
				defer func() { <-sem }()
			case <-ctx.Done():
				return
			}
			CheckCert(ent, 10)
			_ = store.SaveCertMonitor(ctx, ent)
			slog.Debug("Checked TLS certificate monitor", "target", ent.Target, "state", ent.State)
		}(cm)
	}

	wg.Wait()
	return nil
}
