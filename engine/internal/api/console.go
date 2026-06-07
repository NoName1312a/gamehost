package api

import (
	"bufio"
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"

	"github.com/leop1/gamehost/engine/internal/safe"
	"github.com/leop1/gamehost/engine/internal/server"
)

// originAllowed reports whether a WebSocket handshake's Origin is acceptable.
// A missing Origin means a non-browser client (the desktop shell, curl) that
// isn't subject to the browser same-origin policy, so it's allowed. A present
// Origin (always sent by browsers) must exactly match the configured allow-list
// — this is what blocks a malicious web page from hijacking the console.
func originAllowed(r *http.Request, allowed []string) bool {
	origin := r.Header.Get("Origin")
	if origin == "" {
		return true
	}
	for _, a := range allowed {
		if strings.EqualFold(origin, a) {
			return true
		}
	}
	return false
}

// console streams a server's live log output over a WebSocket and accepts
// console commands from the UI. Output and command echoes are serialized
// through a single writer goroutine (gorilla requires one concurrent writer).
func (a *API) console(w http.ResponseWriter, r *http.Request) {
	s, ok := a.mgr.Get(chi.URLParam(r, "id"))
	if !ok {
		http.Error(w, "server not found", http.StatusNotFound)
		return
	}

	upgrader := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool { return originAllowed(r, a.cfg.AllowOrigins) },
	}
	conn, err := upgrader.Upgrade(w, r, nil)
	if err != nil {
		return
	}
	defer conn.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	out := make(chan string, 256)

	// single writer — guarded so a write panic can't crash the whole engine.
	safe.Go("console-writer:"+s.ID, func() {
		for {
			select {
			case <-ctx.Done():
				return
			case msg := <-out:
				_ = conn.SetWriteDeadline(time.Now().Add(10 * time.Second))
				if err := conn.WriteMessage(websocket.TextMessage, []byte(msg)); err != nil {
					cancel()
					return
				}
			}
		}
	})

	// log streamer — guarded for the same reason.
	safe.Go("console-logs:"+s.ID, func() {
		reader, err := a.rt.LogsReader(ctx, s.ContainerName(), 200)
		if err != nil {
			send(ctx, out, "[gamehost] could not attach to the console: "+err.Error())
			return
		}
		defer reader.Close()
		sc := bufio.NewScanner(reader)
		sc.Buffer(make([]byte, 0, 64*1024), 1024*1024)
		for sc.Scan() {
			if !send(ctx, out, sc.Text()) {
				return
			}
		}
	})

	// read commands from the UI until the socket closes
	for {
		_, data, err := conn.ReadMessage()
		if err != nil {
			cancel()
			return
		}
		cmd := strings.TrimSpace(string(data))
		if cmd != "" {
			a.handleCommand(ctx, s, cmd, out)
		}
	}
}

func (a *API) handleCommand(ctx context.Context, s *server.Server, cmd string, out chan string) {
	if s.CommandMethod != "rcon-cli" {
		send(ctx, out, "[gamehost] console input isn't supported for this game yet.")
		return
	}
	send(ctx, out, "> "+cmd)
	cctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	res, err := a.rt.Exec(cctx, s.ContainerName(), "rcon-cli", cmd)
	if err != nil {
		send(ctx, out, "[gamehost] command failed: "+err.Error())
		return
	}
	if res != "" {
		send(ctx, out, res)
	}
}

// send pushes a line to the writer goroutine, respecting cancellation.
func send(ctx context.Context, out chan string, msg string) bool {
	select {
	case <-ctx.Done():
		return false
	case out <- msg:
		return true
	}
}
