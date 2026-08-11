package engine

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/deps"
	"github.com/darkmintis/Tern/internal/fingerprint"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/projectmeta"
)

func (e *Engine) runBuild(
	ctx context.Context,
	ad adapter.Adapter,
	root string,
	step config.Step,
	opts Options,
	artifactsMap map[config.Platform]string,
	mu *sync.Mutex,
	em *output.Emitter,
) (msg, status string, err error) {
	kind := string(step.ArtifactKind)
	if kind == "" {
		if step.Platform == config.PlatformAndroid && step.Mode == config.ModeRelease {
			kind = "aab"
		} else if step.Platform == config.PlatformAndroid {
			kind = "apk"
		} else {
			kind = "ipa"
		}
	}

	fp, _ := fingerprint.Compute(fingerprint.Input{
		ProjectRoot: root,
		Platform:    step.Platform,
		Mode:        step.Mode,
		Kind:        kind,
		Flavor:      step.Flavor,
		Scheme:      step.Scheme,
	})

	if !opts.SkipIncremental && !opts.Clean && !opts.DryRun {
		if rec, lerr := artifacts.Load(root, step.Platform); lerr == nil {
			if rec.Fingerprint == fp && rec.Kind == kind {
				if _, serr := os.Stat(rec.Path); serr == nil {
					mu.Lock()
					artifactsMap[step.Platform] = rec.Path
					mu.Unlock()
					msg = fmt.Sprintf("skip build (inputs unchanged, artifact sha256=%s)", shortHash(rec.SHA256))
					em.Emit(output.Event{Type: "skip", Status: "ok", Message: msg})
					return msg, "skipped", nil
				}
			}
		}
	}

	skipPub, lockSum, _ := deps.ShouldSkipPubGet(root)
	if skipPub {
		em.Emit(output.Event{Type: "deps", Status: "ok", Message: "skip flutter pub get (lockfiles unchanged)"})
	}

	art, berr := ad.Build(ctx, adapter.BuildOptions{
		ProjectRoot:  root,
		Platform:     step.Platform,
		Mode:         step.Mode,
		ArtifactKind: kind,
		Flavor:       step.Flavor,
		Scheme:       step.Scheme,
		SkipPubGet:   skipPub && !opts.DryRun,
		Clean:        opts.Clean,
		DryRun:       opts.DryRun,
	})
	if berr != nil {
		return "", "error", berr
	}

	mu.Lock()
	artifactsMap[step.Platform] = art.Path
	mu.Unlock()
	msg = fmt.Sprintf("built %s (%s)", art.Path, art.Kind)

	if !opts.DryRun {
		ver, _ := projectmeta.FlutterVersion(root)
		_ = artifacts.Save(root, artifacts.Record{
			Platform:    step.Platform,
			Kind:        art.Kind,
			Path:        art.Path,
			Version:     ver,
			Fingerprint: fp,
		})
		if !skipPub {
			_ = deps.MarkResolved(root, lockSum)
		} else if lockSum != "" {
			_ = deps.MarkResolved(root, lockSum)
		}
	}
	return msg, "ok", nil
}
