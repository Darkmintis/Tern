package engine

import (
	"context"
	"os"
	"strings"
	"sync"

	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/safety"
)

// playTrackRef is a Play upload destination found in a lane.
type playTrackRef struct {
	Target string
	Track  string
}

func playTracksFromLane(lane config.Lane) []playTrackRef {
	seen := map[string]bool{}
	var out []playTrackRef
	for _, s := range lane.Steps {
		if s.Kind != config.StepUpload && s.Kind != config.StepShip {
			continue
		}
		target := strings.ToLower(strings.TrimSpace(s.UploadTarget))
		if target != "" && target != "play_store" {
			continue
		}
		if target == "" && s.Platform == config.PlatformIOS {
			continue
		}
		if target == "" {
			target = "play_store"
		}
		track := strings.TrimSpace(s.Track)
		if track == "" {
			track = "internal"
		}
		key := target + "|" + track
		if seen[key] {
			continue
		}
		seen[key] = true
		out = append(out, playTrackRef{Target: target, Track: track})
	}
	return out
}

func (e *Engine) ensurePlayVersion(
	ctx context.Context,
	root string,
	target, track string,
	opts Options,
	em *output.Emitter,
) error {
	if e.Upload == nil || e.Upload.Play == nil {
		return nil
	}
	target = strings.ToLower(strings.TrimSpace(target))
	if target != "" && target != "play_store" {
		return nil
	}
	if strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) == "" {
		em.Emit(output.Event{
			Type: "version_check", Status: "skipped",
			Message: "Play version check skipped (GOOGLE_APPLICATION_CREDENTIALS not set)",
		})
		return nil
	}
	if target == "" {
		target = "play_store"
	}
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		Ctx:         ctx,
		ProjectRoot: root,
		Target:      target,
		Track:       track,
		DryRun:      opts.DryRun,
		Yes:         opts.Yes,
		Force:       opts.Force,
		Lookup:      e.Upload.Play.Lookup,
	})
	if err != nil {
		em.Emit(output.Event{
			Type: "version_check", Status: "error",
			Message: err.Error(),
		})
		return err
	}
	status := "ok"
	if res.Skipped {
		status = "skipped"
	} else if res.Bumped {
		status = "bumped"
	}
	em.Emit(output.Event{Type: "version_check", Status: status, Message: res.Message})
	return nil
}

// ensurePlayVersionsBeforeBuilds runs the Play versionCode gate once before
// the first Android release build so a clash fails before flutter build.
func (e *Engine) ensurePlayVersionsBeforeBuilds(
	ctx context.Context,
	root string,
	tracks []playTrackRef,
	opts Options,
	em *output.Emitter,
	done *sync.Once,
) error {
	if len(tracks) == 0 {
		return nil
	}
	var guardErr error
	done.Do(func() {
		if e.Upload == nil || e.Upload.Play == nil {
			return
		}
		if strings.TrimSpace(os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")) == "" {
			em.Emit(output.Event{
				Type: "version_check", Status: "skipped",
				Message: "Play version check skipped (GOOGLE_APPLICATION_CREDENTIALS not set)",
			})
			return
		}
		for _, tr := range tracks {
			res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
				Ctx:         ctx,
				ProjectRoot: root,
				Target:      tr.Target,
				Track:       tr.Track,
				DryRun:      opts.DryRun,
				Yes:         opts.Yes,
				Force:       opts.Force,
				Lookup:      e.Upload.Play.Lookup,
			})
			if err != nil {
				em.Emit(output.Event{Type: "version_check", Status: "error", Message: err.Error()})
				guardErr = err
				return
			}
			status := "ok"
			if res.Skipped {
				status = "skipped"
			} else if res.Bumped {
				status = "bumped"
			}
			em.Emit(output.Event{Type: "version_check", Status: status, Message: res.Message})
		}
	})
	return guardErr
}
