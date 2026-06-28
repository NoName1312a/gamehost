package docker

import (
	"strings"
	"testing"
)

func TestRunArgs(t *testing.T) {
	spec := CreateSpec{
		Name:      "gamehost-abc123",
		Image:     "itzg/minecraft-server:latest",
		Env:       map[string]string{"EULA": "TRUE", "TYPE": "PAPER", "MEMORY": "4G"},
		Ports:     []PortMapping{{Host: 25565, Container: 25565, Protocol: "tcp"}},
		MemoryMB:  4096,
		Volume:    "gamehost-abc123-data",
		DataPath:  "/data",
		OpenStdin: true,
	}

	got := strings.Join(RunArgs(spec), " ")
	want := "run -d --name gamehost-abc123 --restart on-failure:3 -i -m 6144m " +
		"--pids-limit 4096 --ulimit nofile=1048576:1048576 " +
		"-e EULA=TRUE -e MEMORY=4G -e TYPE=PAPER " +
		"-p 25565:25565/tcp -v gamehost-abc123-data:/data itzg/minecraft-server:latest"

	if got != want {
		t.Fatalf("RunArgs mismatch:\n got: %s\nwant: %s", got, want)
	}
}

func TestRunArgsDefaultsProtocolAndSkipsEmpty(t *testing.T) {
	spec := CreateSpec{
		Name:  "gamehost-x",
		Image: "alpine",
		Ports: []PortMapping{{Host: 2456, Container: 2456}}, // no protocol -> tcp
	}
	got := strings.Join(RunArgs(spec), " ")
	want := "run -d --name gamehost-x --restart on-failure:3 " +
		"--pids-limit 4096 --ulimit nofile=1048576:1048576 -p 2456:2456/tcp alpine"
	if got != want {
		t.Fatalf("RunArgs mismatch:\n got: %s\nwant: %s", got, want)
	}
}

// TestRunArgsResourceLimits verifies an explicit CPU + PID cap and the default
// file-descriptor ulimit are passed to docker run.
func TestRunArgsResourceLimits(t *testing.T) {
	spec := CreateSpec{
		Name:      "gamehost-lim",
		Image:     "itzg/minecraft-server",
		CPUs:      1.5,
		PidsLimit: 512,
	}
	got := strings.Join(RunArgs(spec), " ")
	for _, want := range []string{"--cpus 1.5", "--pids-limit 512", "--ulimit nofile=1048576:1048576"} {
		if !strings.Contains(got, want) {
			t.Errorf("RunArgs missing %q in:\n%s", want, got)
		}
	}
}

// TestRunArgsDefaultPidsLimit verifies a fork-bomb guard is applied by default
// (PidsLimit == 0) without the caller opting in.
func TestRunArgsDefaultPidsLimit(t *testing.T) {
	got := strings.Join(RunArgs(CreateSpec{Name: "x", Image: "alpine"}), " ")
	if !strings.Contains(got, "--pids-limit 4096") {
		t.Errorf("expected default --pids-limit 4096 in:\n%s", got)
	}
}

// TestRunArgsPidsLimitDisabled verifies a negative PidsLimit opts out of the cap.
func TestRunArgsPidsLimitDisabled(t *testing.T) {
	got := strings.Join(RunArgs(CreateSpec{Name: "x", Image: "alpine", PidsLimit: -1}), " ")
	if strings.Contains(got, "--pids-limit") {
		t.Errorf("expected no --pids-limit when disabled, got:\n%s", got)
	}
}

// TestRunArgsNoCPUByDefault verifies CPU is uncapped unless explicitly set, so
// existing single-server installs see no behavior change.
func TestRunArgsNoCPUByDefault(t *testing.T) {
	got := strings.Join(RunArgs(CreateSpec{Name: "x", Image: "alpine"}), " ")
	if strings.Contains(got, "--cpus") {
		t.Errorf("expected no --cpus by default, got:\n%s", got)
	}
}
