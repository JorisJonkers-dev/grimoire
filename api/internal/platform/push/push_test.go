package push_test

import (
	"context"
	"crypto/ecdh"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/pgtest"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/push"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

type members map[uuid.UUID]string

func (m members) Member(_ context.Context, _, id uuid.UUID) (domain.Member, error) {
	s, ok := m[id]
	if !ok {
		return domain.Member{}, apperr.ErrNotFound
	}
	return domain.Member{ID: id, Subject: s}, nil
}

// device is a browser's push keys, as PushSubscription.getKey gives them.
func device(t *testing.T) (string, string) {
	t.Helper()
	k, err := ecdh.P256().GenerateKey(rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	secret := make([]byte, 16)
	_, _ = rand.Read(secret)
	return base64.RawURLEncoding.EncodeToString(k.PublicKey().Bytes()), base64.RawURLEncoding.EncodeToString(secret)
}

// pushService records what reaches it and answers with a status per path.
type pushService struct {
	mu     sync.Mutex
	got    []*http.Request
	status map[string]int
}

func (p *pushService) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	_, _ = io.ReadAll(r.Body)
	p.mu.Lock()
	p.got = append(p.got, r)
	code := p.status[r.URL.Path]
	p.mu.Unlock()
	if code == 0 {
		code = http.StatusCreated
	}
	w.WriteHeader(code)
}

// seen copies what has reached the service so far.
func (p *pushService) seen() []*http.Request {
	p.mu.Lock()
	defer p.mu.Unlock()
	return append([]*http.Request{}, p.got...)
}

func TestDevicesHearAboutTurnsUntilTheyGoAway(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store, err := pg.Open(ctx, pgtest.URL(t))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(store.Close)
	priv, pub, _ := webpush.GenerateVAPIDKeys()
	service := &pushService{status: map[string]int{"/gone": http.StatusGone}}
	srv := httptest.NewTLSServer(service)
	t.Cleanup(srv.Close)
	player, stranger := uuid.New(), uuid.New()
	s := &push.Sender{
		Pool: store.Pool(), Members: members{player: "player"}, PublicKey: pub, PrivateKey: priv, Contact: "mailto:dm@example.com",
		Client: srv.Client(), Log: slog.New(slog.NewTextHandler(io.Discard, nil)),
	}
	if s.Key() != pub {
		t.Fatal("key")
	}
	p256dh, auth := device(t)
	if _, err := s.Subscribe(ctx, "player", "http://insecure.example/x", p256dh, auth); !errors.Is(err, push.ErrBadEndpoint) {
		t.Fatalf("http endpoint: %v", err)
	}
	phone, err := s.Subscribe(ctx, "player", srv.URL+"/phone", p256dh, auth)
	if err != nil {
		t.Fatal(err)
	}
	if again, _ := s.Subscribe(ctx, "player", srv.URL+"/phone", p256dh, auth); again != phone {
		t.Fatal("subscribing the same device twice keeps one subscription")
	}
	if _, err := s.Subscribe(ctx, "player", srv.URL+"/gone", p256dh, auth); err != nil {
		t.Fatal(err)
	}
	s.Notify(uuid.New(), player, live.Notice{Title: "Your turn", Body: "Aria is up.", URL: "/x"})
	s.Notify(uuid.New(), stranger, live.Notice{Title: "Your turn"})
	s.Wait()
	if got := service.seen(); len(got) != 2 || !strings.HasPrefix(got[0].Header.Get("Authorization"), "vapid t=") || got[0].Header.Get("Urgency") != "high" {
		t.Fatalf("deliveries = %d", len(got))
	}
	s.Notify(uuid.New(), player, live.Notice{Title: "Reaction"})
	s.Wait()
	if got := service.seen(); len(got) != 3 {
		t.Fatalf("a gone device is forgotten: %d deliveries", len(got))
	}
	if err := s.Unsubscribe(ctx, "someone", phone); !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("someone else's device: %v", err)
	}
	if err := s.Unsubscribe(ctx, "player", phone); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Subscribe(ctx, "player", "https://unreachable.invalid/x", p256dh, auth); err != nil {
		t.Fatal(err)
	}
	s.Client = &http.Client{Transport: refuse{}}
	s.Notify(uuid.New(), player, live.Notice{Title: "Your turn"})
	s.Wait()
	store.Close()
	s.Notify(uuid.New(), player, live.Notice{Title: "Your turn"})
	s.Wait()
}

type refuse struct{}

func (refuse) RoundTrip(*http.Request) (*http.Response, error) { return nil, errors.New("offline") }
