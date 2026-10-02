package mail_test

import (
	"bytes"
	"context"
	"log/slog"
	"strings"
	"testing"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail"
)

func TestLogMailerWritesTheEmail(t *testing.T) {
	t.Parallel()
	var buf bytes.Buffer
	m := mail.Log{Log: slog.New(slog.NewTextHandler(&buf, nil))}
	if err := m.Send(context.Background(), mail.Message{To: "aria@example.com", Subject: "Hello", Text: "Body"}); err != nil || !strings.Contains(buf.String(), "aria@example.com") {
		t.Fatalf("logged = %q %v", buf.String(), err)
	}
}

func TestSMTPRefusesHeaderInjection(t *testing.T) {
	t.Parallel()
	m := mail.SMTP{Addr: "127.0.0.1:1", From: "grimoire@example.com", Now: time.Now}
	if err := m.Send(context.Background(), mail.Message{To: "a@example.com\r\nBcc: x@example.com", Subject: "Hi", Text: "x"}); err == nil || !strings.Contains(err.Error(), "injection") {
		t.Fatalf("err = %v", err)
	}
	if err := m.Send(context.Background(), mail.Message{To: "a@example.com", Subject: "Hi", Text: "x"}); err == nil {
		t.Fatal("no server listens on port 1")
	}
	auth := mail.SMTP{Addr: "127.0.0.1:1", Username: "u", Password: "p", From: "g@example.com", Now: time.Now}
	if err := auth.Send(context.Background(), mail.Message{To: "a@example.com", Subject: "Hi", Text: "x"}); err == nil {
		t.Fatal("no server listens on port 1")
	}
}

// An email with HTML carries the text as its alternative; one without stays plain.
func TestComposeOffersTextAndHTML(t *testing.T) {
	t.Parallel()
	at := time.Date(2026, 10, 2, 12, 0, 0, 0, time.UTC)
	plain := mail.Compose("g@example.com", mail.Message{To: "a@example.com", Subject: "Hi", Text: "one\ntwo"}, at, "b")
	if !strings.Contains(plain, "Content-Type: text/plain; charset=utf-8\r\n\r\none\r\ntwo") || strings.Contains(plain, "multipart") {
		t.Fatalf("plain = %q", plain)
	}
	both := mail.Compose("g@example.com", mail.Message{To: "a@example.com", Subject: "Hi", Text: "text", HTML: "<p>html</p>"}, at, "b1")
	for _, want := range []string{`multipart/alternative; boundary="b1"`, "--b1\r\nContent-Type: text/plain", "text\r\n--b1\r\nContent-Type: text/html", "<p>html</p>\r\n--b1--", "Date: Fri, 02 Oct 2026 12:00:00 +0000"} {
		if !strings.Contains(both, want) {
			t.Errorf("multipart lacks %q", want)
		}
	}
}
