package artifacts_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/config"
)

func TestSaveLoadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	art := filepath.Join(dir, "app.aab")
	if err := os.WriteFile(art, []byte("fake-aab"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := artifacts.Save(dir, artifacts.Record{
		Platform: config.PlatformAndroid,
		Kind:     "aab",
		Path:     art,
		Version:  "1.0.0+1",
	}); err != nil {
		t.Fatal(err)
	}
	rec, err := artifacts.Load(dir, config.PlatformAndroid)
	if err != nil {
		t.Fatal(err)
	}
	if rec.SHA256 == "" || rec.Path != art {
		t.Fatalf("%+v", rec)
	}
	path, _, err := artifacts.ResolvePath(dir, config.PlatformAndroid, "last")
	if err != nil || path != art {
		t.Fatalf("%s %v", path, err)
	}
}
