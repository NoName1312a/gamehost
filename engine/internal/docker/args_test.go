package docker

import (
	"strings"
	"testing"
)

// TestRunArgsAppendsCommandArgsAfterImage pins that a template's command
// arguments land where Docker expects them — after the image, as the
// container's command — and not among the `docker run` flags.
//
// Some game images take required settings only as command arguments: the
// Terraria image refuses to create a world without `-autocreate`, so without
// this the game cannot be started at all.
func TestRunArgsAppendsCommandArgsAfterImage(t *testing.T) {
	spec := CreateSpec{
		Name:  "gamehost-abc",
		Image: "ryshe/terraria:latest",
		Ports: []PortMapping{{Host: 7777, Container: 7777, Protocol: "tcp"}},
		Args:  []string{"-autocreate", "2", "-players", "8"},
	}

	got := RunArgs(spec)
	joined := strings.Join(got, " ")

	imageIdx := -1
	for i, a := range got {
		if a == "ryshe/terraria:latest" {
			imageIdx = i
			break
		}
	}
	if imageIdx < 0 {
		t.Fatalf("image missing from args: %s", joined)
	}
	tail := strings.Join(got[imageIdx+1:], " ")
	if tail != "-autocreate 2 -players 8" {
		t.Errorf("command args after the image = %q, want %q", tail, "-autocreate 2 -players 8")
	}
}

func TestRunArgsWithoutCommandArgsEndsAtTheImage(t *testing.T) {
	spec := CreateSpec{Name: "gamehost-x", Image: "alpine"}
	got := RunArgs(spec)
	if got[len(got)-1] != "alpine" {
		t.Errorf("last arg = %q, want the image to be last when no command args are set", got[len(got)-1])
	}
}
