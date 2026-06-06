// Package templates loads game definitions from YAML files. Each template
// describes how to run one game as a container (image, ports, env, and the
// user-configurable variables shown in the UI). Adding a new game means adding
// a YAML file here — no engine code change required.
package templates

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// Port describes a network port a server listens on.
type Port struct {
	Name      string `yaml:"name" json:"name"`
	Container int    `yaml:"container" json:"container"`
	Protocol  string `yaml:"protocol" json:"protocol"` // tcp | udp
	Default   int    `yaml:"default" json:"default"`   // suggested host port
}

// Variable is a user-configurable setting surfaced in the create-server form.
type Variable struct {
	Key         string   `yaml:"key" json:"key"`
	Label       string   `yaml:"label" json:"label"`
	Description string   `yaml:"description" json:"description"`
	Default     string   `yaml:"default" json:"default"`
	Type        string   `yaml:"type" json:"type"` // string | int | bool | enum
	Options     []string `yaml:"options,omitempty" json:"options,omitempty"`
	Required    bool     `yaml:"required" json:"required"`
}

// Template is a single game definition.
type Template struct {
	ID            string            `yaml:"id" json:"id"`
	Name          string            `yaml:"name" json:"name"`
	Game          string            `yaml:"game" json:"game"`
	Category      string            `yaml:"category" json:"category"`
	Description   string            `yaml:"description" json:"description"`
	Icon          string            `yaml:"icon" json:"icon"`
	SteamAppID    int               `yaml:"steamAppId,omitempty" json:"steamAppId,omitempty"` // for cover art
	Cover         string            `yaml:"cover,omitempty" json:"cover,omitempty"`           // explicit cover URL override
	Image         string            `yaml:"image" json:"image"`
	Runtime       string            `yaml:"runtime" json:"runtime"` // java | steamcmd | custom
	StopCommand   string            `yaml:"stopCommand" json:"stopCommand"`
	DataPath      string            `yaml:"dataPath" json:"dataPath"`           // in-container path to persist (volume mount)
	CommandMethod string            `yaml:"commandMethod" json:"commandMethod"` // rcon-cli | stdin | none
	MinMemoryMB   int               `yaml:"minMemoryMB" json:"minMemoryMB"`
	RecMemoryMB   int               `yaml:"recMemoryMB" json:"recMemoryMB"`
	Ports         []Port            `yaml:"ports" json:"ports"`
	Env           map[string]string `yaml:"env" json:"env"`
	Variables     []Variable        `yaml:"variables" json:"variables"`
}

// Registry is an in-memory, concurrency-safe collection of templates loaded
// from a directory.
type Registry struct {
	dir   string
	mu    sync.RWMutex
	items map[string]Template
}

// NewRegistry creates a registry backed by the given directory.
func NewRegistry(dir string) *Registry {
	return &Registry{dir: dir, items: map[string]Template{}}
}

// Load (re)reads every *.yaml/*.yml file in the directory. It is safe to call
// at runtime to hot-reload templates.
func (r *Registry) Load() error {
	entries, err := os.ReadDir(r.dir)
	if err != nil {
		return fmt.Errorf("read templates dir %q: %w", r.dir, err)
	}

	items := make(map[string]Template)
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		ext := strings.ToLower(filepath.Ext(e.Name()))
		if ext != ".yaml" && ext != ".yml" {
			continue
		}
		b, err := os.ReadFile(filepath.Join(r.dir, e.Name()))
		if err != nil {
			return fmt.Errorf("read template %s: %w", e.Name(), err)
		}
		var t Template
		if err := yaml.Unmarshal(b, &t); err != nil {
			return fmt.Errorf("parse template %s: %w", e.Name(), err)
		}
		if t.ID == "" {
			t.ID = strings.TrimSuffix(e.Name(), filepath.Ext(e.Name()))
		}
		items[t.ID] = t
	}

	r.mu.Lock()
	r.items = items
	r.mu.Unlock()
	return nil
}

// List returns all templates sorted by display name.
func (r *Registry) List() []Template {
	r.mu.RLock()
	defer r.mu.RUnlock()

	out := make([]Template, 0, len(r.items))
	for _, t := range r.items {
		out = append(out, t)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Name < out[j].Name })
	return out
}

// Get returns a single template by ID.
func (r *Registry) Get(id string) (Template, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	t, ok := r.items[id]
	return t, ok
}
