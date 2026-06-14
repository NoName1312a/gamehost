package templates

import (
	"path/filepath"
	"testing"
)

// TestRealTemplatesLoad parses every bundled game template and fails on any YAML
// error — a typo would otherwise only surface at runtime when a user picks that
// game. It also pins that the templates we added config variables to expose them.
func TestRealTemplatesLoad(t *testing.T) {
	reg := NewRegistry(filepath.Join("..", "..", "..", "templates"))
	if err := reg.Load(); err != nil {
		t.Fatalf("load bundled templates: %v", err)
	}
	all := reg.List()
	if len(all) < 20 {
		t.Fatalf("expected the full template library, got only %d", len(all))
	}
	for _, tpl := range all {
		if tpl.ID == "" || tpl.Image == "" {
			t.Errorf("template %q is missing id or image", tpl.Name)
		}
	}
	// These were zero-variable templates; TPL-1 gave the env-configurable ones
	// real settings.
	for _, id := range []string{"mordhau", "conan", "squad", "insurgency"} {
		tpl, ok := reg.Get(id)
		if !ok {
			t.Errorf("template %q not found", id)
			continue
		}
		if len(tpl.Variables) == 0 {
			t.Errorf("template %q should expose config variables", id)
		}
	}
}
