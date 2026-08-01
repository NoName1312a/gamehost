package server

import (
	"context"
	"fmt"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
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
	// memMB overrides the template's recommended memory. Only for games whose
	// recommendation exceeds what a test machine has — ARK asks for 16 GB, and
	// a container capped above the host's RAM is killed rather than throttled.
	memMB int
	// envVars names environment variables whose values become template
	// variables of the same name. Credentials belong here and not in vars: a
	// third-party API key has no business in the source tree, and a case that
	// needs one skips itself rather than failing when it is absent.
	envVars []string
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
	{
		// MODRINTH_MODPACK has no default and is required, so this is the one
		// template that cannot be created without being told what to run.
		template: "minecraft-modrinth",
		vars:     map[string]string{"MODRINTH_MODPACK": "cobblemon-fabric", "MEMORY": "2G"},
		boot:     15 * time.Minute, // fetches and installs the whole modpack first
	},
	{
		// CurseForge refuses automated downloads without an API key, so this
		// case is skipped unless one is supplied out of band.
		template: "minecraft-curseforge",
		vars:     map[string]string{"MEMORY": "2G"},
		envVars:  []string{"CF_API_KEY", "CF_PAGE_URL"},
		boot:     20 * time.Minute,
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
	// Valheim belongs here, not with the light games: it needs no account, but
	// SteamCMD still fetches the whole server on first start.
	{template: "valheim", boot: 25 * time.Minute},
	{template: "corekeeper", boot: 25 * time.Minute},
	// Bootable at all only since ECO_TOKEN stopped being a required variable:
	// the form used to refuse a server without a token the image never read.
	{template: "eco", boot: 25 * time.Minute},
	{template: "unturned", boot: 25 * time.Minute},
	// 45, not 30: the SteamCMD fetch alone took ~29 minutes here, so the old
	// budget left the game no time to start once it had finished downloading.
	{template: "enshrouded", boot: 45 * time.Minute},
	{template: "palworld", boot: 30 * time.Minute},
	{template: "vrising", boot: 30 * time.Minute},
	{template: "satisfactory", boot: 40 * time.Minute},
	{template: "mordhau", boot: 40 * time.Minute},
	{template: "tf2", boot: 45 * time.Minute},
}

// heavyGamesEnv gates the tier that costs tens of GB per game.
const heavyGamesEnv = "GAMEHOST_GAME_SMOKE_HEAVY"

// heavyGames need no third-party account either, but download tens of GB each
// and several want 8 GB of RAM. Ordered smallest-download first so a run that
// is cut short still produces results. Squad is last because it is ~50 GB on
// its own.
var heavyGames = []gameCase{
	{template: "rust", boot: 45 * time.Minute},
	{template: "soulmask", boot: 45 * time.Minute},
	{template: "insurgency", boot: 45 * time.Minute},
	// 8192 is ARK's own stated minimum; its 16384 recommendation is more than
	// this host has, and Docker kills a container capped above available RAM.
	{template: "ark-ascended", boot: 90 * time.Minute, memMB: 8192},
	{template: "conan", boot: 60 * time.Minute},
	{template: "cs2", boot: 60 * time.Minute},
	{template: "squad", boot: 90 * time.Minute},
}

// TestHeavyGamesBoot is the same check for the tens-of-GB tier.
func TestHeavyGamesBoot(t *testing.T) {
	if os.Getenv(heavyGamesEnv) == "" {
		t.Skipf("downloads tens of GB per game; set %s=1 to run", heavyGamesEnv)
	}
	bootGames(t, heavyGames)
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

			// The pull gets its own budget so the boot deadline measures the
			// game starting up, not the download. Sharing one context between
			// them meant a slow pull ate into the boot window and then expired
			// mid-wait, which surfaced as a bogus "container exited" verdict
			// against a server that was demonstrably alive and saving.
			pullCtx, cancelPull := context.WithTimeout(context.Background(), 60*time.Minute)
			defer cancelPull()
			if err := rt.Pull(pullCtx, tpl.Image, nil); err != nil {
				t.Fatalf("pull %s: %v", tpl.Image, err)
			}

			ctx, cancel := context.WithTimeout(context.Background(), gc.boot+10*time.Minute)
			defer cancel()

			vars := map[string]string{}
			for k, v := range gc.vars {
				vars[k] = v
			}
			for _, key := range gc.envVars {
				v := os.Getenv(key)
				if v == "" {
					t.Skipf("%s needs %s in the environment; not set", gc.template, key)
				}
				vars[key] = v
			}

			req := CreateRequest{TemplateID: gc.template, Name: "smoke", Variables: vars, MemoryMB: gc.memMB}
			// Whatever else is on this machine is not the template's fault. A
			// paused container still owns its host binding, so Minecraft's
			// 25565 is routinely taken on a developer box and the run died at
			// `docker run` with "port is already allocated". Only the host side
			// moves — the container port is unchanged, which is what the
			// listening check looks at.
			if len(tpl.Ports) > 0 && !hostPortFree(tpl.Ports[0].Default) {
				free := freeHostPort(t)
				t.Logf("host port %d is in use; publishing on %d instead", tpl.Ports[0].Default, free)
				req.Port = free
			}

			srv, err := m.Create(req)
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
				// Only a terminal state counts as death. Inspect reports any
				// failure — a daemon hiccup, an expired context — as
				// State{Exists:false} with a blank status, which is
				// indistinguishable from "not running" but is not evidence the
				// container died. Treating every non-Running answer as a crash
				// failed a healthy Enshrouded server on a transient hiccup.
				if st := rt.Inspect(ctx, name); st.Exists && (st.Status == "exited" || st.Status == "dead") {
					t.Fatalf("container %s during boot; logs:\n%s",
						st.Status, containerLogs(name, 60))
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

// hostPortFree reports whether the port can actually be published.
//
// Asking the kernel is not enough. Docker keeps its own allocation table and
// rejects a publish for a port it has already handed to another container even
// when nothing is listening — a paused container still owns its binding. And on
// Docker Desktop the binding lands on IPv6 via wslrelay, so a bind probe on
// 127.0.0.1 succeeds and reports a port free that `docker run` then refuses.
func hostPortFree(port int) bool {
	if dockerPublishedPorts()[port] {
		return false
	}
	l, err := net.Listen("tcp", fmt.Sprintf("127.0.0.1:%d", port))
	if err != nil {
		return false
	}
	l.Close()
	return true
}

// dockerPublishedPorts collects the host ports currently published by
// containers. Entries look like "0.0.0.0:25565->25565/tcp, [::]:25565->25565/tcp".
func dockerPublishedPorts() map[int]bool {
	out, err := exec.Command("docker", "ps", "--format", "{{.Ports}}").CombinedOutput()
	if err != nil {
		return nil
	}
	used := map[int]bool{}
	for _, m := range hostPortRe.FindAllStringSubmatch(string(out), -1) {
		lo, err := strconv.Atoi(m[1])
		if err != nil {
			continue
		}
		hi := lo
		if m[2] != "" {
			if h, err := strconv.Atoi(m[2]); err == nil {
				hi = h
			}
		}
		for p := lo; p <= hi; p++ {
			used[p] = true
		}
	}
	return used
}

// hostPortRe pulls the host side out of a "…:<host>-><container>/proto" mapping.
// Consecutive ports are collapsed into a range — Valheim publishes as
// "2456-2457->2456-2457/udp" — so a single-port pattern would miss both.
var hostPortRe = regexp.MustCompile(`:(\d+)(?:-(\d+))?->`)

// freeHostPort asks the kernel for an unused port.
func freeHostPort(t *testing.T) int {
	t.Helper()
	l, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("find a free port: %v", err)
	}
	defer l.Close()
	return l.Addr().(*net.TCPAddr).Port
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
// It reports every missing port rather than the first. Stopping at the first
// gap hides whether a game opened none of its sockets or all but one, and those
// mean different things: none points at a server that never really started, one
// at a port the template should not be declaring.
func portsAreServing(container string, ports []docker.PortMapping) error {
	var missing []string
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
			missing = append(missing, fmt.Sprintf("%d/%s", p.Container, proto))
		}
	}
	if len(missing) > 0 {
		return fmt.Errorf("nothing listening on %d of %d declared ports: %s",
			len(missing), len(ports), strings.Join(missing, ", "))
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
