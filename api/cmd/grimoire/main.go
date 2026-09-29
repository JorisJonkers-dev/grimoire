// Command grimoire is the composition root: it wires adapters to use cases and serves the API.
package main

import (
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/crosscheck"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/open5e"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
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
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cmd := "serve"
	if len(args) > 0 {
		cmd = args[0]
	}
	switch cmd {
	case "snapshot":
		return writeSnapshot(ctx, args[1:])
	case "crosscheck":
		return crossCheck(ctx, args[1:])
	}
	cfg, err := config.Load(os.Getenv)
	if err != nil {
		return err
	}
	switch cmd {
	case "migrate":
		return pg.Migrate(ctx, cfg.DatabaseURL)
	case "import":
		return importCompendium(ctx, cfg, logger)
	case "serve":
		return serve(ctx, cfg, logger)
	default:
		return fmt.Errorf("unknown command %q (want serve, migrate, import, snapshot or crosscheck)", cmd)
	}
}

// writeSnapshot refreshes the pinned compendium snapshot from Open5e.
func writeSnapshot(ctx context.Context, args []string) error {
	out := "db/seeds/" + snapshot.File
	if len(args) > 0 {
		out = args[0]
	}
	client := open5e.Client{BaseURL: "https://api.open5e.com", HTTP: &http.Client{Timeout: 90 * time.Second}, Backoff: 5 * time.Second}
	snap, err := client.Fetch(ctx)
	if err != nil {
		return err
	}
	raw, err := json.Marshal(snap)
	if err != nil {
		return err
	}
	var buf bytes.Buffer
	zw, _ := gzip.NewWriterLevel(&buf, gzip.BestCompression)
	if _, err := zw.Write(raw); err != nil {
		return err
	}
	if err := zw.Close(); err != nil {
		return err
	}
	return os.WriteFile(out, buf.Bytes(), 0o600) //nolint:gosec // developer command writing where the developer points it
}

func crossCheck(ctx context.Context, args []string) error {
	snap, _, err := snapshot.Load(seeds())
	if err != nil {
		return err
	}
	client := crosscheck.Client{BaseURL: "https://www.dnd5eapi.co", HTTP: &http.Client{Timeout: 60 * time.Second}, Workers: 8}
	report, err := client.Run(ctx, snap)
	if err != nil {
		return err
	}
	out := "../docs/compendium-crosscheck.md"
	if len(args) > 0 {
		out = args[0]
	}
	return os.WriteFile(out, []byte(report.Markdown()), 0o600) //nolint:gosec // developer command writing where the developer points it
}

func importCompendium(ctx context.Context, cfg config.Config, logger *slog.Logger) error {
	store, err := pg.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	return runImport(ctx, pgstore.New(store.Pool()), logger)
}

func runImport(ctx context.Context, store *pgstore.Store, logger *slog.Logger) error {
	snap, hash, err := snapshot.Load(seeds())
	if err != nil {
		return err
	}
	changed, err := store.Import(ctx, snap, hash)
	if err != nil {
		return err
	}
	logger.Info("compendium import", "changed", changed, "spells", len(snap.Spells), "hash", hash[:12])
	return nil
}

func seeds() fs.FS {
	sub, _ := fs.Sub(db.Seeds, "seeds")
	return sub
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
	compendiumStore := pgstore.New(store.Pool())
	if cfg.AutoImport {
		if err := runImport(ctx, compendiumStore, logger); err != nil {
			return err
		}
	}

	handler, err := httpapi.New(httpapi.Options{
		Handler: &httpapi.Handler{
			Version: version, Store: store, Compendium: compendiumStore, Log: logger,
			Campaigns: campaignapp.NewService(campaignpg.New(store.Pool())),
		},
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
