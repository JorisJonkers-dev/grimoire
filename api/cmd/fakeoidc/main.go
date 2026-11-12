// Command fakeoidc serves a fake OIDC provider shaped like jorisjonkers.dev, so the e2e suite signs in
// through OIDC locally and in CI without reaching the real provider. It is a test double: its sign-in
// page asks for no password, and it must never be deployed.
//
// Its people, with and without the Grimoire grant and with the admin role, are oidctest.People.
package main

import (
	"context"
	"errors"
	"flag"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc/oidctest"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("fakeoidc stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	fs := flag.NewFlagSet("fakeoidc", flag.ContinueOnError)
	addr := fs.String("addr", "127.0.0.1:18766", "address to listen on")
	issuer := fs.String("issuer", "http://127.0.0.1:18766", "the URL the provider is reached at and signs as")
	clientID := fs.String("client-id", oidctest.ClientID, "the one client the provider serves")
	clientSecret := fs.String("client-secret", "", "the client's secret; empty serves a public client")
	redirect := fs.String("redirect-uri", "http://localhost:4173/oidc/callback", "the client's registered redirect URI")
	if err := fs.Parse(args); err != nil {
		return err
	}
	p, err := oidctest.NewProvider(oidctest.Options{Issuer: *issuer, ClientID: *clientID, ClientSecret: *clientSecret, RedirectURL: *redirect, People: oidctest.People()})
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	srv := &http.Server{Addr: *addr, Handler: p.Handler(), ReadHeaderTimeout: 5 * time.Second}
	go func() {
		<-ctx.Done()
		shutdown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		_ = srv.Shutdown(shutdown) //nolint:contextcheck // the serving context is already done
	}()
	logger.Info("fake OIDC provider listening", "addr", *addr, "issuer", *issuer)
	if err := srv.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}
