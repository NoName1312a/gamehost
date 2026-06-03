// Command engine is the GameHost daemon: it drives the container runtime,
// streams server consoles, and exposes a REST + WebSocket API to the UI.
package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/leop1/gamehost/engine/internal/api"
	"github.com/leop1/gamehost/engine/internal/config"
	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	cfg := config.Load()

	rt := docker.New()

	reg := templates.NewRegistry(cfg.TemplatesDir)
	if err := reg.Load(); err != nil {
		// Non-fatal: the panel still boots so the user can see the setup wizard.
		slog.Warn("failed to load game templates", "dir", cfg.TemplatesDir, "err", err)
	} else {
		slog.Info("loaded game templates", "count", len(reg.List()), "dir", cfg.TemplatesDir)
	}

	srv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           api.NewRouter(cfg, rt, reg),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		slog.Info("engine listening", "addr", cfg.Addr)
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("server error", "err", err)
			os.Exit(1)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)
	<-stop

	slog.Info("shutting down")
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		slog.Error("graceful shutdown failed", "err", err)
	}
}
