package tunnel

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"
)

func TestLocateFrpcViaEnv(t *testing.T) {
	dir := t.TempDir()
	bin := filepath.Join(dir, "frpc.exe")
	if err := os.WriteFile(bin, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("GAMEHOST_FRPC", bin)
	if got := locate(); got != bin {
		t.Fatalf("locate via env: got %q want %q", got, bin)
	}

	bogus := filepath.Join(dir, "nope.exe")
	t.Setenv("GAMEHOST_FRPC", bogus)
	if locate() == bogus {
		t.Fatal("locate returned a non-existent override")
	}
}

func TestWriteConfigGolden(t *testing.T) {
	cfg := filepath.Join(t.TempDir(), "frpc.toml")
	proxies := []localProxy{
		{Name: "gn-myserver-game", Proto: "udp", LocalPort: 25565, RemotePort: 30001, Secret: "s3cr3t"},
		{Name: "gn-other-game", Proto: "tcp", LocalPort: 2456, RemotePort: 30002, Secret: "other-secret"},
	}
	if err := writeConfig(cfg, "frps.gn.coderaum.com:7000", "frps-tok", proxies); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	got, err := os.ReadFile(cfg)
	if err != nil {
		t.Fatal(err)
	}
	want := `serverAddr = "frps.gn.coderaum.com"
serverPort = 7000
auth.method = "token"
auth.token = "frps-tok"

[[proxies]]
name = "gn-myserver-game"
type = "udp"
localIP = "host.docker.internal"
localPort = 25565
remotePort = 30001
metadatas.gnsecret = "s3cr3t"

[[proxies]]
name = "gn-other-game"
type = "tcp"
localIP = "host.docker.internal"
localPort = 2456
remotePort = 30002
metadatas.gnsecret = "other-secret"
`
	if string(got) != want {
		t.Fatalf("config mismatch:\n--- got ---\n%s\n--- want ---\n%s", got, want)
	}
}

// fakeFrpc re-executes the test binary as a stand-in for frpc, routed to
// TestHelperProcess via the GH_WANT_HELPER env var. Standard os/exec pattern.
func fakeFrpc(name string, args ...string) *exec.Cmd {
	cs := append([]string{"-test.run=TestHelperProcess", "--", name}, args...)
	cmd := exec.Command(os.Args[0], cs...)
	cmd.Env = append(os.Environ(), "GH_WANT_HELPER=1")
	return cmd
}

// TestHelperProcess is not a real test: when GH_WANT_HELPER=1 it impersonates a
// long-running frpc daemon that blocks until its parent kills it.
func TestHelperProcess(t *testing.T) {
	if os.Getenv("GH_WANT_HELPER") != "1" {
		return
	}
	time.Sleep(10 * time.Second) // safety net; the parent kills us first
	os.Exit(0)
}

func TestSidecarStartStop(t *testing.T) {
	orig := execCommand
	execCommand = fakeFrpc
	defer func() { execCommand = orig }()

	sc := &sidecar{bin: "frpc"}
	if err := sc.restart(filepath.Join(t.TempDir(), "frpc.toml")); err != nil {
		t.Fatalf("restart: %v", err)
	}
	if !sc.isRunning() {
		t.Fatal("sidecar should be running after restart")
	}

	sc.stop()
	if sc.isRunning() {
		t.Fatal("sidecar should not be running after stop")
	}
}

func TestSidecarRestartReplacesProcess(t *testing.T) {
	orig := execCommand
	execCommand = fakeFrpc
	defer func() { execCommand = orig }()

	sc := &sidecar{bin: "frpc"}
	cfg := filepath.Join(t.TempDir(), "frpc.toml")
	if err := sc.restart(cfg); err != nil {
		t.Fatalf("first restart: %v", err)
	}
	first := sc.pid()
	if err := sc.restart(cfg); err != nil {
		t.Fatalf("second restart: %v", err)
	}
	if !sc.isRunning() {
		t.Fatal("sidecar should be running after second restart")
	}
	if sc.pid() == first {
		t.Fatal("restart should have spawned a new process")
	}
	sc.stop()
}
