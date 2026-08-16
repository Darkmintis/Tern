package engine_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/engine"
	"github.com/darkmintis/Tern/internal/output"
)

type benchAdapter struct {
	delay time.Duration
}

func (benchAdapter) Name() string       { return "flutter" }
func (benchAdapter) Detect(string) bool { return true }

func (b benchAdapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	select {
	case <-ctx.Done():
		return adapter.BuildArtifact{}, ctx.Err()
	case <-time.After(b.delay):
	}
	path := filepath.Join(opts.ProjectRoot, "out-"+string(opts.Platform)+"-"+string(opts.Mode))
	_ = os.WriteFile(path, []byte("bin"), 0o644)
	return adapter.BuildArtifact{Path: path, Platform: opts.Platform, Kind: "aab"}, nil
}

func benchLane(b *testing.B, ternfile string) {
	b.Helper()
	dir := b.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"),
		[]byte("name: demo\nversion: 1.0.0+1\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("v:1\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, "Ternfile"), []byte(ternfile), 0o644)
	cfg, err := config.Load(dir)
	if err != nil {
		b.Fatal(err)
	}
	ad := benchAdapter{delay: 50 * time.Millisecond}
	eng := engine.New(adapter.NewRegistry(ad))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		if err := eng.RunLane(context.Background(), cfg, "r", engine.Options{
			ProjectRoot: dir,
			Emitter:     output.New(output.ModeJSON),
		}); err != nil {
			b.Fatal(err)
		}
	}
}

// BenchmarkLane_2ParallelBuilds schedules android + ios concurrently.
// Both builds cost delay; wall-clock should approach config.BuildDelay, not 2x.
func BenchmarkLane_2ParallelBuilds(b *testing.B) {
	benchLane(b, "lane r:\n  build android release\n  build ios release\n")
}

// BenchmarkLane_2SequentialBuilds uses two same-platform builds, forcing
// sequential execution (the parallel-group guard rejects duplicate platforms).
func BenchmarkLane_2SequentialBuilds(b *testing.B) {
	benchLane(b, "lane r:\n  build android release\n  build android debug\n")
}
