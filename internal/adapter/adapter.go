package adapter

import (
	"context"

	"github.com/darkmintis/Tern/internal/config"
)

// BuildOptions are passed to Adapter.Build.
type BuildOptions struct {
	ProjectRoot string
	Platform    config.Platform
	Mode        config.Mode
	DryRun      bool
}

// BuildArtifact is the output of a successful build.
type BuildArtifact struct {
	Path     string
	Platform config.Platform
	Kind     string // aab, apk, ipa, xcframework, etc.
}

// Adapter is the framework boundary (ADR 0001).
type Adapter interface {
	Name() string
	Detect(projectRoot string) bool
	Build(ctx context.Context, opts BuildOptions) (BuildArtifact, error)
}

// Registry holds registered adapters in priority order.
type Registry struct {
	adapters []Adapter
}

func NewRegistry(adapters ...Adapter) *Registry {
	return &Registry{adapters: adapters}
}

func (r *Registry) All() []Adapter { return r.adapters }

// Detect returns the first matching adapter for projectRoot.
func (r *Registry) Detect(projectRoot string) (Adapter, bool) {
	for _, a := range r.adapters {
		if a.Detect(projectRoot) {
			return a, true
		}
	}
	return nil, false
}

// ByName finds an adapter by name.
func (r *Registry) ByName(name string) (Adapter, bool) {
	for _, a := range r.adapters {
		if a.Name() == name {
			return a, true
		}
	}
	return nil, false
}
