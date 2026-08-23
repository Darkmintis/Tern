package safety

import (
	"context"
	"fmt"
	"strings"

	"github.com/darkmintis/Tern/internal/bump"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/upload/play"
)

// PlayVersionOpts controls the pre-upload Play versionCode gate.
type PlayVersionOpts struct {
	Ctx         context.Context
	ProjectRoot string
	Target      string
	Track       string
	PackageName string
	DryRun      bool
	// Yes auto-bumps without prompting (also required in CI / non-TTY).
	Yes bool
	// Force skips the check entirely.
	Force bool
	// Lookup reads the newest eligible release on a track.
	Lookup func(ctx context.Context, req play.LookupRequest) (play.SourceRelease, error)
	// BumpPast applies BumpPastStore; defaults to bump.BumpPastStore.
	BumpPast func(projectRoot string, storeVC int64, dryRun bool) (bump.Result, error)
	Prompt   func(question string) (string, error)
	IsCI     *bool
	IsTTY    *bool
}

// PlayVersionResult describes what the gate decided.
type PlayVersionResult struct {
	Skipped bool
	Bumped  bool
	Message string
	StoreVC int64
	LocalVC int64
	Local   string
	Store   string
}

// EnsurePlayVersionAhead refuses or bumps when local versionCode is not
// strictly greater than the newest eligible release on the Play track.
// Non-Play targets are skipped. Empty / never-published tracks are OK.
func EnsurePlayVersionAhead(opts PlayVersionOpts) (PlayVersionResult, error) {
	out := PlayVersionResult{}
	target := strings.ToLower(strings.TrimSpace(opts.Target))
	if opts.Force || (target != "" && target != "play_store") {
		out.Skipped = true
		out.Message = "play version check skipped"
		return out, nil
	}
	if opts.Lookup == nil {
		out.Skipped = true
		out.Message = "play version check skipped (no lookup)"
		return out, nil
	}

	root := opts.ProjectRoot
	pkg := strings.TrimSpace(opts.PackageName)
	if pkg == "" {
		id, err := projectmeta.AndroidPackageID(root)
		if err != nil || id == "" {
			out.Skipped = true
			out.Message = "play version check skipped (no Android package id)"
			return out, nil
		}
		pkg = id
	}

	local, err := projectmeta.FlutterLocalVersion(root)
	if err != nil {
		return out, err
	}
	out.Local = local.Raw
	out.LocalVC = local.Code

	track := strings.TrimSpace(opts.Track)
	if track == "" {
		track = "internal"
	}
	ctx := opts.Ctx
	if ctx == nil {
		ctx = context.Background()
	}
	store, err := opts.Lookup(ctx, play.LookupRequest{PackageName: pkg, Track: track})
	if err != nil {
		return out, ternerrors.WrapHint(ternerrors.ClassUpload,
			"could not read Play track version",
			"check GOOGLE_APPLICATION_CREDENTIALS and Play API access; or pass --force to skip this check",
			err)
	}
	if !store.Eligible || store.VersionCode <= 0 {
		out.Skipped = true
		out.Message = fmt.Sprintf("Play track %q has no eligible release yet; local %s is fine", track, local.Raw)
		return out, nil
	}
	out.StoreVC = store.VersionCode
	out.Store = storeLabel(store)

	if local.Code > store.VersionCode {
		out.Message = fmt.Sprintf("local versionCode %d > Play track %q (%d); ok",
			local.Code, track, store.VersionCode)
		return out, nil
	}

	reason := fmt.Sprintf(
		"local version %s (versionCode %d) is not ahead of Play track %q (latest %s, versionCode %d)",
		local.Raw, local.Code, track, out.Store, store.VersionCode)
	if local.Code == 0 {
		reason = fmt.Sprintf(
			"local version %s has no +build (versionCode); Play track %q already has versionCode %d",
			local.Raw, track, store.VersionCode)
	}

	bumpFn := opts.BumpPast
	if bumpFn == nil {
		bumpFn = bump.BumpPastStore
	}

	if opts.Yes {
		res, berr := bumpFn(root, store.VersionCode, opts.DryRun)
		if berr != nil {
			return out, berr
		}
		out.Bumped = !opts.DryRun
		out.Message = reason + "; auto-bumped: " + res.Message
		return out, nil
	}

	ci := InCI(opts.IsCI)
	tty := isInteractive(opts.IsTTY)
	hint := "bump with `tern bump version patch` (or build), or re-run with --yes to auto-bump past the store version"
	if ci || !tty {
		return out, ternerrors.NewHint(ternerrors.ClassUpload, reason, hint)
	}

	prompt := opts.Prompt
	if prompt == nil {
		prompt = defaultPrompt
	}
	q := fmt.Sprintf(
		"Play track %q already has %s (versionCode %d).\nLocal is %s (versionCode %d).\nBump patch + versionCode past the store and continue? [y/N]: ",
		track, out.Store, store.VersionCode, local.Raw, local.Code)
	ans, perr := prompt(q)
	if perr != nil {
		return out, ternerrors.WrapHint(ternerrors.ClassUpload, "version bump prompt failed", hint, perr)
	}
	ans = strings.ToLower(strings.TrimSpace(ans))
	if ans != "y" && ans != "yes" {
		return out, ternerrors.NewHint(ternerrors.ClassUpload,
			"upload canceled: local versionCode must be greater than Play's",
			hint)
	}
	res, berr := bumpFn(root, store.VersionCode, opts.DryRun)
	if berr != nil {
		return out, berr
	}
	out.Bumped = !opts.DryRun
	out.Message = reason + "; bumped: " + res.Message
	return out, nil
}

func storeLabel(r play.SourceRelease) string {
	if name := strings.TrimSpace(r.Name); name != "" {
		return name
	}
	return fmt.Sprintf("versionCode %d", r.VersionCode)
}
