package templates

import (
	"fmt"
	"strings"
)

// dockerHubRegistry is the registry a bare image name resolves against, and
// dockerHubLibrary is the namespace Hub hides from official image names.
const (
	dockerHubRegistry = "registry-1.docker.io"
	dockerHubLibrary  = "library"
)

// ImageRef is a template's `image:` string split into the parts a registry v2
// API call needs.
type ImageRef struct {
	Registry   string // host[:port], e.g. registry-1.docker.io
	Repository string // e.g. itzg/minecraft-server or veloren/veloren/server-cli
	Tag        string // e.g. latest
}

// ParseImageRef splits an image reference into registry, repository and tag.
//
// The first path segment is a registry host only if it looks like one — it has
// a dot or a port, or it is localhost. Otherwise it is part of the repository,
// which is what keeps veloren/veloren/server-cli from losing its first segment
// to a phantom host.
func ParseImageRef(image string) (ImageRef, error) {
	image = strings.TrimSpace(image)
	if image == "" {
		return ImageRef{}, fmt.Errorf("empty image reference")
	}

	registry := dockerHubRegistry
	remainder := image
	if head, rest, ok := strings.Cut(image, "/"); ok && isRegistryHost(head) {
		registry, remainder = head, rest
	}

	tag := "latest"
	if repo, t, ok := strings.Cut(remainder, ":"); ok {
		remainder, tag = repo, t
	}
	if remainder == "" {
		return ImageRef{}, fmt.Errorf("image reference %q has no repository", image)
	}

	// Hub's official images are served from library/, which the short name hides.
	if registry == dockerHubRegistry && !strings.Contains(remainder, "/") {
		remainder = dockerHubLibrary + "/" + remainder
	}

	return ImageRef{Registry: registry, Repository: remainder, Tag: tag}, nil
}

// isRegistryHost reports whether a leading path segment names a registry rather
// than the first part of a repository path.
func isRegistryHost(s string) bool {
	return strings.ContainsAny(s, ".:") || s == "localhost"
}

// ManifestURL is the registry v2 endpoint that answers whether this exact tag
// can be pulled.
func (r ImageRef) ManifestURL() string {
	return fmt.Sprintf("https://%s/v2/%s/manifests/%s", r.Registry, r.Repository, r.Tag)
}

// String renders the reference back to its canonical form.
func (r ImageRef) String() string {
	return fmt.Sprintf("%s/%s:%s", r.Registry, r.Repository, r.Tag)
}
