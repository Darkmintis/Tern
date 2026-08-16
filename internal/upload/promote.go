package upload

import (
	"bufio"
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

// PromoteOpts drives a track/stage promotion. Promote never rebuilds, never
// re-signs, and never re-uploads: it points the target track at a release that
// already exists on the source track.
type PromoteOpts struct {
	ProjectRoot string
	// Source is the current stage: a Play track (internal|alpha|beta|production|closed)
	// or testflight.
	Source string
	// Target is the destination stage: a Play track or appstore.
	Target string
	// Rollout carries the release into target as a staged rollout, fraction in
	// (0,1] of users. 0 means 100% immediately.
	Rollout float64
	// PackageName overrides the Android applicationId.
	PackageName string
	// BundleID overrides the iOS bundle id.
	BundleID string
	// ReleaseVersion overrides the iOS marketing version string (e.g. 1.2.3).
	ReleaseVersion string
	// DryRun performs the read-only lookup + conflict check and prints the plan
	// without the confirmation gate or any write.
	DryRun bool
	// Yes is only honored if the operator explicitly passed --yes themselves.
	Yes bool
	// Confirm decides whether to proceed once the plan is printed. When nil, the
	// operator is prompted on stdin and anything but y/yes aborts. This gate
	// behaves identically for humans and AI agents.
	Confirm func(plan PromotePlan) (bool, error)
	// Emitter receives promote_start / promote_plan / promote_end events.
	Emitter *output.Emitter
}

// PromotePlan describes exactly what will be promoted before anything happens.
type PromotePlan struct {
	Platform config.Platform
	Source   string
	Target   string
	Track    string // normalized destination track
	Version  string
	Build    string
	Rollout  float64
	// Conflict warns that the target already holds a newer release.
	Conflict string
	// AppleReview marks iOS plans so operators never think review is bypassed.
	AppleReview bool
}

// Describe renders the human-confirmable summary.
func (p PromotePlan) Describe() string {
	version := strings.TrimSpace(p.Version)
	if version == "" {
		version = "(unknown version)"
	}
	build := strings.TrimSpace(p.Build)
	if build == "" {
		build = "(unknown build)"
	}
	var b strings.Builder
	fmt.Fprintf(&b, "promote %s: %s build %s from %s → %s",
		p.Platform, version, build, p.Source, p.Target)
	if r := p.Rollout; r > 0 && r < 1 {
		fmt.Fprintf(&b, " (staged rollout %.0f%% on arrival)", r*100)
	}
	if p.Conflict != "" {
		fmt.Fprintf(&b, "\nwarning: %s", p.Conflict)
	}
	if p.AppleReview {
		fmt.Fprintf(&b, "\nnote: Apple review is still required — Tern does not and cannot skip it")
	}
	return b.String()
}

// ParsePromoteTargets maps user-level stage names to a platform + normalized
// track pair. iOS: testflight → appstore. Android: Play tracks.
func ParsePromoteTargets(source, target string) (config.Platform, string, string, error) {
	src := normalizeStage(source)
	tgt := normalizeStage(target)
	if src == "" || tgt == "" {
		return "", "", "", ternerrors.New(ternerrors.ClassUpload,
			"promote: empty source or target; use tracks like internal, alpha, beta, production, or testflight → appstore")
	}
	srcIOS := isIOSStage(src)
	tgtIOS := isIOSStage(tgt)
	if srcIOS && tgtIOS {
		return config.PlatformIOS, src, tgt, nil
	}
	if srcIOS || tgtIOS {
		return "", "", "", ternerrors.New(ternerrors.ClassUpload,
			fmt.Sprintf("promote: %q and %q must be both iOS stages (testflight/appstore) or both Android tracks", source, target))
	}
	srcTrack, ok := playTrack(src)
	if !ok {
		return "", "", "", ternerrors.New(ternerrors.ClassUpload,
			fmt.Sprintf("promote: unknown Play track %q (use internal, alpha, beta, production)", source))
	}
	tgtTrack, ok := playTrack(tgt)
	if !ok {
		return "", "", "", ternerrors.New(ternerrors.ClassUpload,
			fmt.Sprintf("promote: unknown Play track %q (use internal, alpha, beta, production)", target))
	}
	return config.PlatformAndroid, srcTrack, tgtTrack, nil
}

// Promote promotes an existing release between tracks/stages.
func (c *Client) Promote(ctx context.Context, opts PromoteOpts) error {
	platform, srcTrack, tgtTrack, err := ParsePromoteTargets(opts.Source, opts.Target)
	if err != nil {
		return err
	}
	em := opts.Emitter
	if em == nil {
		em = output.New(output.ModeHuman)
	}
	em.Emit(output.Event{
		Type:    "promote_start",
		Message: fmt.Sprintf("%s %s → %s", platform, opts.Source, opts.Target),
	})

	switch platform {
	case config.PlatformAndroid:
		return c.promotePlay(ctx, opts, srcTrack, tgtTrack, em)
	case config.PlatformIOS:
		return c.promoteASC(ctx, opts, srcTrack, tgtTrack, em)
	default:
		return ternerrors.New(ternerrors.ClassUpload, "promote: unsupported platform "+string(platform))
	}
}

func (c *Client) promotePlay(ctx context.Context, opts PromoteOpts, srcTrack, tgtTrack string, em *output.Emitter) error {
	pkg := strings.TrimSpace(opts.PackageName)
	if pkg == "" {
		if opts.ProjectRoot != "" {
			var perr error
			pkg, perr = projectmeta.AndroidPackageID(opts.ProjectRoot)
			if perr != nil {
				return perr
			}
		}
	}
	if pkg == "" {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			"promote: Android package name required",
			"set ANDROID_PACKAGE_NAME or run promote from inside the project")
	}

	rel, lerr := c.Play.Lookup(ctx, play.LookupRequest{PackageName: pkg, Track: srcTrack})
	if lerr != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "promote: reading source track "+srcTrack, lerr)
	}
	if !rel.Eligible {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			fmt.Sprintf("promote: no eligible release on source track %q", srcTrack),
			"promote needs a completed (or staged) release on the source track; upload one first (tern ship / tern release)")
	}

	plan := PromotePlan{
		Platform: config.PlatformAndroid,
		Source:   opts.Source,
		Target:   opts.Target,
		Track:    tgtTrack,
		Build:    fmt.Sprintf("%d", rel.VersionCode),
		Rollout:  opts.Rollout,
	}
	plan.Version = strings.TrimSpace(rel.Name)
	if plan.Version == "" && opts.ProjectRoot != "" {
		if fv, ferr := projectmeta.FlutterVersion(opts.ProjectRoot); ferr == nil {
			plan.Version = marketingPart(fv)
		}
	}

	if tr, terr := c.Play.Lookup(ctx, play.LookupRequest{PackageName: pkg, Track: tgtTrack}); terr == nil && tr.Eligible {
		switch {
		case tr.VersionCode == rel.VersionCode:
			em.Emit(output.Event{Type: "promote_plan", Status: "ok", Message: fmt.Sprintf(
				"release build %d is already live on track %q — nothing to promote", rel.VersionCode, tgtTrack)})
			return nil
		case tr.VersionCode > rel.VersionCode:
			plan.Conflict = fmt.Sprintf(
				"target track %q already has a newer release (build %d); promoting build %d will overwrite it",
				tgtTrack, tr.VersionCode, rel.VersionCode)
		}
	}

	planBytes := plan.Describe()
	em.Emit(output.Event{Type: "promote_plan", Status: "ok", Message: planBytes})
	if !opts.DryRun {
		ok, cerr := confirmPromote(opts, plan)
		if cerr != nil {
			return cerr
		}
		if !ok {
			em.Emit(output.Event{Type: "promote_end", Status: "canceled", Message: "promotion canceled by operator"})
			return cancelErr(plan)
		}
		msg, perr := c.Play.Promote(ctx, play.PromoteRequest{
			PackageName:  pkg,
			TargetTrack:  tgtTrack,
			Release:      rel,
			UserFraction: opts.Rollout,
		})
		if perr != nil {
			em.Emit(output.Event{Type: "promote_end", Status: "error", Message: perr.Error()})
			return perr
		}
		em.Emit(output.Event{Type: "promote_end", Status: "ok", Message: msg})
		return nil
	}
	em.Emit(output.Event{Type: "promote_end", Status: "dry_run", Message: "dry-run: would promote as described above"})
	return nil
}

func (c *Client) promoteASC(ctx context.Context, opts PromoteOpts, srcTrack, tgtTrack string, em *output.Emitter) error {
	bundle := strings.TrimSpace(opts.BundleID)
	if bundle == "" {
		if opts.ProjectRoot != "" {
			var berr error
			bundle, berr = projectmeta.IOSBundleID(opts.ProjectRoot)
			if berr != nil {
				return berr
			}
		}
	}
	if bundle == "" {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			"promote: iOS bundle id required",
			"set IOS_BUNDLE_ID or run promote from inside the project")
	}

	build, lerr := c.ASC.Lookup(ctx, asc.LookupRequest{PackageName: bundle})
	if lerr != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "promote: looking up TestFlight build", lerr)
	}

	version := strings.TrimSpace(opts.ReleaseVersion)
	if version == "" && opts.ProjectRoot != "" {
		if fv, ferr := projectmeta.FlutterVersion(opts.ProjectRoot); ferr == nil {
			version = marketingPart(fv)
		}
	}
	if version == "" {
		return ternerrors.NewHint(ternerrors.ClassUpload,
			"promote: App Store version string required",
			"pass --release-version (e.g. 1.2.3) or set the version in pubspec.yaml; build numbers alone are not valid App Store version strings")
	}

	plan := PromotePlan{
		Platform:    config.PlatformIOS,
		Source:      opts.Source,
		Target:      opts.Target,
		Track:       tgtTrack,
		Version:     version,
		Build:       build.BuildNumber,
		AppleReview: true,
	}
	planBytes := plan.Describe()
	em.Emit(output.Event{Type: "promote_plan", Status: "ok", Message: planBytes})

	if !opts.DryRun {
		ok, cerr := confirmPromote(opts, plan)
		if cerr != nil {
			return cerr
		}
		if !ok {
			em.Emit(output.Event{Type: "promote_end", Status: "canceled", Message: "promotion canceled by operator"})
			return cancelErr(plan)
		}
		msg, perr := c.ASC.Promote(ctx, asc.PromoteRequest{
			PackageName:    bundle,
			AppID:          build.AppID,
			BuildID:        build.BuildID,
			BuildNumber:    build.BuildNumber,
			ReleaseVersion: version,
		})
		if perr != nil {
			em.Emit(output.Event{Type: "promote_end", Status: "error", Message: perr.Error()})
			return perr
		}
		em.Emit(output.Event{Type: "promote_end", Status: "ok", Message: msg})
		return nil
	}
	em.Emit(output.Event{Type: "promote_end", Status: "dry_run", Message: "dry-run: would promote as described above"})
	return nil
}

// confirmPromote is the non-negotiable gate. It returns (proceed, error).
// --yes is honored only when the operator passed it explicitly (opts.Yes).
func confirmPromote(opts PromoteOpts, plan PromotePlan) (bool, error) {
	if opts.Yes {
		return true, nil
	}
	if opts.Confirm != nil {
		return opts.Confirm(plan)
	}
	// Default: print the plan on stderr, read stdin. Works identically for
	// humans and AI agents driving the CLI — no silent bypass when non-TTY.
	fmt.Fprint(os.Stderr, plan.Describe()+"\n")
	fmt.Fprint(os.Stderr, "Promote this release to "+plan.Target+"? [y/N]: ")
	line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
	ans := strings.ToLower(strings.TrimSpace(line))
	return ans == "y" || ans == "yes", nil
}

// cancelErr wraps a declined confirmation as a clear, classified outcome.
func cancelErr(plan PromotePlan) error {
	return ternerrors.NewHint(ternerrors.ClassUpload,
		"promotion canceled",
		"re-run and answer yes (y), or pass --yes to promote non-interactively")
}

// marketingPart strips the iOS build suffix from "1.2.3+4" → "1.2.3".
func marketingPart(version string) string {
	if i := strings.LastIndex(version, "+"); i > 0 {
		return version[:i]
	}
	return version
}

// normalizeStage canonicalizes user-facing stage names.
func normalizeStage(s string) string {
	switch strings.ToLower(strings.TrimSpace(s)) {
	case "appstore", "app_store", "app-store":
		return "appstore"
	case "testflight":
		return "testflight"
	case "prod":
		return "production"
	default:
		return strings.ToLower(strings.TrimSpace(s))
	}
}

func isIOSStage(s string) bool {
	return s == "testflight" || s == "appstore"
}

// playTrack maps a user stage name to a real Play track.
func playTrack(s string) (string, bool) {
	switch s {
	case "internal", "alpha", "beta":
		return s, true
	case "closed":
		return "alpha", true // closed testing == alpha track
	case "open":
		return "beta", true // open testing == beta track
	case "production":
		return "production", true
	}
	return "", false
}
