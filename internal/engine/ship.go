package engine

import (
	"context"
	"fmt"
	"os"
	"sync"

	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/safety"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/validate"
)

// ShipOptions for Engine.Ship.
type ShipOptions struct {
	ProjectRoot string
	Platform    config.Platform
	From        string // last or path
	Target      string
	Track       string
	Rollout     float64
	DryRun      bool
	Force       bool
	Yes         bool
	ReleaseSpec releasemeta.Spec
	Emitter     *output.Emitter
}

func (e *Engine) runUploadOrShip(
	ctx context.Context,
	root string,
	step config.Step,
	opts Options,
	artifactsMap map[config.Platform]string,
	mu *sync.Mutex,
	em *output.Emitter,
) (string, error) {
	from := step.ShipFrom
	if step.Kind == config.StepUpload {
		from = ""
	}
	var artPath string
	if from != "" {
		p, _, rerr := artifacts.ResolvePath(root, step.Platform, from)
		if rerr != nil {
			if opts.DryRun {
				artPath = from
			} else {
				return "", rerr
			}
		} else {
			artPath = p
		}
	} else {
		mu.Lock()
		artPath = artifactsMap[step.Platform]
		mu.Unlock()
		if artPath == "" {
			if p, _, rerr := artifacts.ResolvePath(root, step.Platform, "last"); rerr == nil {
				artPath = p
			}
		}
	}

	if !opts.DryRun {
		vres, verr := validate.Run(validate.Options{
			ProjectRoot: root,
			Platform:    step.Platform,
			Artifact:    artPath,
			Target:      step.UploadTarget,
			Force:       opts.Force,
			Emitter:     em,
		})
		if verr != nil {
			return "", verr
		}
		_ = vres
	}

	if err := safety.ConfirmProduction(safety.ConfirmOpts{
		Target: step.UploadTarget,
		Track:  step.Track,
		Yes:    opts.Yes,
		DryRun: opts.DryRun,
	}); err != nil {
		return "", err
	}

	return e.Upload.Upload(ctx, upload.Options{
		Platform:    step.Platform,
		Target:      step.UploadTarget,
		Track:       step.Track,
		Rollout:     step.Rollout,
		Artifact:    artPath,
		ProjectRoot: root,
		DryRun:      opts.DryRun,
		ReleaseSpec: upload.SpecFromStep(step),
	})
}

// Ship uploads a saved or explicit artifact without rebuilding.
func (e *Engine) Ship(ctx context.Context, opts ShipOptions) error {
	em := opts.Emitter
	if em == nil {
		em = output.New(output.ModeHuman)
	}
	root := opts.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	platform := opts.Platform
	if platform == "" {
		switch opts.Target {
		case "testflight", "app_store":
			platform = config.PlatformIOS
		default:
			platform = config.PlatformAndroid
		}
	}
	from := opts.From
	if from == "" {
		from = "last"
	}
	path, rec, err := artifacts.ResolvePath(root, platform, from)
	if err != nil {
		return err
	}
	em.Emit(output.Event{Type: "ship_start", Message: fmt.Sprintf("%s → %s", path, opts.Target)})

	if !opts.DryRun {
		if _, verr := validate.Run(validate.Options{
			ProjectRoot: root,
			Platform:    platform,
			Artifact:    path,
			Target:      opts.Target,
			Force:       opts.Force,
			Emitter:     em,
		}); verr != nil {
			return verr
		}
	}

	if err := safety.ConfirmProduction(safety.ConfirmOpts{
		Target: opts.Target,
		Track:  opts.Track,
		Yes:    opts.Yes,
		DryRun: opts.DryRun,
	}); err != nil {
		return err
	}

	msg, uerr := e.Upload.Upload(ctx, upload.Options{
		Platform:    platform,
		Target:      opts.Target,
		Track:       opts.Track,
		Rollout:     opts.Rollout,
		Artifact:    path,
		ProjectRoot: root,
		DryRun:      opts.DryRun,
		ReleaseSpec: opts.ReleaseSpec,
	})
	if uerr != nil {
		em.Emit(output.Event{Type: "ship_end", Status: "error", Message: uerr.Error()})
		return uerr
	}
	em.Emit(output.Event{Type: "ship_end", Status: "ok", Message: msg + " sha256=" + shortHash(rec.SHA256)})
	return nil
}
