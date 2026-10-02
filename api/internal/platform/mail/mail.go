// Package mail sends Grimoire's emails: through SMTP when one is configured, otherwise into the log.
package mail

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"net/smtp"
	"strings"
	"time"
)

// Message is one email, as plain text and, when it has one, HTML.
type Message struct {
	To      string
	Subject string
	Text    string
	HTML    string
}

// Log writes each email into the log instead of sending it, for development.
type Log struct {
	Log *slog.Logger
}

// Send logs the email.
func (l Log) Send(ctx context.Context, m Message) error {
	l.Log.InfoContext(ctx, "email not sent: no SMTP server configured", "to", m.To, "subject", m.Subject, "body", m.Text)
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

// Send sends one email.
func (s SMTP) Send(_ context.Context, m Message) error {
	if strings.ContainsAny(m.To+m.Subject, "\r\n") {
		return fmt.Errorf("mail: header injection in %q", m.To)
	}
	host := s.Addr
	if i := strings.LastIndex(host, ":"); i >= 0 {
		host = host[:i]
	}
	raw := make([]byte, 12)
	_, _ = rand.Read(raw)
	var auth smtp.Auth
	if s.Username != "" {
		auth = smtp.PlainAuth("", s.Username, s.Password, host)
	}
	return smtp.SendMail(s.Addr, auth, s.From, []string{m.To}, []byte(Compose(s.From, m, s.Now(), hex.EncodeToString(raw))))
}

// Compose is the raw email: plain text alone, or plain text and HTML as alternatives.
func Compose(from string, m Message, now time.Time, boundary string) string {
	head := "From: Grimoire <" + from + ">\r\nTo: " + m.To + "\r\nSubject: " + m.Subject + "\r\nDate: " + now.UTC().Format(time.RFC1123Z) + "\r\nMIME-Version: 1.0\r\n"
	crlf := func(s string) string { return strings.ReplaceAll(s, "\n", "\r\n") }
	if m.HTML == "" {
		return head + "Content-Type: text/plain; charset=utf-8\r\n\r\n" + crlf(m.Text)
	}
	return head + "Content-Type: multipart/alternative; boundary=\"" + boundary + "\"\r\n\r\n" +
		"--" + boundary + "\r\nContent-Type: text/plain; charset=utf-8\r\n\r\n" + crlf(m.Text) + "\r\n" +
		"--" + boundary + "\r\nContent-Type: text/html; charset=utf-8\r\n\r\n" + crlf(m.HTML) + "\r\n" +
		"--" + boundary + "--\r\n"
}
