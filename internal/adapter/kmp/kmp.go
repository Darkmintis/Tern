// Package kmp is a Phase 3 scaffold only.
//
// v0 supports Flutter. Do not call this adapter for production releases yet.
//
// Planned: Kotlin Multiplatform Gradle tasks + XCFramework outputs.
package kmp

import (
	"context"
	"os"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
)

// Phase is the roadmap slot for this adapter.
const Phase = 3

// Adapter is a structural placeholder for Kotlin Multiplatform.
type Adapter struct {
	Runner execx.Runner
}

func New(r execx.Runner) *Adapter {
	if r == nil {
		r = &execx.RealRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	return &Adapter{Runner: r}
}

func (a *Adapter) Name() string { return "kmp" }

func (a *Adapter) Detect(projectRoot string) bool {
	_ = projectRoot
	return false
}

func (a *Adapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	_ = ctx
	if opts.DryRun {
		kind := "apk"
		if opts.Platform == config.PlatformIOS {
			kind = "xcframework"
		}
		return adapter.BuildArtifact{Path: "build/kmp-placeholder", Platform: opts.Platform, Kind: kind}, nil
	}
	return adapter.BuildArtifact{}, ternerrors.New(ternerrors.ClassBuild,
		"kmp adapter is Phase 3 scaffold only — Flutter is the supported path in v0")
}
