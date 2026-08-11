package flutter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/diagnose"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
)

// Adapter builds Flutter apps (v0 supported path).
type Adapter struct {
	Runner execx.Runner
	// LookPath finds binaries on PATH. Defaults to exec.LookPath; override in tests.
	LookPath func(file string) (string, error)
}

func New(r execx.Runner) *Adapter {
	if r == nil {
		r = &execx.RealRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	return &Adapter{Runner: r, LookPath: execx.LookPath}
}

func (a *Adapter) Name() string { return "flutter" }

func (a *Adapter) Detect(projectRoot string) bool {
	_, err := os.Stat(filepath.Join(projectRoot, "pubspec.yaml"))
	if err != nil {
		return false
	}
	if _, err := os.Stat(filepath.Join(projectRoot, ".metadata")); err == nil {
		return true
	}
	data, err := os.ReadFile(filepath.Join(projectRoot, "pubspec.yaml"))
	if err != nil {
		return false
	}
	return containsFlutterSDK(string(data))
}

func containsFlutterSDK(pubspec string) bool {
	return contains(pubspec, "flutter:") || contains(pubspec, "sdk: flutter")
}

func contains(s, sub string) bool {
	return len(sub) == 0 || findSub(s, sub)
}

func findSub(s, sub string) bool {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

func effectiveFlavor(opts adapter.BuildOptions) string {
	if opts.Flavor != "" {
		return opts.Flavor
	}
	return opts.Scheme
}

func (a *Adapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	flavor := effectiveFlavor(opts)
	kind, path := expectedArtifact(opts.ProjectRoot, opts.Platform, opts.Mode, opts.ArtifactKind, flavor)
	if opts.DryRun {
		return adapter.BuildArtifact{Path: path, Platform: opts.Platform, Kind: kind}, nil
	}
	look := a.LookPath
	if look == nil {
		look = execx.LookPath
	}
	if _, err := look("flutter"); err != nil {
		return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
			"flutter not found on PATH",
			"install Flutter and ensure `flutter` is on PATH, then re-run", err)
	}

	if opts.Clean {
		if _, err := a.Runner.Run(ctx, opts.ProjectRoot, "flutter", "clean"); err != nil {
			return adapter.BuildArtifact{}, classifyBuildErr("flutter clean failed",
				"fix the clean error or omit --clean", err)
		}
	}

	var args []string
	switch opts.Platform {
	case config.PlatformAndroid:
		if opts.Mode == config.ModeDebug || kind == "apk" {
			if opts.Mode == config.ModeDebug {
				args = []string{"build", "apk", "--debug"}
			} else {
				args = []string{"build", "apk", "--release"}
			}
		} else {
			args = []string{"build", "appbundle", "--release"}
		}
	case config.PlatformIOS:
		if opts.Mode == config.ModeDebug {
			args = []string{"build", "ios", "--debug", "--no-codesign"}
		} else {
			args = []string{"build", "ipa", "--release"}
		}
	default:
		return adapter.BuildArtifact{}, ternerrors.New(ternerrors.ClassBuild, "unsupported platform")
	}
	if flavor != "" {
		args = append(args, "--flavor", flavor)
	}
	if opts.SkipPubGet {
		args = append(args, "--no-pub")
	}

	if _, err := a.Runner.Run(ctx, opts.ProjectRoot, "flutter", args...); err != nil {
		hint := "run `flutter doctor` and ensure the project builds with `flutter build` manually first"
		if opts.Platform == config.PlatformIOS {
			hint = "iOS release requires macOS, Xcode signing, and a valid team — try `flutter build ipa` manually"
		}
		if flavor != "" {
			hint += fmt.Sprintf("; verify flavor/scheme %q exists in the project", flavor)
		}
		return adapter.BuildArtifact{}, classifyBuildErr(fmt.Sprintf("flutter build %s failed", opts.Platform), hint, err)
	}

	outPath := path
	if opts.Platform == config.PlatformIOS && opts.Mode == config.ModeRelease {
		ipa, err := findIPA(outPath)
		if err != nil {
			return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
				"flutter produced no .ipa",
				"check Xcode signing (team, bundle id, provisioning) then re-run", err)
		}
		outPath = ipa
		kind = "ipa"
	}
	if opts.Platform == config.PlatformAndroid {
		resolved, err := resolveAndroidArtifact(outPath, opts.ProjectRoot, kind, flavor)
		if err != nil {
			return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
				"flutter produced no android artifact",
				"ensure `sign android` ran first so android/key.properties exists, then check build/app/outputs", err)
		}
		outPath = resolved
	}
	return adapter.BuildArtifact{Path: outPath, Platform: opts.Platform, Kind: kind}, nil
}

func classifyBuildErr(fallbackMsg, fallbackHint string, err error) error {
	text := ternerrors.StderrOf(err)
	if text == "" {
		text = err.Error()
	}
	if classified := diagnose.Classify(ternerrors.ClassBuild, fallbackMsg, text, err); classified != nil {
		return classified
	}
	return ternerrors.WrapHint(ternerrors.ClassBuild, fallbackMsg, fallbackHint, err)
}

func expectedArtifact(root string, platform config.Platform, mode config.Mode, artifactKind, flavor string) (kind, path string) {
	switch platform {
	case config.PlatformAndroid:
		wantAPK := mode == config.ModeDebug || artifactKind == "apk"
		if wantAPK {
			name := "app-debug.apk"
			if mode != config.ModeDebug {
				name = "app-release.apk"
				if flavor != "" {
					name = fmt.Sprintf("app-%s-release.apk", flavor)
				}
			} else if flavor != "" {
				name = fmt.Sprintf("app-%s-debug.apk", flavor)
			}
			return "apk", filepath.Join(root, "build", "app", "outputs", "flutter-apk", name)
		}
		if flavor != "" {
			// e.g. prodRelease/app-prod-release.aab
			dir := flavor + "Release"
			name := fmt.Sprintf("app-%s-release.aab", flavor)
			return "aab", filepath.Join(root, "build", "app", "outputs", "bundle", dir, name)
		}
		return "aab", filepath.Join(root, "build", "app", "outputs", "bundle", "release", "app-release.aab")
	case config.PlatformIOS:
		return "ipa", filepath.Join(root, "build", "ios", "ipa")
	default:
		return "", ""
	}
}

func findIPA(dirOrFile string) (string, error) {
	info, err := os.Stat(dirOrFile)
	if err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(dirOrFile), ".ipa") {
		return dirOrFile, nil
	}
	dir := dirOrFile
	var found string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".ipa") {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("no ipa under %s", dir)
	}
	return found, nil
}

func resolveAndroidArtifact(preferred, root, kind, flavor string) (string, error) {
	if _, err := os.Stat(preferred); err == nil {
		return preferred, nil
	}
	// Same directory as preferred.
	dir := filepath.Dir(preferred)
	if found := newestWithExt(dir, filepath.Ext(preferred)); found != "" {
		return found, nil
	}
	// Broad search under build/app/outputs.
	ext := ".aab"
	if kind == "apk" {
		ext = ".apk"
	}
	bases := []string{
		filepath.Join(root, "build", "app", "outputs", "bundle"),
		filepath.Join(root, "build", "app", "outputs", "flutter-apk"),
		filepath.Join(root, "build", "app", "outputs", "apk"),
	}
	var candidates []string
	for _, base := range bases {
		_ = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi == nil || fi.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ext) {
				return nil
			}
			if flavor != "" && !strings.Contains(strings.ToLower(p), strings.ToLower(flavor)) {
				return nil
			}
			candidates = append(candidates, p)
			return nil
		})
	}
	if len(candidates) == 0 && flavor != "" {
		// Retry without flavor filter.
		for _, base := range bases {
			_ = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi == nil || fi.IsDir() {
					return nil
				}
				if strings.EqualFold(filepath.Ext(p), ext) {
					candidates = append(candidates, p)
				}
				return nil
			})
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("artifact not found: %s", preferred)
	}
	newest := candidates[0]
	var newestMod int64
	for _, c := range candidates {
		fi, err := os.Stat(c)
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() >= newestMod {
			newestMod = fi.ModTime().UnixNano()
			newest = c
		}
	}
	return newest, nil
}

func newestWithExt(dir, ext string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestMod int64
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ext) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() >= bestMod {
			bestMod = fi.ModTime().UnixNano()
			best = p
		}
	}
	return best
}
