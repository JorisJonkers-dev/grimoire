// Package push tells players' devices about their turn and Reaction Prompts with Web Push, so a
// locked phone still hears about them.
package push

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg/queries"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/domain"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	"github.com/JorisJonkers-dev/grimoire/api/internal/shared/apperr"
)

// Members finds who a Campaign member is.
type Members interface {
	Member(ctx context.Context, campaign, id uuid.UUID) (domain.Member, error)
}

// Sender keeps devices' subscriptions and delivers Notices to them.
type Sender struct {
	Pool    *pgxpool.Pool
	Members Members
	// PublicKey and PrivateKey are the server's VAPID keys; Contact is a mailto: or https: address for push services.
	PublicKey  string
	PrivateKey string
	Contact    string
	// Client sends to push services; nil means the default HTTP client.
	Client webpush.HTTPClient
	Log    *slog.Logger

	wg sync.WaitGroup
}

// ErrBadEndpoint refuses a subscription whose endpoint is not an https URL.
var ErrBadEndpoint = apperr.Refuse("A push endpoint must be an https URL.")

// Key is the VAPID public key a browser subscribes with.
func (s *Sender) Key() string { return s.PublicKey }

// Subscribe keeps a device's subscription for the account that made it.
func (s *Sender) Subscribe(ctx context.Context, subject, endpoint, p256dh, auth string) (uuid.UUID, error) {
	if u, err := url.Parse(endpoint); err != nil || u.Scheme != "https" || u.Host == "" {
		return uuid.UUID{}, ErrBadEndpoint
	}
	return queries.New(s.Pool).UpsertPushSubscription(ctx, queries.UpsertPushSubscriptionParams{Subject: subject, Endpoint: endpoint, P256dh: p256dh, Auth: auth})
}

// Unsubscribe forgets one of the account's devices.
func (s *Sender) Unsubscribe(ctx context.Context, subject string, id uuid.UUID) error {
	n, err := queries.New(s.Pool).DeletePushSubscription(ctx, queries.DeletePushSubscriptionParams{ID: id, Subject: subject})
	if err == nil && n == 0 {
		return apperr.ErrNotFound
	}
	return err
}

// Notify delivers a Notice to every device of a Campaign member, in the background.
func (s *Sender) Notify(campaign, member uuid.UUID, n live.Notice) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.deliver(ctx, campaign, member, n); err != nil {
			s.Log.Warn("push: deliver", "error", err)
		}
	}()
}

// Push wakes the devices a subject opted in on, for an Account's Notification.
func (s *Sender) Push(subject, title, body, url string) {
	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
		defer cancel()
		if err := s.deliverTo(ctx, subject, live.Notice{Title: title, Body: body, URL: url}); err != nil {
			s.Log.Warn("push: deliver", "error", err)
		}
	}()
}

// Wait blocks until every Notice in flight has been delivered or given up on.
func (s *Sender) Wait() { s.wg.Wait() }

func (s *Sender) deliver(ctx context.Context, campaign, member uuid.UUID, n live.Notice) error {
	m, err := s.Members.Member(ctx, campaign, member)
	if err != nil {
		return err
	}
	return s.deliverTo(ctx, m.Subject, n)
}

func (s *Sender) deliverTo(ctx context.Context, subject string, n live.Notice) error {
	q := queries.New(s.Pool)
	subs, err := q.PushSubscriptions(ctx, subject)
	if err != nil {
		return err
	}
	payload, _ := json.Marshal(map[string]string{"title": n.Title, "body": n.Body, "url": n.URL})
	var errs []error
	for _, sub := range subs {
		res, err := webpush.SendNotificationWithContext(ctx, payload, &webpush.Subscription{Endpoint: sub.Endpoint, Keys: webpush.Keys{P256dh: sub.P256dh, Auth: sub.Auth}}, &webpush.Options{
			HTTPClient: s.Client, Subscriber: s.Contact, VAPIDPublicKey: s.PublicKey, VAPIDPrivateKey: s.PrivateKey, TTL: 120, Urgency: webpush.UrgencyHigh,
		})
		if err != nil {
			errs = append(errs, err)
			continue
		}
		_ = res.Body.Close()
		if res.StatusCode == http.StatusNotFound || res.StatusCode == http.StatusGone {
			errs = append(errs, q.DropPushEndpoint(ctx, sub.Endpoint))
		}
	}
	return errors.Join(errs...)
}
