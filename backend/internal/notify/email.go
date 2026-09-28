// Package notify : 通知処理 - メール送信
package notify

import (
	"bytes"
	"crypto/tls"
	"fmt"
	"html/template"
	"log"
	"net"
	"strconv"
	"strings"

	"github.com/wneessen/go-mail"

	"github.com/twsnmp/twsnmpneo/backend/internal/datastore"
	"github.com/twsnmp/twsnmpneo/backend/internal/i18n"
)

func (m *Manager) canSendMail() bool {
	conf := m.getNotifyConf()
	switch conf.Provider {
	case "google", "microsoft":
		return m.store.HasValidNotifyOAuth2Token(&conf)
	case "mscustom":
		if conf.MailServer == "" {
			return false
		}
		return m.store.HasValidNotifyOAuth2Token(&conf)
	default:
		if conf.MailServer == "" ||
			conf.MailFrom == "" ||
			conf.MailTo == "" {
			return false
		}
	}
	return true
}

func (m *Manager) sendNotifyMail(list []*datastore.EventLogEnt) {
	if !m.canSendMail() {
		return
	}
	conf := m.getNotifyConf()
	nl := getLevelNum(conf.Level)
	if nl == 3 {
		return
	}
	nd := m.getNotifyData(list, nl)
	if nd.failureBody != "" {
		err := m.SendMail(nd.failureSubject, nd.failureBody)
		r := ""
		level := "info"
		if err != nil {
			log.Printf("send mail err=%v", err)
			r = fmt.Sprintf("err=%v", err)
			level = "low"
		}
		m.addEventLog(level, fmt.Sprintf(i18n.Trans("Send notify mail %s"), r))
	}
	if nd.repairBody != "" {
		err := m.SendMail(nd.repairSubject, nd.repairBody)
		r := ""
		level := "info"
		if err != nil {
			log.Printf("send mail err=%v", err)
			r = fmt.Sprintf("err=%v", err)
			level = "low"
		}
		m.addEventLog(level, fmt.Sprintf(i18n.Trans("Send repair mail %s"), r))
	}
}

// SendMail sends an email with the given subject and body.
func (m *Manager) SendMail(subject, body string) error {
	if !m.canSendMail() {
		return nil
	}
	conf := m.getNotifyConf()
	switch conf.Provider {
	case "google":
		return m.sendMailOAuth2("smtp.gmail.com", subject, body)
	case "microsoft":
		return m.sendMailOAuth2("smtp-mail.outlook.com", subject, body)
	case "mscustom":
		return m.sendMailOAuth2(conf.MailServer, subject, body)
	default:
		return m.sendMailSMTP(subject, body)
	}
}

func (m *Manager) sendMailSMTP(subject, body string) error {
	conf := m.getNotifyConf()
	host, portStr, err := net.SplitHostPort(conf.MailServer)
	if err != nil {
		host = conf.MailServer
		portStr = ""
	}

	var options []mail.Option

	if portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			options = append(options, mail.WithPort(port))
		}
	}

	tlsconfig := &tls.Config{
		ServerName: host,
		// #nosec G402
		InsecureSkipVerify: conf.InsecureSkipVerify,
	}
	if conf.InsecureSkipVerify {
		for _, e := range tls.CipherSuites() {
			tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, e.ID)
		}
		tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, tls.TLS_RSA_WITH_AES_128_GCM_SHA256)
		tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, tls.TLS_RSA_WITH_AES_256_GCM_SHA384)
	}
	options = append(options, mail.WithTLSConfig(tlsconfig))

	if strings.HasSuffix(conf.MailServer, ":465") {
		options = append(options, mail.WithSSL())
	} else {
		options = append(options, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}

	if conf.User != "" {
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(conf.User),
			mail.WithPassword(conf.Password),
		)
	}

	client, err := mail.NewClient(host, options...)
	if err != nil {
		log.Printf("send mail err=%v", err)
		return err
	}

	message := mail.NewMsg()
	if err := message.From(conf.MailFrom); err != nil {
		log.Printf("send mail err=%v", err)
		return err
	}
	for _, rcpt := range strings.Split(conf.MailTo, ",") {
		if !strings.Contains(rcpt, "@") {
			continue
		}
		if err := message.AddTo(rcpt); err != nil {
			log.Printf("send mail err=%v", err)
			return err
		}
	}

	message.Subject(subject)
	message.SetBodyString(mail.TypeTextHTML, body)

	if err := client.DialAndSend(message); err != nil {
		log.Printf("send mail err=%v", err)
		return err
	}

	log.Printf("send mail to %s", conf.MailTo)
	return nil
}

// SendTestMail sends a test email using the given test configuration.
func SendTestMail(store datastore.DataStore, testConf *datastore.NotifyConfEnt) error {
	m := &Manager{store: store}
	switch testConf.Provider {
	case "google":
		return m.sendTestMailOAuth2("smtp.gmail.com", testConf)
	case "microsoft":
		return m.sendTestMailOAuth2("smtp-mail.outlook.com", testConf)
	case "mscustom":
		return m.sendTestMailOAuth2(testConf.MailServer, testConf)
	default:
		return m.sendTestMailSMTP(testConf)
	}
}

func (m *Manager) sendTestMailSMTP(testConf *datastore.NotifyConfEnt) error {
	host, portStr, err := net.SplitHostPort(testConf.MailServer)
	if err != nil {
		host = testConf.MailServer
		portStr = ""
	}

	var options []mail.Option

	if portStr != "" {
		if port, err := strconv.Atoi(portStr); err == nil {
			options = append(options, mail.WithPort(port))
		}
	}

	tlsconfig := &tls.Config{
		ServerName: host,
		// #nosec G402
		InsecureSkipVerify: testConf.InsecureSkipVerify,
	}
	if testConf.InsecureSkipVerify {
		for _, e := range tls.CipherSuites() {
			tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, e.ID)
		}
		tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, tls.TLS_RSA_WITH_AES_128_GCM_SHA256)
		tlsconfig.CipherSuites = append(tlsconfig.CipherSuites, tls.TLS_RSA_WITH_AES_256_GCM_SHA384)
	}
	options = append(options, mail.WithTLSConfig(tlsconfig), mail.WithDialContextFunc(smtpDialContext))

	if strings.HasSuffix(testConf.MailServer, ":465") {
		options = append(options, mail.WithSSL())
	} else {
		options = append(options, mail.WithTLSPolicy(mail.TLSOpportunistic))
	}

	if testConf.User != "" {
		options = append(options,
			mail.WithSMTPAuth(mail.SMTPAuthAutoDiscover),
			mail.WithUsername(testConf.User),
			mail.WithPassword(testConf.Password),
		)
	}

	client, err := mail.NewClient(host, options...)
	if err != nil {
		log.Printf("send test mail err=%v", err)
		return err
	}

	message := mail.NewMsg()
	if err := message.From(testConf.MailFrom); err != nil {
		log.Printf("send test mail err=%v", err)
		return err
	}
	for _, rcpt := range strings.Split(testConf.MailTo, ",") {
		if !strings.Contains(rcpt, "@") {
			continue
		}
		if err := message.AddTo(rcpt); err != nil {
			log.Printf("send test mail err=%v", err)
			return err
		}
	}

	t, err := template.New("test").Parse(m.store.LoadMailTemplate("test"))
	if err != nil {
		log.Printf("send test mail err=%s", err)
		return err
	}
	buffer := new(bytes.Buffer)
	if err = t.Execute(buffer, map[string]interface{}{
		"Title": testConf.Subject + i18n.Trans("(test mail)"),
	}); err != nil {
		return err
	}
	body := buffer.String()

	message.Subject(testConf.Subject)
	message.SetBodyString(mail.TypeTextHTML, body)

	if err := client.DialAndSend(message); err != nil {
		log.Printf("send test mail err=%v", err)
		return err
	}

	return nil
}

func (m *Manager) sendMailOAuth2(server, subject, body string) error {
	token := refreshOAuth2Token(m.store)
	if token == nil {
		return fmt.Errorf("oauth2 token not found or invalid")
	}
	conf := m.getNotifyConf()
	host, portStr, err := net.SplitHostPort(server)
	var port int
	if err != nil {
		host = server
	} else {
		port, _ = strconv.Atoi(portStr)
	}
	opts := []mail.Option{
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthXOAUTH2),
		mail.WithUsername(conf.User),
		mail.WithPassword(token.AccessToken),
	}
	if port > 0 {
		opts = append(opts, mail.WithPort(port))
	}
	client, err := mail.NewClient(host, opts...)
	if err != nil {
		return err
	}
	message := mail.NewMsg()
	if err := message.From(conf.MailFrom); err != nil {
		return err
	}
	for _, rcpt := range strings.Split(conf.MailTo, ",") {
		if !strings.Contains(rcpt, "@") {
			continue
		}
		if err := message.AddTo(rcpt); err != nil {
			return err
		}
	}

	message.Subject(subject)
	message.SetBodyString(mail.TypeTextHTML, body)
	return client.DialAndSend(message)
}

func (m *Manager) sendTestMailOAuth2(server string, testConf *datastore.NotifyConfEnt) error {
	token := refreshOAuth2Token(m.store)
	if token == nil {
		return fmt.Errorf("oauth2 token not found or invalid")
	}
	host, portStr, err := net.SplitHostPort(server)
	var port int
	if err != nil {
		host = server
	} else {
		port, _ = strconv.Atoi(portStr)
	}
	opts := []mail.Option{
		mail.WithTLSPortPolicy(mail.TLSMandatory),
		mail.WithSMTPAuth(mail.SMTPAuthXOAUTH2),
		mail.WithUsername(testConf.User),
		mail.WithPassword(token.AccessToken),
	}
	if testConf.Provider == "mscustom" {
		opts = append(opts, mail.WithDialContextFunc(smtpDialContext))
	} else {
		opts = append(opts, mail.WithDialContextFunc(publicDialContext))
	}
	if port > 0 {
		opts = append(opts, mail.WithPort(port))
	}
	client, err := mail.NewClient(host, opts...)
	if err != nil {
		return err
	}
	message := mail.NewMsg()
	if err := message.From(testConf.MailFrom); err != nil {
		return err
	}
	for _, rcpt := range strings.Split(testConf.MailTo, ",") {
		if !strings.Contains(rcpt, "@") {
			continue
		}
		if err := message.AddTo(rcpt); err != nil {
			return err
		}
	}
	t, err := template.New("test").Parse(m.store.LoadMailTemplate("test"))
	if err != nil {
		log.Printf("send test mail err=%s", err)
		return err
	}
	buffer := new(bytes.Buffer)
	if err = t.Execute(buffer, map[string]interface{}{
		"Title": testConf.Subject + i18n.Trans("(test mail)"),
	}); err != nil {
		return err
	}
	body := buffer.String()
	message.Subject(testConf.Subject)
	message.SetBodyString(mail.TypeTextHTML, body)
	return client.DialAndSend(message)
}
