// Package native is a Phase 2 scaffold only.
//
// v0 supports Flutter. Do not call this adapter for production releases yet.
// Detect is disabled; Build returns ErrNotReady except in dry-run.
//
// Planned: Gradle assemble/bundle + xcodebuild for pure Android/iOS apps.
package native

import (
	"context"
	"os"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
)

// Phase is the roadmap slot for this adapter.
const Phase = 2

// Adapter is a structural placeholder for native Android/iOS.
type Adapter struct {
	Runner execx.Runner
}

func New(r execx.Runner) *Adapter {
	if r == nil {
		r = &execx.RealRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	return &Adapter{Runner: r}
}

func (a *Adapter) Name() string { return "native" }

// Detect is intentionally false in v0 so only Flutter is selected.
func (a *Adapter) Detect(projectRoot string) bool {
	_ = projectRoot
	return false
}

func (a *Adapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	_ = ctx
	if opts.DryRun {
		return adapter.BuildArtifact{
			Path:     "build/native-placeholder",
			Platform: opts.Platform,
			Kind:     placeholderKind(opts.Platform),
		}, nil
	}
	return adapter.BuildArtifact{}, ternerrors.New(ternerrors.ClassBuild,
		"native adapter is Phase 2 scaffold only — Flutter is the supported path in v0")
}

func placeholderKind(p config.Platform) string {
	if p == config.PlatformIOS {
		return "app"
	}
	return "aab"
}
