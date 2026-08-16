package adapter_test

import (
	"context"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
)

type stubAdapter struct {
	name   string
	detect bool
}

func (s stubAdapter) Name() string            { return s.name }
func (s stubAdapter) Detect(root string) bool { return s.detect }
func (s stubAdapter) Build(ctx context.Context, opts adapter.BuildOptions) (adapter.BuildArtifact, error) {
	return adapter.BuildArtifact{Path: filepath.Join(opts.ProjectRoot, "out"), Platform: opts.Platform, Kind: "aab"}, nil
}

func pkgDir(t *testing.T, name string) string {
	t.Helper()
	return filepath.Join(t.TempDir(), name)
}

func TestRegistryDetectPriorityOrder(t *testing.T) {
	cases := []struct {
		name     string
		adapters []adapter.Adapter
		want     string
		ok       bool
	}{
		{"no adapters", nil, "", false},
		{"none detect", []adapter.Adapter{stubAdapter{name: "a"}, stubAdapter{name: "b"}}, "", false},
		{"first winner", []adapter.Adapter{
			stubAdapter{name: "flutter", detect: false},
			stubAdapter{name: "native", detect: true},
			stubAdapter{name: "kmp", detect: true},
		}, "native", true},
		{"earliest detect wins", []adapter.Adapter{
			stubAdapter{name: "flutter", detect: true},
			stubAdapter{name: "native", detect: true},
		}, "flutter", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			reg := adapter.NewRegistry(tc.adapters...)
			got, ok := reg.Detect(pkgDir(t, "proj"))
			if ok != tc.ok {
				t.Fatalf("ok=%v want %v", ok, tc.ok)
			}
			if tc.ok && got.Name() != tc.want {
				t.Fatalf("detected %q want %q", got.Name(), tc.want)
			}
		})
	}
}

func TestRegistryByName(t *testing.T) {
	reg := adapter.NewRegistry(stubAdapter{name: "flutter"})
	for _, tc := range []struct {
		name string
		ok   bool
	}{
		{"flutter", true},
		{"native", false},
		{"", false},
	} {
		got, ok := reg.ByName(tc.name)
		if ok != tc.ok || (ok && got.Name() != tc.name) {
			t.Fatalf("ByName(%q) = (%v, %v) want ok=%v", tc.name, got, ok, tc.ok)
		}
	}
}

func TestRegistryAllPreservesOrder(t *testing.T) {
	reg := adapter.NewRegistry(
		stubAdapter{name: "flutter"},
		stubAdapter{name: "native"},
		stubAdapter{name: "kmp"},
	)
	all := reg.All()
	if len(all) != 3 {
		t.Fatalf("All() length=%d", len(all))
	}
	for i, want := range []string{"flutter", "native", "kmp"} {
		if all[i].Name() != want {
			t.Fatalf("All()[%d]=%q want %q", i, all[i].Name(), want)
		}
	}
}

func TestBuildOptionsAndArtifactFlow(t *testing.T) {
	ad := stubAdapter{name: "flutter", detect: true}
	dir := pkgDir(t, "proj")
	art, err := ad.Build(context.Background(), adapter.BuildOptions{
		ProjectRoot:  dir,
		Platform:     config.PlatformAndroid,
		Mode:         config.ModeRelease,
		ArtifactKind: "apk",
		Flavor:       "free",
		Scheme:       "Release",
		DryRun:       true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if art.Kind != "aab" {
		t.Fatalf("kind=%q", art.Kind)
	}
	if art.Platform != config.PlatformAndroid {
		t.Fatalf("platform=%q", art.Platform)
	}
	if got, want := art.Path, filepath.Join(dir, "out"); got != want {
		t.Fatalf("path=%q want %q", got, want)
	}
}
