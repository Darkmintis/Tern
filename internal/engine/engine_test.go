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
	"github.com/darkmintis/Tern/internal/output"
)

type fakeRunner struct{}

func (fakeRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	return "ok", nil
}

func TestRunLane_DryRunNoOp(t *testing.T) {
	dir := t.TempDir()
	pubspec := []byte("name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n")
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), pubspec, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	tern := `
lane release:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal
`
	if err := os.WriteFile(filepath.Join(dir, "Ternfile"), []byte(tern), 0o644); err != nil {
		t.Fatal(err)
	}
	t.Setenv("ANDROID_KEYSTORE", filepath.Join(dir, "ks.jks"))
	t.Setenv("ANDROID_KEYSTORE_PASSWORD", "store-pass-strong")
	t.Setenv("ANDROID_KEY_ALIAS", "upload")
	t.Setenv("ANDROID_KEY_PASSWORD", "key-pass-strong")
	if err := os.WriteFile(filepath.Join(dir, "ks.jks"), []byte("fake"), 0o644); err != nil {
		t.Fatal(err)
	}

	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	reg := adapter.NewRegistry(flutter.New(fakeRunner{}))
	eng := engine.New(reg)
	em := output.New(output.ModeJSON)
	if err := eng.RunLane(context.Background(), cfg, "release", engine.Options{
		ProjectRoot: dir,
		DryRun:      true,
		Emitter:     em,
	}); err != nil {
		t.Fatal(err)
	}
}

func TestSyncCertsLiveRejected(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("v:1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte("lane r:\n  sync_certs pull repo:env:CERT_REPO\n"), 0o644)
	t.Setenv("CERT_REPO", "git@example.com:org/certs.git")
	cfg, _ := config.Load(dir)
	eng := engine.New(adapter.NewRegistry(flutter.New(fakeRunner{})))
	err := eng.RunLane(context.Background(), cfg, "r", engine.Options{ProjectRoot: dir})
	if err == nil || ternerrors.HintOf(err) == "" {
		t.Fatalf("%v", err)
	}
}

func TestDetectFlutter(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("dependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	ad := flutter.New(nil)
	if !ad.Detect(dir) {
		t.Fatal("expected flutter detect")
	}
}
