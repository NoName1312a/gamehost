package server

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/leop1/gamehost/engine/internal/templates"
)

// newTestManagerWith builds a Manager over a single inline template, so a test
// can pin behaviour against a template shaped for it rather than the shared
// Minecraft fixture.
func newTestManagerWith(t *testing.T, yaml, id string) (*Manager, string) {
	t.Helper()
	tdir := t.TempDir()
	if err := os.WriteFile(filepath.Join(tdir, id+".yaml"), []byte(yaml), 0o644); err != nil {
		t.Fatal(err)
	}
	reg := templates.NewRegistry(tdir)
	if err := reg.Load(); err != nil {
		t.Fatalf("load templates: %v", err)
	}
	dataDir := t.TempDir()
	m, err := NewManager(dataDir, &fakeRuntime{}, nil, reg)
	if err != nil {
		t.Fatalf("new manager: %v", err)
	}
	return m, dataDir
}

// TestExpandArgs pins how a template's command arguments are filled in from a
// server's settings.
//
// The interesting case is an optional setting left blank. A flag whose value
// expands to nothing must take the flag with it: emitting `-pass` with no
// value shifts every argument after it by one, and the game either rejects the
// command line or silently reads the next flag as the password.
func TestExpandArgs(t *testing.T) {
	cases := []struct {
		name string
		args []string
		env  map[string]string
		want []string
	}{
		{
			name: "substitutes a value",
			args: []string{"-autocreate", "${WORLD_SIZE}"},
			env:  map[string]string{"WORLD_SIZE": "3"},
			want: []string{"-autocreate", "3"},
		},
		{
			name: "an empty value drops its flag too",
			args: []string{"-autocreate", "${WORLD_SIZE}", "-pass", "${PASSWORD}"},
			env:  map[string]string{"WORLD_SIZE": "2", "PASSWORD": ""},
			want: []string{"-autocreate", "2"},
		},
		{
			name: "an unset variable behaves like an empty one",
			args: []string{"-players", "${MAXPLAYERS}", "-worldname", "${WORLD_NAME}"},
			env:  map[string]string{"MAXPLAYERS": "8"},
			want: []string{"-players", "8"},
		},
		{
			name: "literal args are untouched",
			args: []string{"-autocreate", "2"},
			env:  map[string]string{},
			want: []string{"-autocreate", "2"},
		},
		{
			name: "a placeholder inside a larger value is substituted in place",
			args: []string{"-world", "/data/${WORLD_NAME}.wld"},
			env:  map[string]string{"WORLD_NAME": "gamehost"},
			want: []string{"-world", "/data/gamehost.wld"},
		},
		{
			name: "no args stays no args",
			args: nil,
			env:  map[string]string{"A": "b"},
			want: nil,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := expandArgs(tc.args, tc.env)
			if strings.Join(got, "\x00") != strings.Join(tc.want, "\x00") {
				t.Errorf("expandArgs(%v) = %v, want %v", tc.args, got, tc.want)
			}
		})
	}
}

const argsTemplate = `
id: test-args
name: Test Args
game: test
image: example/game:latest
dataPath: /data
minMemoryMB: 256
recMemoryMB: 512
args: ["-autocreate", "${WORLD_SIZE}", "-pass", "${PASSWORD}"]
ports:
  - name: game
    container: 7777
    protocol: tcp
    default: 7777
env: {}
variables:
  - key: WORLD_SIZE
    label: World size
    default: "2"
    type: string
    required: false
  - key: PASSWORD
    label: Password
    default: ""
    type: string
    required: false
`

// TestCreateAppliesVariableDefaults covers the half of the problem that is not
// about args at all: a template's variable defaults were never applied, so a
// server created without explicitly passing every variable ran with them
// unset. That is invisible while the image has its own defaults, and fatal the
// moment a required argument is built from one.
func TestCreateAppliesVariableDefaults(t *testing.T) {
	m, _ := newTestManagerWith(t, argsTemplate, "test-args")

	s, err := m.Create(CreateRequest{TemplateID: "test-args", Name: "defaults"})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Env["WORLD_SIZE"] != "2" {
		t.Errorf("WORLD_SIZE = %q, want the template default %q", s.Env["WORLD_SIZE"], "2")
	}
}

func TestCreateLetsAnExplicitValueWinOverTheDefault(t *testing.T) {
	m, _ := newTestManagerWith(t, argsTemplate, "test-args")

	s, err := m.Create(CreateRequest{
		TemplateID: "test-args",
		Name:       "explicit",
		Variables:  map[string]string{"WORLD_SIZE": "3"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if s.Env["WORLD_SIZE"] != "3" {
		t.Errorf("WORLD_SIZE = %q, want the explicit %q", s.Env["WORLD_SIZE"], "3")
	}
}

// TestSpecForExpandsTemplateArgs is the end of the chain: the container command
// the runtime is handed carries the template's args with the server's settings
// filled in.
func TestSpecForExpandsTemplateArgs(t *testing.T) {
	m, _ := newTestManagerWith(t, argsTemplate, "test-args")

	s, err := m.Create(CreateRequest{
		TemplateID: "test-args",
		Name:       "spec",
		Variables:  map[string]string{"WORLD_SIZE": "3"},
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	spec := m.specFor(s)
	if strings.Join(spec.Args, " ") != "-autocreate 3" {
		t.Errorf("spec.Args = %v, want [-autocreate 3] (PASSWORD is blank, so -pass must be dropped)", spec.Args)
	}
}
