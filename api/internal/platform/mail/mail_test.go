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
	if err := m.Send(context.Background(), "aria@example.com", "Hello", "Body"); err != nil || !strings.Contains(buf.String(), "aria@example.com") {
		t.Fatalf("logged = %q %v", buf.String(), err)
	}
}

func TestSMTPRefusesHeaderInjection(t *testing.T) {
	t.Parallel()
	m := mail.SMTP{Addr: "127.0.0.1:1", From: "grimoire@example.com", Now: time.Now}
	if err := m.Send(context.Background(), "a@example.com\r\nBcc: x@example.com", "Hi", "x"); err == nil || !strings.Contains(err.Error(), "injection") {
		t.Fatalf("err = %v", err)
	}
	if err := m.Send(context.Background(), "a@example.com", "Hi", "x"); err == nil {
		t.Fatal("no server listens on port 1")
	}
	auth := mail.SMTP{Addr: "127.0.0.1:1", Username: "u", Password: "p", From: "g@example.com", Now: time.Now}
	if err := auth.Send(context.Background(), "a@example.com", "Hi", "x"); err == nil {
		t.Fatal("no server listens on port 1")
	}
}
