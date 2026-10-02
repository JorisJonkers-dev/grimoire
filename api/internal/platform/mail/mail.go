// Package mail sends Grimoire's emails: through SMTP when one is configured, otherwise into the log.
package mail

import (
	"context"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// Log writes each email into the log instead of sending it, for development.
type Log struct {
	Log *slog.Logger
}

// Send logs the email.
func (l Log) Send(ctx context.Context, to, subject, body string) error {
	l.Log.InfoContext(ctx, "email not sent: no SMTP server configured", "to", to, "subject", subject, "body", body)
	return nil
}

// SMTP sends email through a server with plain authentication over STARTTLS.
type SMTP struct {
	Addr     string
	Username string
	Password string
	From     string
	Now      func() time.Time
}

// Send sends one plain-text email.
func (s SMTP) Send(_ context.Context, to, subject, body string) error {
	if strings.ContainsAny(to+subject, "\r\n") {
		return fmt.Errorf("mail: header injection in %q", to)
	}
	host := s.Addr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	msg := "From: Grimoire <" + s.From + ">\r\nTo: " + to + "\r\nSubject: " + subject + "\r\nDate: " + s.Now().UTC().Format(time.RFC1123Z) +
		"\r\nMIME-Version: 1.0\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + strings.ReplaceAll(body, "\n", "\r\n")
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, host)
	}
	return smtp.SendMail(s.Addr, auth, s.From, []string{to}, []byte(msg))
}
