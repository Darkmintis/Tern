package history

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkmintis/Tern/internal/config"
)

func TestAppendAndLoad(t *testing.T) {
	dir := t.TempDir()
	rec := Record{
		Version:      "1.2.3",
		Build:        4,
		Platform:     config.PlatformAndroid,
		Target:       "play_store",
		Track:        "internal",
		ArtifactPath: "/path/to/app.aab",
		ReleasedAt:   time.Date(2026, 8, 26, 10, 0, 0, 0, time.UTC),
	}
	if err := Append(dir, rec); err != nil {
		t.Fatal(err)
	}
	h, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Releases) != 1 {
		t.Fatalf("expected 1 release, got %d", len(h.Releases))
	}
	if h.Releases[0].Version != "1.2.3" {
		t.Fatalf("expected version 1.2.3, got %s", h.Releases[0].Version)
	}
}

func TestLast(t *testing.T) {
	dir := t.TempDir()
	_ = Append(dir, Record{Version: "1.0.0", Build: 1, Platform: config.PlatformAndroid, Track: "internal"})
	_ = Append(dir, Record{Version: "1.1.0", Build: 2, Platform: config.PlatformAndroid, Track: "internal"})
	last, err := Last(dir)
	if err != nil {
		t.Fatal(err)
	}
	if last == nil {
		t.Fatal("expected last release")
	}
	if last.Version != "1.1.0" {
		t.Fatalf("expected version 1.1.0, got %s", last.Version)
	}
}

func TestLastForTrack(t *testing.T) {
	dir := t.TempDir()
	_ = Append(dir, Record{Version: "1.0.0", Build: 1, Platform: config.PlatformAndroid, Track: "internal"})
	_ = Append(dir, Record{Version: "1.1.0", Build: 2, Platform: config.PlatformAndroid, Track: "production"})
	_ = Append(dir, Record{Version: "1.2.0", Build: 3, Platform: config.PlatformAndroid, Track: "internal"})
	last, err := LastForTrack(dir, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if last == nil {
		t.Fatal("expected last release for internal")
	}
	if last.Version != "1.2.0" {
		t.Fatalf("expected version 1.2.0, got %s", last.Version)
	}
}

func TestLastForTrackEmpty(t *testing.T) {
	dir := t.TempDir()
	last, err := LastForTrack(dir, "internal")
	if err != nil {
		t.Fatal(err)
	}
	if last != nil {
		t.Fatal("expected nil for empty history")
	}
}

func TestLoadNonExistent(t *testing.T) {
	dir := t.TempDir()
	h, err := Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	if len(h.Releases) != 0 {
		t.Fatalf("expected empty history, got %d releases", len(h.Releases))
	}
}

func TestSaveCreatesDirectory(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "dir")
	rec := Record{Version: "1.0.0", Build: 1, Platform: config.PlatformAndroid, Track: "internal", ReleasedAt: time.Now()}
	if err := Append(dir, rec); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, DirName, FileName)); err != nil {
		t.Fatalf("history file not created: %v", err)
	}
}
