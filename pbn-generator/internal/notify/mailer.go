package notify

import (
	"context"
	"crypto/tls"
	"fmt"
	"net/smtp"
)

type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
}

type NoopMailer struct{}

func (NoopMailer) Send(ctx context.Context, to, subject, body string) error {
	return nil
}

type SMTPConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	From     string
	UseTLS   bool
}

type SMTPMailer struct {
	cfg  SMTPConfig
	auth smtp.Auth
}

func NewSMTPMailer(cfg SMTPConfig) *SMTPMailer {
	var auth smtp.Auth
	if cfg.User != "" {
		auth = smtp.PlainAuth("", cfg.User, cfg.Password, cfg.Host)
	}
	return &SMTPMailer{cfg: cfg, auth: auth}
}

func (m *SMTPMailer) Send(ctx context.Context, to, subject, body string) error {
	addr := fmt.Sprintf("%s:%d", m.cfg.Host, m.cfg.Port)
	msg := fmt.Sprintf("From: %s\r\nTo: %s\r\nSubject: %s\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=\"utf-8\"\r\n\r\n%s", m.cfg.From, to, subject, body)

	// Prefer STARTTLS if TLS not forced; if UseTLS true, start with TLS.
	dial := func() (*smtp.Client, error) {
		if m.cfg.UseTLS {
			conn, err := tls.Dial("tcp", addr, &tls.Config{ServerName: m.cfg.Host})
			if err != nil {
				return nil, err
			}
			return smtp.NewClient(conn, m.cfg.Host)
		}
		return smtp.Dial(addr)
	}

	c, err := dial()
	if err != nil {
		return err
	}
	defer c.Close()

	// Upgrade to TLS if supported and not already.
	if !m.cfg.UseTLS {
		if ok, _ := c.Extension("STARTTLS"); ok {
			if err := c.StartTLS(&tls.Config{ServerName: m.cfg.Host}); err != nil {
				return err
			}
		}
	}

	if m.auth != nil {
		if ok, _ := c.Extension("AUTH"); ok {
			if err := c.Auth(m.auth); err != nil {
				return err
			}
		}
	}
	if err := c.Mail(m.cfg.From); err != nil {
		return err
	}
	if err := c.Rcpt(to); err != nil {
		return err
	}
	w, err := c.Data()
	if err != nil {
		return err
	}
	if _, err := w.Write([]byte(msg)); err != nil {
		return err
	}
	if err := w.Close(); err != nil {
		return err
	}
	return c.Quit()
}
