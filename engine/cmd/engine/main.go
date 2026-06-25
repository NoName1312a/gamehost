// Command engine is the GameHost daemon: it drives the container runtime,
// streams server consoles, and exposes a REST + WebSocket API to the UI.
package main

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"

	"github.com/leop1/gamehost/engine/internal/account"
	"github.com/leop1/gamehost/engine/internal/api"
	"github.com/leop1/gamehost/engine/internal/audit"
	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/license"
	"github.com/leop1/gamehost/engine/internal/network"
	"github.com/leop1/gamehost/engine/internal/remote"
	"github.com/leop1/gamehost/engine/internal/safe"
	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/telemetry"
	"github.com/leop1/gamehost/engine/internal/templates"
	"github.com/leop1/gamehost/engine/internal/tunnel"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	// Opt-in, off-by-default diagnostics. The endpoint is empty unless configured
	// via GAMEHOST_TELEMETRY_URL; with no endpoint, telemetry is a complete no-op
	// even after opt-in, so nothing leaves the machine.
	telStore := telemetry.NewStore(cfg.DataDir)
	reporter := telemetry.NewReporter(telStore, os.Getenv("GAMEHOST_TELEMETRY_URL"), api.Version)
	safe.OnPanic = reporter.ReportPanic   // report background-goroutine panics
	defer reporter.Recover("engine-main") // and a panic on the main goroutine

	rt := docker.New()
	netMapper := network.New()

	// The built-in GameNest tunnel is on by default (baked relay URL). Set
	// GAMEHOST_TUNNEL_DISABLE=1 to force it off, or GAMEHOST_TUNNEL_URL to
	// override the relay control-plane address.
	var tunAgent *tunnel.Agent
	if url := resolveTunnelURL(os.Getenv); url != "" {
		tunAgent = tunnel.New(cfg.DataDir, url)
	}

	// The GameNest platform account is dormant unless a platform URL is set
	// (GAMENEST_PLATFORM_URL). When unset, no store is created and the account
	// routes report "not configured".
	var acctStore *account.Store
	if url := os.Getenv("GAMENEST_PLATFORM_URL"); url != "" {
		acctStore = account.New(cfg.DataDir, url)
	}

	reg := templates.NewRegistry(cfg.TemplatesDir)
	if err := reg.Load(); err != nil {
		// Non-fatal: the panel still boots so the user can see the setup wizard.
		slog.Warn("failed to load game templates", "dir", cfg.TemplatesDir, "err", err)
	} else {
		slog.Info("loaded game templates", "count", len(reg.List()), "dir", cfg.TemplatesDir)
	}

	mgr, err := server.NewManager(cfg.DataDir, rt, netMapper, reg)
	if err != nil {
		slog.Error("failed to initialize server manager", "err", err)
		os.Exit(1)
	}
	if tunAgent != nil {
		mgr.SetTunnel(api.AdaptTunnel(tunAgent))
	}
	if acctStore != nil {
		mgr.SetAccount(api.AdaptAccount(acctStore))
	}

	authStore, err := auth.New(cfg.DataDir)
	if err != nil {
		slog.Error("failed to initialize auth store", "err", err)
		os.Exit(1)
	}

	remoteCtrl := remote.New(cfg.DataDir, "0.0.0.0")

	auditLog, err := audit.NewFile(cfg.DataDir)
	if err != nil {
		slog.Warn("audit log unavailable", "err", err) // non-fatal
	}

	// The desktop app is fully free & open source — no feature gating. The license
	// store is kept only to recognize optional supporter/hosted keys (no desktop
	// feature depends on it). See engine/internal/license.
	licenseStore := license.NewStore(cfg.DataDir, license.EmbeddedPublicKey())

	srv := &http.Server{
		Addr: cfg.Addr,
		Handler: api.NewRouter(api.Deps{
			Cfg: cfg, RT: rt, Reg: reg, Mgr: mgr, Net: netMapper, Tunnel: tunAgent,
			Auth: authStore, Remote: remoteCtrl, Audit: auditLog, License: licenseStore,
			Telemetry: telStore, Account: acctStore,
		}),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Re-open the remote-access HTTPS listener if it was left enabled.
	if err := remoteCtrl.StartIfEnabled(); err != nil {
		slog.Warn("remote access failed to start", "err", err)
	}

	// Drive per-server daily restart/backup schedules for the engine's lifetime.
	schedCtx, schedCancel := context.WithCancel(context.Background())
	defer schedCancel()
	go mgr.RunScheduler(schedCtx)

	go func() {
		slog.Info("engine listening", "addr", cfg.Addr, "data", cfg.DataDir)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	// Anonymous startup ping (no-op unless opted in + endpoint configured).
	reporter.Send(telemetry.Event{Type: "engine_start", Data: map[string]string{"os": runtime.GOOS}})

	// Shut down on a signal OR when our parent (the desktop shell) exits. The
	// shell spawns us with a stdin pipe and sets GAMEHOST_PARENT_WATCH; when the
	// app dies the pipe closes (stdin EOF), so the engine never orphans. The env
	// gate keeps standalone/dev runs (TTY stdin) from exiting immediately.
	shutdown := make(chan struct{})
	var once sync.Once
	trigger := func(reason string) {
		once.Do(func() {
			slog.Info("shutting down", "reason", reason)
			close(shutdown)
		})
	}

	sigc := make(chan os.Signal, 1)
	signal.Notify(sigc, os.Interrupt, syscall.SIGTERM)
	go func() { <-sigc; trigger("signal") }()

	if os.Getenv("GAMEHOST_PARENT_WATCH") != "" {
		go func() {
			_, _ = io.Copy(io.Discard, os.Stdin) // blocks until the parent closes stdin
			trigger("parent exited")
		}()
	}

	<-shutdown
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	// Remove any UPnP port mappings so they don't linger on the router.
	netMapper.UnmapAll(ctx)
	if tunAgent != nil {
		tunAgent.Stop()
	}
	remoteCtrl.Shutdown(ctx)
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
