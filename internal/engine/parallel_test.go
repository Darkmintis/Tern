package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/engine"
	"github.com/darkmintis/Tern/internal/output"
)

type timeWindow struct{ start, end time.Time }

type slowAdapter struct {
	mu     sync.Mutex
	builds []timeWindow
}

func (s *slowAdapter) Name() string       { return "flutter" }
func (s *slowAdapter) Detect(string) bool { return true }
func (s *slowAdapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	start := time.Now()
	select {
	case <-ctx.Done():
		return adapter.BuildArtifact{}, ctx.Err()
	case <-time.After(80 * time.Millisecond):
	}
	s.mu.Lock()
	s.builds = append(s.builds, timeWindow{start: start, end: time.Now()})
	s.mu.Unlock()
	path := filepath.Join(opts.ProjectRoot, "out-"+string(opts.Platform))
	_ = os.WriteFile(path, []byte(string(opts.Platform)), 0o644)
	kind := "aab"
	if opts.Platform == config.PlatformIOS {
		kind = "ipa"
	}
	return adapter.BuildArtifact{Path: path, Platform: opts.Platform, Kind: kind}, nil
}

func boolPtr(v bool) *bool { return &v }

func setupProject(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte("lane r:\n  build android release\n  build ios release\n"), 0o644)
	return dir
}

// TestParallelBuilds verifies builds overlap when --parallel is set.
func TestParallelBuilds(t *testing.T) {
	dir := setupProject(t)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ad := &slowAdapter{}
	eng := engine.New(adapter.NewRegistry(ad))
	if err := eng.RunLane(context.Background(), cfg, "r", engine.Options{
		ProjectRoot: dir,
		Parallel:    boolPtr(true),
		Emitter:     output.New(output.ModeJSON),
	}); err != nil {
		t.Fatal(err)
	}
	if len(ad.builds) != 2 {
		t.Fatalf("builds=%d", len(ad.builds))
	}
	if !overlap(ad.builds[0], ad.builds[1]) {
		t.Fatalf("expected parallel builds to overlap, got %v and %v", ad.builds[0], ad.builds[1])
	}
}

// TestSequentialBuilds verifies builds run one after another by default.
func TestSequentialBuilds(t *testing.T) {
	dir := setupProject(t)
	cfg, err := config.Load(dir)
	if err != nil {
		t.Fatal(err)
	}
	ad := &slowAdapter{}
	eng := engine.New(adapter.NewRegistry(ad))
	if err := eng.RunLane(context.Background(), cfg, "r", engine.Options{
		ProjectRoot: dir,
		Parallel:    nil, // default: sequential
		Emitter:     output.New(output.ModeJSON),
	}); err != nil {
		t.Fatal(err)
	}
	if len(ad.builds) != 2 {
		t.Fatalf("builds=%d", len(ad.builds))
	}
	if overlap(ad.builds[0], ad.builds[1]) {
		t.Fatalf("expected sequential builds, but they overlapped: %v and %v", ad.builds[0], ad.builds[1])
	}
}

func overlap(a, b timeWindow) bool {
	start := a.start
	if b.start.After(start) {
		start = b.start
	}
	end := a.end
	if b.end.Before(end) {
		end = b.end
	}
	return start.Before(end)
}
