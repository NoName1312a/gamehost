// Package config resolves runtime configuration from the environment with
// sensible defaults, so the engine runs out-of-the-box in dev and can be
// pointed at real data/template directories in production.
package config

import (
	"os"
	"path/filepath"
)

// Config holds resolved engine settings.
type Config struct {
	// Addr is the host:port the HTTP API binds to.
	Addr string
	// TemplatesDir is where game definition YAML files live.
	TemplatesDir string
	// DataDir is where per-server volumes and engine state are stored.
	DataDir string
	// AllowOrigins is the CORS allow-list for the browser UI (dev mode).
	AllowOrigins []string
}

// Load reads configuration from the environment, applying defaults.
func Load() Config {
	return Config{
		Addr:         envOr("GAMEHOST_ADDR", "127.0.0.1:8723"),
		TemplatesDir: envOr("GAMEHOST_TEMPLATES", defaultTemplatesDir()),
		DataDir:      envOr("GAMEHOST_DATA", defaultDataDir()),
		AllowOrigins: []string{
			// Vite dev server (browser dev + `tauri dev`).
			"http://localhost:5173",
			"http://127.0.0.1:5173",
			// Tauri desktop webview custom protocol. Windows serves the bundled
			// app from http(s)://tauri.localhost; macOS/Linux use tauri://localhost.
			"http://tauri.localhost",
			"https://tauri.localhost",
			"tauri://localhost",
		},
	}
}

func envOr(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// defaultTemplatesDir locates the game templates. Installed app first: the
// engine binary sits in the install root with the bundled templates at
// resources/templates beside it, and the exe path is the only reliable anchor
// there (cwd depends on how the OS launched us, and the desktop shell's
// GAMEHOST_TEMPLATES env plumbing must not be a single point of failure).
// Dev fallback: the repo's templates/ folder relative to the working directory
// (`go run ./cmd/engine` from engine/ — templates live one level up).
func defaultTemplatesDir() string {
	if exe, err := os.Executable(); err == nil {
		dir := filepath.Dir(exe)
		for _, cand := range []string{
			filepath.Join(dir, "resources", "templates"),
			filepath.Join(dir, "templates"),
		} {
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				return cand
			}
		}
	}
	if wd, err := os.Getwd(); err == nil {
		for _, cand := range []string{
			filepath.Join(wd, "templates"),
			filepath.Join(wd, "..", "templates"),
		} {
			if fi, err := os.Stat(cand); err == nil && fi.IsDir() {
				return cand
			}
		}
	}
	return "templates"
}

func defaultDataDir() string {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "data"
	}
	return filepath.Join(dir, "gamehost", "data")
}
