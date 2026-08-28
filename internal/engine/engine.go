package engine

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/bump"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/history"
	"github.com/darkmintis/Tern/internal/notify"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/platform"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/signing"
	"github.com/darkmintis/Tern/internal/testrunner"
	"github.com/darkmintis/Tern/internal/upload"
	"golang.org/x/sync/errgroup"
)

// Options for lane execution.
type Options struct {
	ProjectRoot string
	DryRun      bool
	// Force skips pre-upload validation failures.
	Force bool
	// Yes confirms production uploads without a prompt (required in CI).
	Yes bool
	// Clean forces flutter clean before builds.
	Clean bool
	// SkipIncremental disables fingerprint reuse.
	SkipIncremental bool
	// Parallel runs multi-platform builds concurrently when set.
	// Default (nil) is sequential to protect low-RAM local machines.
	Parallel *bool
	Emitter  *output.Emitter
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
		CertSync: &signing.CertSync{Backend: nil},
		Upload:   upload.NewClient(),
	}
}

// RunLane executes a named lane from cfg.
func (e *Engine) RunLane(ctx context.Context, cfg *config.Config, laneName string, opts Options) error {
	// Auto-detect platform tools (ANDROID_HOME, JAVA_HOME, etc.)
	platform.AutoConfigure()

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

	// Save pubspec.yaml before the lane so we can restore it on failure.
	// In dry-run, we always restore (to undo the bump). In real runs, we
	// restore only when the lane fails (so a timeout/error doesn't leave
	// pubspec in a half-bumped state).
	savedPubspec, _ := os.ReadFile(pubspecPath(root))

	// Restore pubspec on Ctrl+C or SIGTERM.
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigCh
		signal.Stop(sigCh)
		e.restorePubspec(root, savedPubspec, em, laneName)
		os.Exit(1)
	}()

	ad, ok := e.Registry.Detect(root)
	if !ok {
		return ternerrors.New(ternerrors.ClassBuild, "no project adapter detected in "+root)
	}

	// Selective platforms: only platforms referenced by the lane are considered.
	platforms := lanePlatforms(lane)
	em.Emit(output.Event{
		Type: "select", Lane: laneName, Status: "ok",
		Message: fmt.Sprintf("platforms=%v", platforms),
	})

	artifactsMap := map[config.Platform]string{}
	var mu sync.Mutex
	playTracks := playTracksFromLane(lane)
	var playVersionOnce sync.Once
	parallelOnce := sync.Once{}

	i := 0
	for i < len(lane.Steps) {
		// Parallel group: consecutive independent build steps for different platforms.
		if lane.Steps[i].Kind == config.StepBuild {
			if err := e.ensurePlayVersionsBeforeBuilds(ctx, root, playTracks, opts, em, &playVersionOnce); err != nil {
				class, _ := ternerrors.AsClass(err)
				em.Emit(output.Event{
					Type: "error", Lane: laneName, Status: "error",
					Message: ternerrors.MessageOf(err), Hint: ternerrors.HintOf(err), ErrorClass: string(class),
				})
				em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
				return err
			}
			group := []config.Step{lane.Steps[i]}
			j := i + 1
			seen := map[config.Platform]bool{lane.Steps[i].Platform: true}
			for j < len(lane.Steps) && lane.Steps[j].Kind == config.StepBuild {
				p := lane.Steps[j].Platform
				if seen[p] {
					break
				}
				seen[p] = true
				group = append(group, lane.Steps[j])
				j++
			}
			if len(group) > 1 {
				parallel := opts.Parallel != nil && *opts.Parallel
				parallelOnce.Do(func() {
					mode := "sequential"
					if parallel {
						mode = "parallel"
					}
					em.Emit(output.Event{
						Type: "parallel_mode", Lane: laneName, Status: mode,
						Message: fmt.Sprintf("multi_platform_builds mode=%s", mode),
					})
				})
				if parallel {
					if err := e.runParallelBuilds(ctx, ad, root, group, opts, artifactsMap, &mu, em, laneName); err != nil {
						class, _ := ternerrors.AsClass(err)
						em.Emit(output.Event{
							Type: "error", Lane: laneName, Status: "error",
							Message: ternerrors.MessageOf(err), Hint: ternerrors.HintOf(err), ErrorClass: string(class),
						})
						em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
						e.restorePubspec(root, savedPubspec, em, laneName)
						return err
					}
					i = j
					continue
				}
				// Sequential: run each build step one by one.
				for _, step := range group {
					if err := e.runStep(ctx, ad, root, step, opts, artifactsMap, &mu, em, laneName, ""); err != nil {
						class, _ := ternerrors.AsClass(err)
						em.Emit(output.Event{
							Type: "error", Lane: laneName, Step: step.Raw,
							Status: "error", Message: ternerrors.MessageOf(err), Hint: ternerrors.HintOf(err), ErrorClass: string(class),
						})
						em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
						e.restorePubspec(root, savedPubspec, em, laneName)
						return err
					}
				}
				i = j
				continue
			}
		}

		step := lane.Steps[i]
		if err := e.runStep(ctx, ad, root, step, opts, artifactsMap, &mu, em, laneName, ""); err != nil {
			class, _ := ternerrors.AsClass(err)
			em.Emit(output.Event{
				Type: "error", Lane: laneName, Step: step.Raw,
				Status: "error", Message: ternerrors.MessageOf(err), Hint: ternerrors.HintOf(err), ErrorClass: string(class),
			})
			em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
			e.restorePubspec(root, savedPubspec, em, laneName)
			// Send failure notification
			if !opts.DryRun && (os.Getenv("TELEGRAM_BOT_TOKEN") != "" || os.Getenv("TERN_TELEGRAM_BOT_TOKEN") != "") {
				version, _ := projectmeta.FlutterVersion(root)
				lastRec, _ := history.Last(root)
				track := "internal"
				platform := "android"
				if lastRec != nil {
					track = lastRec.Track
					platform = string(lastRec.Platform)
				}
				_ = notify.NotifyReleaseFailure(ctx, version, platform, track, err.Error())
			}
			return err
		}
		i++
	}

	em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "ok", DurationMs: time.Since(start).Milliseconds()})
	if !opts.DryRun {
		e.clearReleaseNotes(root, em, laneName)
		// Send success notification if telegram is configured
		if os.Getenv("TELEGRAM_BOT_TOKEN") != "" || os.Getenv("TERN_TELEGRAM_BOT_TOKEN") != "" {
			version, _ := projectmeta.FlutterVersion(root)
			lastRec, _ := history.Last(root)
			track := "internal"
			platform := "android"
			if lastRec != nil {
				track = lastRec.Track
				platform = string(lastRec.Platform)
			}
			_ = notify.NotifyReleaseSuccess(ctx, version, platform, track)
		}
	}
	if opts.DryRun {
		e.restorePubspec(root, savedPubspec, em, laneName)
	}
	return nil
}

func pubspecPath(root string) string {
	return root + "/pubspec.yaml"
}

const releaseNotesTemplate = `<!-- Write release notes for the next version below. -->
<!-- Tern reads this file during upload and clears it after a successful release. -->
<!-- Use one line per entry. Markdown is supported. -->

`

func (e *Engine) clearReleaseNotes(root string, em *output.Emitter, laneName string) {
	path := filepath.Join(root, "RELEASE.md")
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return
	}
	if err := os.WriteFile(path, []byte(releaseNotesTemplate), 0o644); err != nil {
		em.Emit(output.Event{
			Type: "warning", Lane: laneName, Status: "ok",
			Message: "could not clear RELEASE.md: " + err.Error(),
		})
		return
	}
	em.Emit(output.Event{
		Type: "release_notes_cleared", Lane: laneName, Status: "ok",
		Message: "RELEASE.md cleared for next version",
	})
}

func (e *Engine) restorePubspec(root string, saved []byte, em *output.Emitter, laneName string) {
	if saved == nil {
		return
	}
	path := pubspecPath(root)
	if err := os.WriteFile(path, saved, 0o644); err != nil {
		em.Emit(output.Event{
			Type: "error", Lane: laneName, Status: "error",
			Message: "pubspec restore failed: " + err.Error(),
		})
		return
	}
	em.Emit(output.Event{
		Type: "pubspec_restored", Lane: laneName, Status: "ok",
		Message: "pubspec.yaml restored to original state",
	})
}

func lanePlatforms(lane config.Lane) []config.Platform {
	seen := map[config.Platform]bool{}
	var out []config.Platform
	for _, s := range lane.Steps {
		if s.Platform == "" {
			continue
		}
		if !seen[s.Platform] {
			seen[s.Platform] = true
			out = append(out, s.Platform)
		}
	}
	return out
}

func (e *Engine) runParallelBuilds(
	ctx context.Context,
	ad adapter.Adapter,
	root string,
	group []config.Step,
	opts Options,
	artifactsMap map[config.Platform]string,
	mu *sync.Mutex,
	em *output.Emitter,
	laneName string,
) error {
	g, gctx := errgroup.WithContext(ctx)
	pg := "build"
	for _, step := range group {
		step := step
		g.Go(func() error {
			return e.runStep(gctx, ad, root, step, opts, artifactsMap, mu, em, laneName, pg)
		})
	}
	return g.Wait()
}

func (e *Engine) runStep(
	ctx context.Context,
	ad adapter.Adapter,
	root string,
	step config.Step,
	opts Options,
	artifactsMap map[config.Platform]string,
	mu *sync.Mutex,
	em *output.Emitter,
	laneName string,
	parallelGroup string,
) error {
	st := time.Now()
	em.Emit(output.Event{Type: "step_start", Lane: laneName, Step: step.Raw, ParallelGroup: parallelGroup})
	var msg string
	var err error
	status := "ok"

	switch step.Kind {
	case config.StepBuild:
		msg, status, err = e.runBuild(ctx, ad, root, step, opts, artifactsMap, mu, em)
	case config.StepSign:
		mu.Lock()
		art := artifactsMap[step.Platform]
		mu.Unlock()
		res, serr := e.Signing.Sign(ctx, signing.SignOptions{
			Platform:    step.Platform,
			With:        step.SignWith,
			EnvRef:      step.EnvRef,
			Artifact:    art,
			ProjectRoot: root,
			DryRun:      opts.DryRun,
		})
		err = serr
		msg = res.Message
	case config.StepUpload, config.StepShip:
		msg, err = e.runUploadOrShip(ctx, root, step, opts, artifactsMap, mu, em)
	case config.StepBump:
		// Always write to pubspec so version checks and git tags see the
		// real bumped version. In dry-run the caller restores pubspec.yaml
		// after the lane completes.
		res, berr := bump.BumpVersion(root, step.BumpLevel, false)
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
			if step.NotifyVia == "telegram" {
				version, _ := projectmeta.FlutterVersion(root)
				// Get the last release info for context
				lastRec, _ := history.Last(root)
				track := "internal"
				platform := "android"
				if lastRec != nil {
					track = lastRec.Track
					platform = string(lastRec.Platform)
				}
				if terr := notify.NotifyReleaseSuccess(ctx, version, platform, track); terr != nil {
					err = terr
				} else {
					msg = "notification sent to Telegram"
				}
			} else {
				err = ternerrors.NewHint(ternerrors.ClassConfig,
					"unsupported notify channel: "+step.NotifyVia,
					"supported channels: telegram")
			}
		}
	case config.StepTest:
		res, terr := testrunner.Run(ctx, testrunner.Options{
			ProjectRoot: root,
			Command:     step.TestCommand,
		}, em)
		err = terr
		msg = res.Message
	default:
		err = ternerrors.New(ternerrors.ClassConfig, "unsupported step: "+string(step.Kind))
	}

	if opts.DryRun && err == nil && status == "ok" {
		status = "dry_run"
	}
	if err != nil {
		status = "error"
	}
	em.Emit(output.Event{
		Type: "step_end", Lane: laneName, Step: step.Raw,
		Status: status, Message: msg, DurationMs: time.Since(st).Milliseconds(),
		ParallelGroup: parallelGroup,
	})
	return err
}

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}

func (e *Engine) recordRelease(root, laneName string, step config.Step, artPath string, artSHA string) {
	if step.Kind != config.StepUpload && step.Kind != config.StepShip {
		return
	}
	versionStr, err := projectmeta.FlutterVersion(root)
	if err != nil {
		return
	}
	parts := strings.SplitN(versionStr, "+", 2)
	version := versionStr
	build := 0
	if len(parts) == 2 {
		version = parts[0]
		_, _ = fmt.Sscanf(parts[1], "%d", &build)
	}
	rec := history.Record{
		Version:      version,
		Build:        build,
		Platform:     step.Platform,
		Target:       step.UploadTarget,
		Track:        step.Track,
		ArtifactPath: artPath,
		ArtifactSHA:  artSHA,
		ReleasedAt:   time.Now().UTC(),
		Lane:         laneName,
		Rollout:      step.Rollout,
	}
	_ = history.Append(root, rec)
}
