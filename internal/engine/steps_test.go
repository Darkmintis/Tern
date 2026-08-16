package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/engine"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

func writeTernfile(t *testing.T, dir, body string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "Ternfile"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func flutterRoot(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"),
		[]byte("name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644)
	return dir
}

func engineFor(t *testing.T, dir string) (*engine.Engine, *config.Config, error) {
	t.Helper()
	cfg, err := config.Load(dir)
	if err != nil {
		return nil, nil, err
	}
	return engine.New(adapter.NewRegistry(flutter.New(fakeRunner{}))), cfg, nil
}

func TestUnknownLane(t *testing.T) {
	dir := flutterRoot(t)
	writeTernfile(t, dir, "lane a:\n  build android release\n")
	eng, cfg, _ := engineFor(t, dir)
	err := eng.RunLane(context.Background(), cfg, "nope", engine.Options{ProjectRoot: dir, DryRun: true})
	if err == nil {
		t.Fatal("expected unknown lane error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassConfig {
		t.Fatalf("class=%q", class)
	}
}

func TestNoProjectDetect(t *testing.T) {
	dir := t.TempDir()
	writeTernfile(t, dir, "lane a:\n  build android release\n")
	cfg, _ := config.Load(dir)
	eng := engine.New(adapter.NewRegistry(flutter.New(fakeRunner{})))
	err := eng.RunLane(context.Background(), cfg, "a", engine.Options{ProjectRoot: dir, DryRun: true})
	if err == nil {
		t.Fatal("expected detection failure")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassBuild {
		t.Fatalf("class=%q", class)
	}
}

func TestDryRunFullLane(t *testing.T) {
	dir := flutterRoot(t)
	_ = os.WriteFile(filepath.Join(dir, "ks.jks"), []byte("fake"), 0o600)
	t.Setenv("ANDROID_KEYSTORE", filepath.Join(dir, "ks.jks"))
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")
	writeTernfile(t, dir, `
lane release:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal
`)
	eng, cfg, _ := engineFor(t, dir)
	if err := eng.RunLane(context.Background(), cfg, "release", engine.Options{
		ProjectRoot: dir,
		DryRun:      true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestShipStepDryRun(t *testing.T) {
	dir := flutterRoot(t)
	writeTernfile(t, dir, "lane s:\n  ship android from last to play_store track:internal\n")
	eng, cfg, _ := engineFor(t, dir)
	if err := eng.RunLane(context.Background(), cfg, "s", engine.Options{
		ProjectRoot: dir,
		DryRun:      true,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestDryRunMarksStepsStatus(t *testing.T) {
	dir := flutterRoot(t)
	writeTernfile(t, dir, "lane b:\n  build android debug\n")
	eng, cfg, _ := engineFor(t, dir)
	if err := eng.RunLane(context.Background(), cfg, "b", engine.Options{ProjectRoot: dir, DryRun: true}); err != nil {
		t.Fatal(err)
	}
}
