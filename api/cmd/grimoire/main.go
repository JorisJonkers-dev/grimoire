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
	"strings"
	"syscall"
	"time"

	"github.com/JorisJonkers-dev/grimoire/api/db"
	campaignapp "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/app"
	campaignpg "github.com/JorisJonkers-dev/grimoire/api/internal/campaign/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/crosscheck"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/open5e"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/compendium/snapshot"
	identityapp "github.com/JorisJonkers-dev/grimoire/api/internal/identity/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/identity/oidc"
	identitypg "github.com/JorisJonkers-dev/grimoire/api/internal/identity/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/config"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpapi"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/httpx"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/mail"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/pg"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/push"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/rng"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/storage"
	"github.com/JorisJonkers-dev/grimoire/api/internal/platform/webui"
	playapp "github.com/JorisJonkers-dev/grimoire/api/internal/play/app"
	"github.com/JorisJonkers-dev/grimoire/api/internal/play/live"
	playpg "github.com/JorisJonkers-dev/grimoire/api/internal/play/pgstore"
	prepapp "github.com/JorisJonkers-dev/grimoire/api/internal/prep/app"
	preppg "github.com/JorisJonkers-dev/grimoire/api/internal/prep/pgstore"
	"github.com/JorisJonkers-dev/grimoire/api/internal/rules/dice"
	socialapp "github.com/JorisJonkers-dev/grimoire/api/internal/social/app"
	socialpg "github.com/JorisJonkers-dev/grimoire/api/internal/social/pgstore"
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

func mailer(cfg config.Config, logger *slog.Logger) identityapp.Mailer {
	if s := cfg.SMTP; s != nil {
		return mail.SMTP{Addr: s.Addr, Username: s.Username, Password: s.Password, From: s.From, Now: time.Now}
	}
	return mail.Log{Log: logger}
}

func blobs(cfg config.Config, logger *slog.Logger) campaignapp.Blobs {
	if cfg.S3 != nil {
		logger.Info("assets in S3", "endpoint", cfg.S3.Endpoint, "bucket", cfg.S3.Bucket)
		return storage.NewS3(storage.S3Config(*cfg.S3))
	}
	logger.Warn("assets on local disk; set GRIMOIRE_S3_ENDPOINT in production", "dir", cfg.AssetDir)
	return storage.Dir{Path: cfg.AssetDir}
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

	characters := &campaignapp.Characters{
		Repo: campaignpg.New(store.Pool()), Compendium: compendiumStore, Combat: campaignapp.NoCombat{}, Now: time.Now,
		Blobs: blobs(cfg, logger),
	}
	hub := &live.Hub{
		Store: playpg.New(store.Pool()), Stats: playpg.Statblocks{Store: playpg.New(store.Pool()), Characters: characters}, Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())}, Owner: playpg.Owner{Pool: store.Pool()}, Now: time.Now, Log: logger,
		Seed: rng.Seed, Source: func(seed uint64) dice.Source { return rng.New(seed) },
	}
	defer hub.Shutdown()
	var notices httpapi.PushService
	if p := cfg.Push; p != nil {
		sender := &push.Sender{
			Pool: store.Pool(), Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())}, PublicKey: p.PublicKey, PrivateKey: p.PrivateKey,
			Contact: p.Contact, Log: logger,
		}
		defer sender.Wait()
		hub.Notify, notices = sender, sender
	}
	accounts := &identityapp.Service{
		Repo: identitypg.New(store.Pool()), Mailer: mailer(cfg, logger), Passwords: identityapp.DefaultPasswords(), Now: time.Now,
		Admins: map[string]bool{}, BaseURL: cfg.BaseURL, Strong: httpx.Strong,
	}
	for _, s := range cfg.AdminSubjects {
		accounts.Admins[s] = true
	}
	oidcName := ""
	if o := cfg.OIDC; o != nil {
		accounts.OIDC = oidc.New(oidc.Config{
			Issuer: o.Issuer, ClientID: o.ClientID, ClientSecret: o.ClientSecret, RedirectURL: strings.TrimRight(cfg.BaseURL, "/") + "/oidc/callback",
			RolesClaim: o.RolesClaim, HTTP: &http.Client{Timeout: 10 * time.Second},
		})
		accounts.Grant, accounts.AdminRole, oidcName = o.GrantRole, o.AdminRole, o.Name
	}
	social := &socialapp.Service{Repo: socialpg.New(store.Pool()), Now: time.Now}
	handler, err := httpapi.New(httpapi.Options{
		Sessions: accounts, TrustForwardAuth: cfg.TrustForwardAuth,
		Handler: &httpapi.Handler{
			Push: notices, Accounts: accounts, OIDCName: oidcName,
			Friends: social, Conversations: social,
			Version: version, Store: store, Compendium: compendiumStore, Log: logger,
			Campaigns:  campaignapp.NewService(campaignpg.New(store.Pool())),
			Characters: characters,
			NPCs:       &campaignapp.NPCs{Repo: campaignpg.New(store.Pool()), Now: time.Now},
			Sessions: &playapp.Sessions{
				Repo: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())}, Live: hub, Now: time.Now,
			},
			Hub: hub, LiveMembers: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())},
			Prep: &prepapp.Service{
				Repo: preppg.New(store.Pool()), Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())}, Now: time.Now,
				Seed: rng.Seed, Source: func(seed uint64) dice.Source { return rng.New(seed) },
			},
			Maps: &playapp.Maps{
				Repo: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())},
				Blobs: blobs(cfg, logger), Now: time.Now,
			},
			Rolls: &playapp.Rolls{
				Repo: playpg.New(store.Pool()), Members: playpg.CampaignMembers{Store: campaignpg.New(store.Pool())},
				Seed: rng.Seed, Source: func(seed uint64) dice.Source { return rng.New(seed) }, Now: time.Now, Resolved: hub.RollResolved,
			},
		},
		DevSubject:  cfg.DevSubject,
		RateLimit:   cfg.RateLimit,
		Now:         time.Now,
		Web:         webui.Handler(webui.Embedded()),
		Edits:       campaignpg.New(store.Pool()),
		OAuthIssuer: cfg.OAuthIssuer,
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
