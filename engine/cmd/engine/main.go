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
	"sync"
	"syscall"
	"time"

	"github.com/leop1/gamehost/engine/internal/api"
	"github.com/leop1/gamehost/engine/internal/audit"
	"github.com/leop1/gamehost/engine/internal/auth"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/network"
	"github.com/leop1/gamehost/engine/internal/relay"
	"github.com/leop1/gamehost/engine/internal/remote"
	"github.com/leop1/gamehost/engine/internal/server"
	"github.com/leop1/gamehost/engine/internal/templates"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	rt := docker.New()
	netMapper := network.New()
	relayAgent := relay.New(cfg.DataDir)

	reg := templates.NewRegistry(cfg.TemplatesDir)
	if err := reg.Load(); err != nil {
		// Non-fatal: the panel still boots so the user can see the setup wizard.
		slog.Warn("failed to load game templates", "dir", cfg.TemplatesDir, "err", err)
	} else {
		slog.Info("loaded game templates", "count", len(reg.List()), "dir", cfg.TemplatesDir)
	}

	mgr, err := server.NewManager(cfg.DataDir, rt, netMapper, relayAgent, reg)
	if err != nil {
		slog.Error("failed to initialize server manager", "err", err)
		os.Exit(1)
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

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewRouter(cfg, rt, reg, mgr, netMapper, relayAgent, authStore, remoteCtrl, auditLog),
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Re-open the remote-access HTTPS listener if it was left enabled.
	if err := remoteCtrl.StartIfEnabled(); err != nil {
		slog.Warn("remote access failed to start", "err", err)
	}

	// The relay agent is started on demand by the manager — only while a
	// relay-shared server is actually running — so it's never always-on.

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
	relayAgent.Stop()
	remoteCtrl.Shutdown(ctx)
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
