package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/engine"
	"github.com/darkmintis/Tern/internal/output"
)

type slowAdapter struct {
	builds atomic.Int32
}

func (s *slowAdapter) Name() string       { return "flutter" }
func (s *slowAdapter) Detect(string) bool { return true }
func (s *slowAdapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	s.builds.Add(1)
	select {
	case <-ctx.Done():
		return adapter.BuildArtifact{}, ctx.Err()
	case <-time.After(80 * time.Millisecond):
	}
	path := filepath.Join(opts.ProjectRoot, "out-"+string(opts.Platform))
	_ = os.WriteFile(path, []byte(string(opts.Platform)), 0o644)
	kind := "aab"
	if opts.Platform == config.PlatformIOS {
		kind = "ipa"
	}
	return adapter.BuildArtifact{Path: path, Platform: opts.Platform, Kind: kind}, nil
}

func TestParallelBuilds(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte("lane r:\n  build android release\n  build ios release\n"), 0o644)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ad := &slowAdapter{}
	eng := engine.New(adapter.NewRegistry(ad))
	start := time.Now()
	if err := eng.RunLane(context.Background(), cfg, "r", engine.Options{
		ProjectRoot: dir,
		Emitter:     output.New(output.ModeJSON),
	}); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	if ad.builds.Load() != 2 {
		t.Fatalf("builds=%d", ad.builds.Load())
	}
	// Sequential would be ~160ms+; parallel should finish closer to one build.
	if elapsed > 140*time.Millisecond {
		t.Fatalf("expected parallel wall-clock, got %s", elapsed)
	}
}
