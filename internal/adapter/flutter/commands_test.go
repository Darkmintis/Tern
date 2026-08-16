package flutter_test

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// recordingRunner captures every command invocation with an optional fail point.
type recordingRunner struct {
	calls  [][]string
	failOn map[string]error
}

func (r *recordingRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	call := append([]string{name}, args...)
	r.calls = append(r.calls, call)
	if err := r.failOn[name]; err != nil {
		return "", err
	}
	return "ok", nil
}

func lastCall(t *testing.T, r *recordingRunner) []string {
	t.Helper()
	if len(r.calls) == 0 {
		t.Fatal("no calls recorded")
	}
	return r.calls[len(r.calls)-1]
}

func androidProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "build", "app", "outputs", "bundle", "release")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "app-release.aab"), []byte("aab"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildAndroidReleaseInvokesAppbundle(t *testing.T) {
	dir := androidProject(t)
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
	})
	if err != nil {
		t.Fatal(err)
	}
	got := lastCall(t, r)
	want := []string{"flutter", "build", "appbundle", "--release"}
	if strings.Join(got, "\x00") != strings.Join(want, "\x00") {
		t.Fatalf("command %v want %v", got, want)
	}
	if art.Kind != "aab" || filepath.Base(art.Path) != "app-release.aab" {
		t.Fatalf("artifact %+v", art)
	}
}

func apkProject(t *testing.T, name string) string {
	t.Helper()
	dir := t.TempDir()
	out := filepath.Join(dir, "build", "app", "outputs", "flutter-apk")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, name), []byte("apk"), 0o644); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestBuildAndroidApkInvokesApk(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot:  apkProject(t, "app-release.apk"),
		Platform:     config.PlatformAndroid,
		Mode:         config.ModeRelease,
		ArtifactKind: "apk",
	}); err != nil {
		t.Fatal(err)
	}
	got := lastCall(t, r)
	if want := "build apk --release"; !strings.HasSuffix(strings.Join(got, " "), want) {
		t.Fatalf("command %v want suffix %q", got, want)
	}
}

func TestBuildAndroidDebugInvokesApkDebug(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: apkProject(t, "app-debug.apk"),
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeDebug,
	}); err != nil {
		t.Fatal(err)
	}
	got := lastCall(t, r)
	if want := "build apk --debug"; !strings.HasSuffix(strings.Join(got, " "), want) {
		t.Fatalf("command %v want suffix %q", got, want)
	}
}

func TestBuildFlavorAndNoPubAppended(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "build", "app", "outputs", "bundle", "prodRelease")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "app-prod-release.aab"), []byte("aab"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
		Flavor:      "prod",
		SkipPubGet:  true,
	}); err != nil {
		t.Fatal(err)
	}
	got := lastCall(t, r)
	joined := strings.Join(got, " ")
	if !strings.Contains(joined, "--flavor") || !strings.Contains(joined, "prod") || !strings.Contains(joined, "--no-pub") {
		t.Fatalf("command %v missing flavor/--no-pub", got)
	}
}

func TestBuildCleanRunsCleanFirst(t *testing.T) {
	dir := androidProject(t)
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
		Clean:       true,
	}); err != nil {
		t.Fatal(err)
	}
	if got := r.calls[0]; got[0] != "flutter" || got[1] != "clean" {
		t.Fatalf("expected clean first, got %v", got)
	}
}

func TestBuildIOSReleaseFindsIPA(t *testing.T) {
	dir := t.TempDir()
	ipaDir := filepath.Join(dir, "build", "ios", "ipa")
	if err := os.MkdirAll(ipaDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(ipaDir, "Runner.ipa"), []byte("ipa"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformIOS,
		Mode:        config.ModeRelease,
	})
	if err != nil {
		t.Fatal(err)
	}
	if want := "build ipa --release"; !strings.HasSuffix(strings.Join(lastCall(t, r), " "), want) {
		t.Fatalf("command %v want suffix %q", lastCall(t, r), want)
	}
	if art.Kind != "ipa" || filepath.Base(art.Path) != "Runner.ipa" {
		t.Fatalf("artifact %+v", art)
	}
}

func TestBuildIOSDebugNoCodesign(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformIOS,
		Mode:        config.ModeDebug,
	}); err != nil {
		t.Fatal(err)
	}
	got := lastCall(t, r)
	if want := "build ios --debug --no-codesign"; !strings.HasSuffix(strings.Join(got, " "), want) {
		t.Fatalf("command %v want suffix %q", got, want)
	}
}

func TestBuildUnsupportedPlatform(t *testing.T) {
	ad := flutter.New(&recordingRunner{failOn: map[string]error{}})
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.Platform("macos"),
		Mode:        config.ModeRelease,
	})
	if err == nil {
		t.Fatal("expected unsupported platform error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassBuild {
		t.Fatalf("class=%q", class)
	}
}

func TestBuildFlutterMissing(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	ad.LookPath = func(string) (string, error) { return "", os.ErrNotExist }
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
	})
	if err == nil || !strings.Contains(err.Error(), "flutter not found") {
		t.Fatalf("got %v", err)
	}
}

func TestBuildRunnerFailureClassified(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{
		"flutter": ternerrors.WrapStderr(ternerrors.ClassExec, "cmd", "trying to find matching licenses... SDK licenses not accepted", errors.New("boom")),
	}}
	ad := flutter.New(r)
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeDebug,
	})
	if err == nil {
		t.Fatal("expected error")
	}
	if hint := ternerrors.HintOf(err); !strings.Contains(hint, "flutter doctor --android-licenses") {
		t.Fatalf("expected license hint, got: %v", err)
	}
}

func TestBuildCleanFailure(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{
		"flutter": errors.New("clean exploded"),
	}}
	ad := flutter.New(r)
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
		Clean:       true,
	})
	if err == nil {
		t.Fatal("expected clean failure")
	}
}

func TestBuildIOSNoIPAProduced(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformIOS,
		Mode:        config.ModeRelease,
	})
	if err == nil {
		t.Fatal("expected no-ipa error")
	}
	if hint := ternerrors.HintOf(err); hint == "" {
		t.Fatalf("expected hint, got %v", err)
	}
}

func TestBuildAndroidNoArtifactFound(t *testing.T) {
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: t.TempDir(),
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
	})
	if err == nil {
		t.Fatal("expected missing artifact error")
	}
	if hint := ternerrors.HintOf(err); hint == "" {
		t.Fatalf("expected hint, got %v", err)
	}
}

func TestEffectiveFlavorSchemeFallback(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "build", "app", "outputs", "flutter-apk")
	if err := os.MkdirAll(out, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(out, "app-prod-release.apk"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	r := &recordingRunner{failOn: map[string]error{}}
	ad := flutter.New(r)
	// Scheme on Android with no flavor should act as flavor.
	if _, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot:  dir,
		Platform:     config.PlatformAndroid,
		Mode:         config.ModeRelease,
		ArtifactKind: "apk",
		Scheme:       "prod",
	}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(strings.Join(lastCall(t, r), " "), "--flavor prod") {
		t.Fatalf("scheme not mapped to flavor: %v", lastCall(t, r))
	}
}
