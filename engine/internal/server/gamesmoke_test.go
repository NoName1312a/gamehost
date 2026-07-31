package server

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/leop1/gamehost/engine/internal/docker"
	"github.com/leop1/gamehost/engine/internal/templates"
)

// gameSmokeEnv opts a run in to Docker. Off by default: this pulls real images
// and boots real game servers, which needs a working daemon, several GB of disk
// and minutes of wall clock.
const gameSmokeEnv = "GAMEHOST_GAME_SMOKE"

// gameCase is one game the smoke run boots for real.
//
// The registry smoke test in internal/templates only proves an image still
// resolves. It cannot see a wrong dataPath, a missing port or an env var the
// image ignores — all three of which Veloren shipped with, and only one of
// which was the dead image reference. This is the test that catches the rest,
// because it asks the running container what it actually did.
type gameCase struct {
	template string
	vars     map[string]string
	// boot is how long the server may take before it must be listening. First
	// boots generate worlds and sometimes download a server jar, so these are
	// per-game rather than one global timeout.
	boot time.Duration
}

// easyGames are the templates that need no third-party account and fit
// comfortably in a normal machine's RAM. Games gated behind a token (Don't
// Starve Together, Eco, CurseForge modpacks) and the very heavy ones (ARK,
// Conan, Rust, Squad) are deliberately not here — they need credentials or
// tens of GB, so they belong in a separate, slower run.
var easyGames = []gameCase{
	{template: "veloren", boot: 4 * time.Minute},
	{template: "terraria", boot: 4 * time.Minute},
	{template: "openttd", boot: 3 * time.Minute},
	{template: "vintagestory", boot: 5 * time.Minute},
	{template: "factorio", boot: 4 * time.Minute},
	{template: "necesse", boot: 4 * time.Minute},
	{template: "minecraft-bedrock", boot: 5 * time.Minute},
	{
		template: "minecraft-java-paper",
		vars:     map[string]string{"VERSION": "1.21.4", "TYPE": "PAPER", "MEMORY": "1G"},
		boot:     8 * time.Minute, // downloads the server jar, then generates a world
	},
}

// mediumGamesEnv gates the SteamCMD tier separately from the easy one. These
// download the game itself on first boot — tens of GB for the Source titles —
// so a run costs hours and real disk, and nobody should trip it by accident
// while trying to check the light games.
const mediumGamesEnv = "GAMEHOST_GAME_SMOKE_MEDIUM"

// mediumGames need no third-party account either, but fetch the game through
// SteamCMD when the container first starts. The boot budgets are download
// budgets, not startup budgets — the image pull is the small part.
var mediumGames = []gameCase{
	{template: "corekeeper", boot: 25 * time.Minute},
	{template: "unturned", boot: 25 * time.Minute},
	{template: "enshrouded", boot: 30 * time.Minute},
	{template: "palworld", boot: 30 * time.Minute},
	{template: "vrising", boot: 30 * time.Minute},
	{template: "satisfactory", boot: 40 * time.Minute},
	{template: "mordhau", boot: 40 * time.Minute},
	{template: "tf2", boot: 45 * time.Minute},
}

// TestEasyGamesBoot creates each game through the real Manager and the real
// Docker runtime — the same path the app takes — then proves the server is
// actually serving: every port the template publishes has a listening socket,
// and the data volume the template points at is where the game wrote.
func TestEasyGamesBoot(t *testing.T) {
	if os.Getenv(gameSmokeEnv) == "" {
		t.Skipf("boots real containers; set %s=1 to run", gameSmokeEnv)
	}
	bootGames(t, easyGames)
}

// TestMediumGamesBoot is the same check for the SteamCMD tier.
func TestMediumGamesBoot(t *testing.T) {
	if os.Getenv(mediumGamesEnv) == "" {
		t.Skipf("downloads tens of GB; set %s=1 to run", mediumGamesEnv)
	}
	bootGames(t, mediumGames)
}

func bootGames(t *testing.T, games []gameCase) {
	t.Helper()

	rt := docker.New()
	reg := templates.NewRegistry(filepath.Join("..", "..", "..", "templates"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load bundled templates: %v", err)
	}

	for _, gc := range games {
		t.Run(gc.template, func(t *testing.T) {
			tpl, ok := reg.Get(gc.template)
			if !ok {
				t.Fatalf("template %q not in the bundled catalogue", gc.template)
			}

			m, err := NewManager(t.TempDir(), rt, nil, reg)
			if err != nil {
				t.Fatalf("new manager: %v", err)
			}

			// Pull first so the boot deadline measures the game starting up,
			// not the download.
			ctx, cancel := context.WithTimeout(context.Background(), 30*time.Minute)
			defer cancel()
			if err := rt.Pull(ctx, tpl.Image, nil); err != nil {
				t.Fatalf("pull %s: %v", tpl.Image, err)
			}

			srv, err := m.Create(CreateRequest{TemplateID: gc.template, Name: "smoke", Variables: gc.vars})
			if err != nil {
				t.Fatalf("create: %v", err)
			}
			t.Cleanup(func() {
				cleanupCtx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
				defer cancel()
				_ = m.Delete(cleanupCtx, srv.ID)
			})

			if err := m.Start(ctx, srv.ID); err != nil {
				t.Fatalf("start: %v", err)
			}

			name := srv.ContainerName()
			deadline := time.Now().Add(gc.boot)
			var lastErr error
			for time.Now().Before(deadline) {
				if st := rt.Inspect(ctx, name); !st.Running {
					t.Fatalf("container exited during boot (status %q); logs:\n%s",
						st.Status, containerLogs(name, 40))
				}
				if lastErr = portsAreServing(name, srv.Ports); lastErr == nil {
					break
				}
				time.Sleep(5 * time.Second)
			}
			if lastErr != nil {
				t.Fatalf("not serving within %s: %v; logs:\n%s", gc.boot, lastErr, containerLogs(name, 40))
			}

			// A template whose dataPath is wrong still boots — it just writes
			// into the container layer, so every world is silently discarded on
			// the next recreate. Only the volume proves it.
			if err := volumeHasData(name, srv.DataPath); err != nil {
				t.Errorf("dataPath %q: %v", srv.DataPath, err)
			}
			t.Logf("%s: serving %s, data under %s", gc.template, portList(srv.Ports), srv.DataPath)
		})
	}
}

func portList(ports []docker.PortMapping) string {
	parts := make([]string, 0, len(ports))
	for _, p := range ports {
		parts = append(parts, fmt.Sprintf("%d/%s", p.Container, p.Protocol))
	}
	return strings.Join(parts, " ")
}

// portsAreServing checks that the process inside the container holds a socket
// on every port the template declares.
//
// It reads /proc/net directly instead of running netstat, because not every
// game image ships net-tools, and it checks inside the container rather than
// dialing from the host because a UDP port cannot be probed by connecting to
// it — an unbound UDP port accepts a datagram just as silently as a bound one.
func portsAreServing(container string, ports []docker.PortMapping) error {
	for _, p := range ports {
		proto := p.Protocol
		if proto == "" {
			proto = "tcp"
		}
		files := []string{"/proc/net/" + proto, "/proc/net/" + proto + "6"}
		out, err := exec.Command("docker", "exec", container,
			"cat", files[0], files[1]).CombinedOutput()
		if err != nil && len(out) == 0 {
			return fmt.Errorf("read %s inside container: %v", files, err)
		}
		if !hasListener(string(out), p.Container, proto) {
			return fmt.Errorf("nothing listening on %d/%s", p.Container, proto)
		}
	}
	return nil
}

// hasListener parses /proc/net/{tcp,udp} for a socket bound to port. The
// local_address column is HEXIP:HEXPORT; for TCP the state column must be 0A
// (LISTEN), while a UDP socket is bound as soon as it appears.
func hasListener(procNet string, port int, proto string) bool {
	want := strings.ToUpper(strconv.FormatInt(int64(port), 16))
	if len(want) < 4 {
		want = strings.Repeat("0", 4-len(want)) + want
	}
	for _, line := range strings.Split(procNet, "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 {
			continue
		}
		_, hexPort, ok := strings.Cut(fields[1], ":")
		if !ok || !strings.EqualFold(hexPort, want) {
			continue
		}
		if proto == "udp" || strings.EqualFold(fields[3], "0A") {
			return true
		}
	}
	return false
}

// volumeHasData reports whether the game wrote anything under the mount point
// the template declares as its data path.
func volumeHasData(container, dataPath string) error {
	out, err := exec.Command("docker", "exec", container,
		"sh", "-c", "ls -A "+dataPath+" 2>/dev/null | head -20").CombinedOutput()
	if err != nil && len(out) == 0 {
		return fmt.Errorf("list: %v", err)
	}
	if strings.TrimSpace(string(out)) == "" {
		return fmt.Errorf("nothing was written there — the game stores its data elsewhere, so worlds would be lost on recreate")
	}
	return nil
}

func containerLogs(container string, lines int) string {
	out, _ := exec.Command("docker", "logs", "--tail", strconv.Itoa(lines), container).CombinedOutput()
	return string(out)
}
