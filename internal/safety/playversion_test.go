package safety_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/safety"
	"github.com/darkmintis/Tern/internal/upload/play"
)

func writePubspec(t *testing.T, dir, version string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: app\nversion: "+version+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestEnsurePlayVersionAheadOK(t *testing.T) {
	dir := t.TempDir()
	writePubspec(t, dir, "1.2.4+10")
	tty := true
	ci := false
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		ProjectRoot: dir,
		Target:      "play_store",
		Track:       "internal",
		PackageName: "com.example.app",
		IsCI:        &ci,
		IsTTY:       &tty,
		Lookup: func(context.Context, play.LookupRequest) (play.SourceRelease, error) {
			return play.SourceRelease{Track: "internal", VersionCode: 9, Name: "1.2.3", Eligible: true}, nil
		},
	})
	if err != nil || res.Bumped {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestEnsurePlayVersionAheadPromptsAndBumps(t *testing.T) {
	dir := t.TempDir()
	writePubspec(t, dir, "1.2.3+9")
	tty := true
	ci := false
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		ProjectRoot: dir,
		Target:      "play_store",
		Track:       "internal",
		PackageName: "com.example.app",
		IsCI:        &ci,
		IsTTY:       &tty,
		Prompt:      func(string) (string, error) { return "yes", nil },
		Lookup: func(context.Context, play.LookupRequest) (play.SourceRelease, error) {
			return play.SourceRelease{Track: "internal", VersionCode: 9, Name: "1.2.3", Eligible: true}, nil
		},
	})
	if err != nil || !res.Bumped {
		t.Fatalf("%+v %v", res, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	if !strings.Contains(string(data), "version: 1.2.4+10") {
		t.Fatalf("got %s", data)
	}
}

func TestEnsurePlayVersionAheadCIRequiresYes(t *testing.T) {
	dir := t.TempDir()
	writePubspec(t, dir, "1.0.0+1")
	ci := true
	tty := false
	_, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		ProjectRoot: dir,
		Target:      "play_store",
		Track:       "production",
		PackageName: "com.example.app",
		IsCI:        &ci,
		IsTTY:       &tty,
		Lookup: func(context.Context, play.LookupRequest) (play.SourceRelease, error) {
			return play.SourceRelease{Track: "production", VersionCode: 5, Eligible: true}, nil
		},
	})
	if err == nil || !strings.Contains(err.Error(), "not ahead") {
		t.Fatalf("want refuse, got %v", err)
	}
}

func TestEnsurePlayVersionAheadYesAutoBumps(t *testing.T) {
	dir := t.TempDir()
	writePubspec(t, dir, "2.0.0+2")
	ci := true
	tty := false
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		ProjectRoot: dir,
		Target:      "play_store",
		PackageName: "com.example.app",
		Yes:         true,
		IsCI:        &ci,
		IsTTY:       &tty,
		Lookup: func(context.Context, play.LookupRequest) (play.SourceRelease, error) {
			return play.SourceRelease{VersionCode: 40, Eligible: true}, nil
		},
	})
	if err != nil || !res.Bumped {
		t.Fatalf("%+v %v", res, err)
	}
	data, _ := os.ReadFile(filepath.Join(dir, "pubspec.yaml"))
	if !strings.Contains(string(data), "version: 2.0.1+41") {
		t.Fatalf("got %s", data)
	}
}

func TestEnsurePlayVersionAheadEmptyTrackOK(t *testing.T) {
	dir := t.TempDir()
	writePubspec(t, dir, "1.0.0+1")
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{
		ProjectRoot: dir,
		Target:      "play_store",
		PackageName: "com.example.app",
		Lookup: func(context.Context, play.LookupRequest) (play.SourceRelease, error) {
			return play.SourceRelease{Eligible: false}, nil
		},
	})
	if err != nil || !res.Skipped {
		t.Fatalf("%+v %v", res, err)
	}
}

func TestEnsurePlayVersionAheadSkipsNonPlay(t *testing.T) {
	res, err := safety.EnsurePlayVersionAhead(safety.PlayVersionOpts{Target: "testflight"})
	if err != nil || !res.Skipped {
		t.Fatalf("%+v %v", res, err)
	}
}
