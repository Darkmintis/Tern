package engine_test

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/engine"
	"github.com/darkmintis/Tern/internal/history"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

func initGitRepo(t *testing.T, dir string) {
	t.Helper()
	cmds := [][]string{
		{"git", "init"},
		{"git", "config", "user.email", "tern@test"},
		{"git", "config", "user.name", "tern"},
		{"git", "add", "-A"},
		{"git", "commit", "-m", "init"},
	}
	for _, args := range cmds {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		if out, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("%v: %s (%v)", args, out, err)
		}
	}
}

func TestTagVersionDryRunCreateAndSkip(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not available")
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: app\nversion: 1.2.3+4\n"), 0o644)
	initGitRepo(t, dir)

	eng := engine.New(adapter.NewRegistry(flutter.New(nil)))
	msg, err := eng.TagVersion(context.Background(), dir, "v", true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "dry-run: would git tag v1.2.3") {
		t.Fatalf("%q", msg)
	}

	msg, err = eng.TagVersion(context.Background(), dir, "v", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "created tag v1.2.3") {
		t.Fatalf("%q", msg)
	}

	msg, err = eng.TagVersion(context.Background(), dir, "v", false)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "already exists (skipped)") {
		t.Fatalf("%q", msg)
	}
}

func TestCommitPubspecDryRun(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: app\nversion: 9.0.0+1\n"), 0o644)
	eng := engine.New(adapter.NewRegistry(flutter.New(nil)))
	msg, err := eng.CommitPubspec(context.Background(), dir, "release: $version", false, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "dry-run") || !strings.Contains(msg, "9.0.0") {
		t.Fatalf("%q", msg)
	}
	msg, err = eng.CommitPubspec(context.Background(), dir, "release: $version", true, true)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(msg, "git add -A") {
		t.Fatalf("%q", msg)
	}
}

type fakePlayUpload struct{}

func (fakePlayUpload) Upload(context.Context, play.UploadRequest) (string, error) {
	return "uploaded", nil
}
func (fakePlayUpload) Lookup(context.Context, play.LookupRequest) (play.SourceRelease, error) {
	return play.SourceRelease{}, nil
}
func (fakePlayUpload) Promote(context.Context, play.PromoteRequest) (string, error) {
	return "", nil
}

type fakeASCUpload struct{}

func (fakeASCUpload) Upload(context.Context, asc.UploadRequest) (string, error) { return "", nil }
func (fakeASCUpload) Lookup(context.Context, asc.LookupRequest) (asc.SourceBuild, error) {
	return asc.SourceBuild{}, nil
}
func (fakeASCUpload) Promote(context.Context, asc.PromoteRequest) (string, error) { return "", nil }
func (fakeASCUpload) SetWhatsNew(context.Context, asc.WhatsNewRequest) error      { return nil }

func TestShipRecordsHistoryGitTag(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: app\nversion: 2.1.0+5\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("v:1\n"), 0o644)
	app := filepath.Join(dir, "android", "app")
	_ = os.MkdirAll(app, 0o755)
	_ = os.WriteFile(filepath.Join(app, "build.gradle"), []byte(`android { defaultConfig { applicationId "com.example.app" } }`), 0o644)

	art := filepath.Join(dir, "app-release.aab")
	if err := os.WriteFile(art, []byte("aab"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := artifacts.Save(dir, artifacts.Record{Platform: config.PlatformAndroid, Kind: "aab", Path: art}); err != nil {
		t.Fatal(err)
	}

	eng := engine.New(adapter.NewRegistry(flutter.New(nil)))
	eng.Upload = &upload.Client{Play: fakePlayUpload{}, ASC: fakeASCUpload{}}
	if _, err := eng.TagVersion(context.Background(), dir, "v", true); err != nil {
		t.Fatal(err)
	}
	if err := eng.Ship(context.Background(), engine.ShipOptions{
		ProjectRoot: dir,
		Platform:    config.PlatformAndroid,
		Target:      "play_store",
		Track:       "internal",
		From:        "last",
		Force:       true,
		Yes:         true,
	}); err != nil {
		t.Fatal(err)
	}
	last, err := history.Last(dir)
	if err != nil || last == nil {
		t.Fatalf("history: %v %#v", err, last)
	}
	if last.GitTag != "v2.1.0" || last.Version != "2.1.0" || last.Build != 5 {
		t.Fatalf("%+v", last)
	}
}
