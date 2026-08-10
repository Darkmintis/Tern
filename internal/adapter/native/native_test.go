package native_test

import (
	"context"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/native"
	"github.com/darkmintis/Tern/internal/config"
)

func TestScaffoldOnly(t *testing.T) {
	ad := native.New(nil)
	if ad.Detect(t.TempDir()) {
		t.Fatal("Detect must stay off in v0")
	}
	if native.Phase != 2 {
		t.Fatalf("phase: %d", native.Phase)
	}
	_, err := ad.Build(context.Background(), adapter.BuildOptions{
		Platform: config.PlatformAndroid,
		Mode:     config.ModeRelease,
	})
	if err == nil {
		t.Fatal("live build should fail on scaffold")
	}
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		Platform: config.PlatformAndroid,
		Mode:     config.ModeRelease,
		DryRun:   true,
	})
	if err != nil || art.Kind == "" {
		t.Fatalf("dry-run: %v %+v", err, art)
	}
}
