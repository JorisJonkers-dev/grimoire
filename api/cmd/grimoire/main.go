// Command grimoire is the composition root: it wires adapters to use cases and serves the API.
package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/config"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/webui"
)

// version is set at build time with -ldflags "-X main.version=…".
var version = "dev"

const shutdownGrace = 10 * time.Second

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	if err := run(os.Args[1:], logger); err != nil {
		logger.Error("grimoire stopped", "error", err)
		os.Exit(1)
	}
}

func run(args []string, logger *slog.Logger) error {
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "migrate":
		return pg.Migrate(ctx, cfg.DatabaseURL)
	case "serve":
		return serve(ctx, cfg, logger)
	default:
		return fmt.Errorf("unknown command %q (want serve or migrate)", cmd)
	}
}

func serve(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	if cfg.AutoMigrate {
		if err := pg.Migrate(ctx, cfg.DatabaseURL); err != nil {
			return err
		}
	}
	store, err := pg.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()

	handler, err := httpapi.New(httpapi.Options{
		Handler:    &httpapi.Handler{Version: version, Store: store, Log: logger},
		DevSubject: cfg.DevSubject,
		RateLimit:  cfg.RateLimit,
		Now:        time.Now,
		Web:        webui.Handler(webui.Embedded()),
	})
	if err != nil {
		return err
	}
	srv := &http.Server{Addr: cfg.Addr, Handler: handler, ReadHeaderTimeout: 5 * time.Second}

	errs := make(chan error, 1)
	go func() {
		logger.Info("listening", "addr", cfg.Addr, "version", version)
		errs <- srv.ListenAndServe()
	}()
	select {
	case err := <-errs:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownGrace)
		defer cancel()
		return srv.Shutdown(shutdownCtx)
	}
}
