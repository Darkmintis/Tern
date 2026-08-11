package engine

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/bump"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/deps"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/fingerprint"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/safety"
	"github.com/darkmintis/Tern/internal/signing"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/validate"
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
	Emitter         *output.Emitter
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

	// Selective platforms: only platforms referenced by the lane are considered.
	platforms := lanePlatforms(lane)
	em.Emit(output.Event{
		Type: "select", Lane: laneName, Status: "ok",
		Message: fmt.Sprintf("platforms=%v", platforms),
	})

	artifactsMap := map[config.Platform]string{}
	var mu sync.Mutex

	i := 0
	for i < len(lane.Steps) {
		// Parallel group: consecutive independent build steps for different platforms.
		if lane.Steps[i].Kind == config.StepBuild {
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
				if err := e.runParallelBuilds(ctx, ad, root, group, opts, artifactsMap, &mu, em, laneName); err != nil {
					class, _ := ternerrors.AsClass(err)
					em.Emit(output.Event{
						Type: "error", Lane: laneName, Status: "error",
						Message: ternerrors.MessageOf(err), Hint: ternerrors.HintOf(err), ErrorClass: string(class),
					})
					em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "error", DurationMs: time.Since(start).Milliseconds()})
					return err
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
			return err
		}
		i++
	}

	em.Emit(output.Event{Type: "lane_end", Lane: laneName, Status: "ok", DurationMs: time.Since(start).Milliseconds()})
	return nil
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

func shortHash(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}

func runGitTag(root, prefix string, dryRun bool) (string, error) {
	ver := "0.0.0"
	pub := filepath.Join(root, "pubspec.yaml")
	if data, err := os.ReadFile(pub); err == nil {
		if r, berr := bump.BumpVersion(root, config.BumpPatch, true); berr == nil && r.Old != "" {
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
	s = filepath.Base(s)
	const p = "version:"
	if len(s) > len(p) && s[:len(p)] == p {
		s = s[len(p):]
	}
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t') {
		s = s[1:]
	}
	return s
}
