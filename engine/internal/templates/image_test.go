package templates

import "testing"

// TestParseImageRef pins how a template's `image:` string maps onto a registry
// v2 endpoint. The catalogue is not all Docker Hub — Veloren's only maintained
// server image lives on registry.gitlab.com — so the split has to be by host,
// not by assuming Hub and prefixing everything.
func TestParseImageRef(t *testing.T) {
	cases := []struct {
		name     string
		in       string
		registry string
		repo     string
		tag      string
	}{
		{
			name:     "docker hub, namespaced",
			in:       "itzg/minecraft-server:latest",
			registry: "registry-1.docker.io",
			repo:     "itzg/minecraft-server",
			tag:      "latest",
		},
		{
			// Hub's official images live under library/, which the short form hides.
			name:     "docker hub, official image gets the library prefix",
			in:       "postgres:17",
			registry: "registry-1.docker.io",
			repo:     "library/postgres",
			tag:      "17",
		},
		{
			name:     "docker hub, no tag means latest",
			in:       "lloesche/valheim-server",
			registry: "registry-1.docker.io",
			repo:     "lloesche/valheim-server",
			tag:      "latest",
		},
		{
			name:     "third-party registry, nested repository path",
			in:       "registry.gitlab.com/veloren/veloren/server-cli:weekly",
			registry: "registry.gitlab.com",
			repo:     "veloren/veloren/server-cli",
			tag:      "weekly",
		},
		{
			// A host is only a host if it looks like one; "veloren/veloren/x" must
			// not have its first segment mistaken for a registry.
			name:     "registry host needs a dot or a port",
			in:       "a/b/c:1.2",
			registry: "registry-1.docker.io",
			repo:     "a/b/c",
			tag:      "1.2",
		},
		{
			name:     "port in the registry host is not a tag",
			in:       "localhost:5000/team/game:dev",
			registry: "localhost:5000",
			repo:     "team/game",
			tag:      "dev",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			ref, err := ParseImageRef(tc.in)
			if err != nil {
				t.Fatalf("ParseImageRef(%q): unexpected error: %v", tc.in, err)
			}
			if ref.Registry != tc.registry {
				t.Errorf("registry = %q, want %q", ref.Registry, tc.registry)
			}
			if ref.Repository != tc.repo {
				t.Errorf("repository = %q, want %q", ref.Repository, tc.repo)
			}
			if ref.Tag != tc.tag {
				t.Errorf("tag = %q, want %q", ref.Tag, tc.tag)
			}
		})
	}
}

func TestParseImageRefRejectsEmpty(t *testing.T) {
	if _, err := ParseImageRef("  "); err == nil {
		t.Fatal("expected an error for an empty image reference, got nil")
	}
}

// TestParseImageRefManifestURL pins the URL the smoke test will actually call,
// so a bad join (double slash, missing /v2) fails here and not as a confusing
// 404 against a live registry.
func TestParseImageRefManifestURL(t *testing.T) {
	ref, err := ParseImageRef("registry.gitlab.com/veloren/veloren/server-cli:weekly")
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	want := "https://registry.gitlab.com/v2/veloren/veloren/server-cli/manifests/weekly"
	if got := ref.ManifestURL(); got != want {
		t.Errorf("ManifestURL() = %q, want %q", got, want)
	}
}
