package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/bump"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/signing"
	"github.com/darkmintis/Tern/internal/upload"
)

// Options for lane execution.
type Options struct {
	ProjectRoot string
	DryRun      bool
	Emitter     *output.Emitter
}

// Engine runs lanes against adapters and shared core services.
type Engine struct {
	Registry *adapter.Registry
	Signing  *signing.Manager
	CertSync *signing.CertSync
	Upload   *upload.Client
}

func New(reg *adapter.Registry) *Engine {
	return &Engine{
		Registry: reg,
		Signing:  signing.NewManager(),
		CertSync: &signing.CertSync{Backend: nil}, // dry-run until Phase 1.5 backend
		Upload:   upload.NewClient(),
	}
}

// RunLane executes a named lane from cfg.
func (e *Engine) RunLane(ctx context.Context, cfg *config.Config, laneName string, opts Options) error {
	lane, ok := cfg.Lane(laneName)
	if !ok {
		return ternerrors.New(ternerrors.ClassConfig, "unknown lane: "+laneName)
	}
	em := opts.Emitter
	if em == nil {
		em = output.New(output.ModeHuman)
	}
	root := opts.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}

	start := time.Now()
	em.Emit(output.Event{Type: "lane_start", Lane: laneName})

	ad, ok := e.Registry.Detect(root)
	if !ok {
		return ternerrors.New(ternerrors.ClassBuild, "no project adapter detected in "+root)
	}

	artifacts := map[config.Platform]string{}

	for _, step := range lane.Steps {
		if err := e.runStep(ctx, ad, root, step, opts, artifacts, em, laneName); err != nil {
			class, _ := ternerrors.AsClass(err)
			em.Emit(output.Event{
				Type: "error", Lane: laneName, Step: step.Raw,
				Status: "error", Message: err.Error(), ErrorClass: string(class),
			})
			em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
			return err
		}
	}

	em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "ok", DurationMs: time.Since(start).Milliseconds()})
	return nil
}

func (e *Engine) runStep(
	ctx context.Context,
	ad adapter.Adapter,
	root string,
	step config.Step,
	opts Options,
	artifacts map[config.Platform]string,
	em *output.Emitter,
	laneName string,
) error {
	st := time.Now()
	em.Emit(output.Event{Type: "step_start", Lane: laneName, Step: step.Raw})
	var msg string
	var err error

	switch step.Kind {
	case config.StepBuild:
		art, berr := ad.Build(ctx, adapter.BuildOptions{
			ProjectRoot: root,
			Platform:    step.Platform,
			Mode:        step.Mode,
			DryRun:      opts.DryRun,
		})
		err = berr
		if err == nil {
			artifacts[step.Platform] = art.Path
			msg = fmt.Sprintf("built %s (%s)", art.Path, art.Kind)
		}
	case config.StepSign:
		res, serr := e.Signing.Sign(ctx, signing.SignOptions{
			Platform:    step.Platform,
			With:        step.SignWith,
			EnvRef:      step.EnvRef,
			Artifact:    artifacts[step.Platform],
			ProjectRoot: root,
			DryRun:      opts.DryRun,
		})
		err = serr
		msg = res.Message
	case config.StepUpload:
		msg, err = e.Upload.Upload(ctx, upload.Options{
			Platform:    step.Platform,
			Target:      step.UploadTarget,
			Track:       step.Track,
			Artifact:    artifacts[step.Platform],
			ProjectRoot: root,
			DryRun:      opts.DryRun,
		})
	case config.StepBump:
		res, berr := bump.BumpVersion(root, step.BumpLevel, opts.DryRun)
		err = berr
		msg = res.Message
	case config.StepTag:
		msg, err = runGitTag(root, step.TagPrefix, opts.DryRun)
	case config.StepSyncCerts:
		if !opts.DryRun && (e.CertSync == nil || e.CertSync.Backend == nil) {
			err = ternerrors.NewHint(ternerrors.ClassSign,
				"sync_certs is not available in v0",
				"remove the sync_certs step from your Ternfile; encrypted cert sync ships in a later release")
			break
		}
		msg, err = e.CertSync.Sync(ctx, signing.SyncOptions{
			Action:  step.SyncAction,
			RepoEnv: step.EnvRef,
			DryRun:  opts.DryRun,
		})
	case config.StepNotify:
		if opts.DryRun {
			msg = fmt.Sprintf("dry-run: would notify %s via env:%s", step.NotifyVia, step.EnvRef)
		} else {
			err = ternerrors.NewHint(ternerrors.ClassConfig,
				"notify is reserved and not implemented yet",
				"remove notify from your Ternfile or wait for a later Tern release")
		}
	default:
		err = ternerrors.New(ternerrors.ClassConfig, "unsupported step: "+string(step.Kind))
	}

	status := "ok"
	if opts.DryRun && err == nil {
		status = "dry_run"
	}
	if err != nil {
		status = "error"
	}
	em.Emit(output.Event{
		Type: "step_end", Lane: laneName, Step: step.Raw,
		Status: status, Message: msg, DurationMs: time.Since(st).Milliseconds(),
	})
	return err
}

func runGitTag(root, prefix string, dryRun bool) (string, error) {
	// Derive version from pubspec if present.
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	if data, err := os.ReadFile(pub); err == nil {
		if r, berr := bump.BumpVersion(root, config.BumpPatch, true); berr == nil && r.Old != "" {
			// Use current version without bumping: parse from dry-run old field
			_ = data
			ver = r.Old
			ver = trimVersionLine(ver)
		}
	}
	tag := bump.TagName(prefix, ver)
	if dryRun {
		return "dry-run: would git tag " + tag, nil
	}
	cmd := exec.Command("git", "tag", tag)
	cmd.Dir = root
	if err := cmd.Run(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassExec, "git tag", err)
	}
	return "created tag " + tag, nil
}

func trimVersionLine(s string) string {
	s = filepath.Base(s) // no-op-ish
	const p = "version:"
	if len(s) > len(p) && s[:len(p)] == p {
		s = s[len(p):]
	}
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}
