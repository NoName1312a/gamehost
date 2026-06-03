package setup

import (
	"context"
	"runtime"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/docker"
)

func TestBuildStartProcess(t *testing.T) {
	got := buildStartProcess("wsl", []string{"--install"})
	want := "Start-Process -FilePath 'wsl' -ArgumentList '--install' -Verb RunAs"
	if got != want {
		t.Fatalf("single arg:\n got %q\nwant %q", got, want)
	}

	got = buildStartProcess("winget", []string{"install", "-e", "--id", "Docker.DockerDesktop"})
	want = "Start-Process -FilePath 'winget' -ArgumentList 'install','-e','--id','Docker.DockerDesktop' -Verb RunAs"
	if got != want {
		t.Fatalf("multi arg:\n got %q\nwant %q", got, want)
	}

	if strings.Contains(buildStartProcess("x", nil), "-ArgumentList") {
		t.Fatal("did not expect -ArgumentList with no args")
	}
}

func TestPSEscape(t *testing.T) {
	if got := psEscape("it's a 'test'"); got != "it''s a ''test''" {
		t.Fatalf("psEscape: got %q", got)
	}
}

func TestRunActionUnknown(t *testing.T) {
	if _, err := RunAction("not-a-real-step"); err == nil {
		t.Fatal("expected an error for an unknown setup step")
	}
}

// TestDetectShape checks the report invariants without depending on what's
// actually installed on the test host (detection is read-only).
func TestDetectShape(t *testing.T) {
	rep := Detect(context.Background(), docker.New())

	if rep.Platform == "" {
		t.Fatal("platform should be set")
	}
	if len(rep.Steps) == 0 {
		t.Fatal("expected at least one step")
	}

	last := rep.Steps[len(rep.Steps)-1]
	if last.ID != stepDockerRunning {
		t.Fatalf("last step = %q, want %q", last.ID, stepDockerRunning)
	}
	if rep.Ready != (last.Status == "ok") {
		t.Fatalf("Ready=%v but docker-running status=%q", rep.Ready, last.Status)
	}
	if runtime.GOOS == "windows" {
		if len(rep.Steps) != 3 || rep.Steps[0].ID != stepWSL2 {
			t.Fatalf("windows report shape wrong: %+v", rep.Steps)
		}
	}
	for _, s := range rep.Steps {
		if s.Status != "ok" && s.Status != "todo" {
			t.Fatalf("step %q has invalid status %q", s.ID, s.Status)
		}
		// A todo step that offers a fix must point at a setup endpoint.
		if s.Action != nil && !strings.HasPrefix(s.Action.Endpoint, "/api/system/setup/") {
			t.Fatalf("step %q action endpoint = %q", s.ID, s.Action.Endpoint)
		}
	}
}
