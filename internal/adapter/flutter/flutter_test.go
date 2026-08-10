package flutter_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/config"
)

type fakeRunner struct{}

func (fakeRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return "ok", nil
}

func TestDetect(t *testing.T) {
	dir := t.TempDir()
	ad := flutter.New(nil)
	if ad.Detect(dir) {
		t.Fatal("empty dir should not detect")
	}
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: x\n"), 0o644)
	if ad.Detect(dir) {
		t.Fatal("pubspec without flutter should not detect")
	}
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	if !ad.Detect(dir) {
		t.Fatal("expected detect")
	}
}

func TestBuildDryRun(t *testing.T) {
	ad := flutter.New(fakeRunner{})
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: "/tmp/app",
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
		DryRun:      true,
	})
	if err != nil || art.Kind != "aab" {
		t.Fatalf("%+v %v", art, err)
	}
}

func TestBuildAndroidRequiresArtifact(t *testing.T) {
	dir := t.TempDir()
	ad := flutter.New(fakeRunner{})
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
	})
	if err == nil {
		t.Fatal("expected missing artifact error")
	}
}

func TestBuildAndroidFindsAAB(t *testing.T) {
	dir := t.TempDir()
	out := filepath.Join(dir, "build", "app", "outputs", "bundle", "release")
	_ = os.MkdirAll(out, 0o755)
	_ = os.WriteFile(filepath.Join(out, "app-release.aab"), []byte("aab"), 0o644)
	ad := flutter.New(fakeRunner{})
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Mode:        config.ModeRelease,
	})
	if err != nil || art.Kind != "aab" {
		t.Fatalf("%+v %v", art, err)
	}
}
