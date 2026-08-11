package flutter

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
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

func (a *Adapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	kind, path := expectedArtifact(opts.ProjectRoot, opts.Platform, opts.Mode, opts.ArtifactKind)
	if opts.DryRun {
		return adapter.BuildArtifact{Path: path, Platform: opts.Platform, Kind: kind}, nil
	}
	look := a.LookPath
	if look == nil {
		look = execx.LookPath
	}
	if _, err := look("flutter"); err != nil {
		return adapter.BuildArtifact{}, ternerrors.Wrap(ternerrors.ClassBuild, "flutter not found on PATH", err)
	}

	if opts.Clean {
		if _, err := a.Runner.Run(ctx, opts.ProjectRoot, "flutter", "clean"); err != nil {
			return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
				"flutter clean failed", "fix the clean error or omit --clean", err)
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
	if opts.SkipPubGet {
		args = append(args, "--no-pub")
	}

	if _, err := a.Runner.Run(ctx, opts.ProjectRoot, "flutter", args...); err != nil {
		hint := "run `flutter doctor` and ensure the project builds with `flutter build` manually first"
		if opts.Platform == config.PlatformIOS {
			hint = "iOS release requires macOS, Xcode signing, and a valid team — try `flutter build ipa` manually"
		}
		return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
			fmt.Sprintf("flutter build %s", opts.Platform), hint, err)
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
		resolved, err := resolveAndroidArtifact(outPath)
		if err != nil {
			return adapter.BuildArtifact{}, ternerrors.WrapHint(ternerrors.ClassBuild,
				"flutter produced no android artifact",
				"ensure `sign android` ran first so android/key.properties exists, then check build/app/outputs", err)
		}
		outPath = resolved
	}
	return adapter.BuildArtifact{Path: outPath, Platform: opts.Platform, Kind: kind}, nil
}

func expectedArtifact(root string, platform config.Platform, mode config.Mode, artifactKind string) (kind, path string) {
	switch platform {
	case config.PlatformAndroid:
		wantAPK := mode == config.ModeDebug || artifactKind == "apk"
		if wantAPK {
			if mode == config.ModeDebug {
				return "apk", filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-debug.apk")
			}
			return "apk", filepath.Join(root, "build", "app", "outputs", "flutter-apk", "app-release.apk")
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

func resolveAndroidArtifact(path string) (string, error) {
	if _, err := os.Stat(path); err == nil {
		return path, nil
	}
	dir := filepath.Dir(path)
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", err
	}
	ext := filepath.Ext(path)
	for _, e := range entries {
		if !e.IsDir() && strings.EqualFold(filepath.Ext(e.Name()), ext) {
			return filepath.Join(dir, e.Name()), nil
		}
	}
	return "", fmt.Errorf("artifact not found: %s", path)
}
