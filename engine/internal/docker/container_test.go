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
	want := "run -d --name gamehost-abc123 --restart unless-stopped -i -m 4096m " +
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
	want := "run -d --name gamehost-x --restart unless-stopped -p 2456:2456/tcp alpine"
	if got != want {
		t.Fatalf("RunArgs mismatch:\n got: %s\nwant: %s", got, want)
	}
}
