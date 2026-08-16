package main

import (
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/version"
)

func TestRewriteLaneShorthand(t *testing.T) {
	root := newRoot()
	cases := []struct {
		name string
		args []string
		want []string
	}{
		{"no args", []string{}, nil},
		{"only program", []string{"tern"}, nil},
		{"flag first", []string{"tern", "-v"}, nil},
		{"known command", []string{"tern", "version"}, nil},
		{"known command flags", []string{"tern", "doctor", "--json"}, nil},
		{"promote known command", []string{"tern", "promote", "internal", "production"}, nil},
		{"lane shorthand", []string{"tern", "release"}, []string{"tern", "run", "release"}},
		{"lane shorthand with flags", []string{"tern", "release", "--dry-run"}, []string{"tern", "run", "release", "--dry-run"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := rewriteLaneShorthand(root, tc.args)
			if (tc.want == nil) != (got == nil) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			if tc.want == nil {
				return
			}
			if len(got) != len(tc.want) {
				t.Fatalf("got %v want %v", got, tc.want)
			}
			for i := range got {
				if got[i] != tc.want[i] {
					t.Fatalf("got %v want %v", got, tc.want)
				}
			}
		})
	}
}

func captureStdout(t *testing.T, fn func()) string {
	t.Helper()
	old := os.Stdout
	r, w, err := os.Pipe()
	if err != nil {
		t.Fatal(err)
	}
	os.Stdout = w
	done := make(chan string, 1)
	go func() {
		data, _ := io.ReadAll(r)
		done <- string(data)
	}()
	fn()
	_ = w.Close()
	out := <-done
	_ = r.Close()
	os.Stdout = old
	return out
}

func TestVersionCommand(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"version"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if want := version.Version + "\n"; out != want {
		t.Fatalf("output=%q want %q", out, want)
	}
}

func TestLanesCommandSorted(t *testing.T) {
	dir := t.TempDir()
	tern := "lane release:\n  build android release\nlane beta:\n  build ios release\n"
	if err := os.WriteFile(filepath.Join(dir, "Ternfile"), []byte(tern), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRoot()
	root.SetArgs([]string{"--dir", dir, "lanes"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if want := "beta\nrelease\n"; out != want {
		t.Fatalf("lanes output=%q want %q", out, want)
	}
}

func TestCacheCommandExplain(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"cache"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "Gradle") || !strings.Contains(out, "CocoaPods") {
		t.Fatalf("cache explain missing content: %q", out)
	}
}

func TestCacheCommandGHAFragment(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"cache", "--github-actions"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if !strings.Contains(out, "actions/cache@v4") || !strings.Contains(out, "tern-pub-") {
		t.Fatalf("fragment missing content: %q", out)
	}
}

func TestRunUnknownLaneErrors(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "Ternfile"), []byte("lane a:\n  build android release\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	root := newRoot()
	root.SetArgs([]string{"--dir", dir, "run", "nope"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected unknown lane error")
	}
	if msg := ternerrors.MessageOf(err); msg != "unknown lane: nope" {
		t.Fatalf("msg=%q", msg)
	}
}

func TestPromoteRequiresTwoArgs(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"promote", "internal"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected promote arg-count error")
	}
	if !strings.Contains(err.Error(), "accepts 2 arg") {
		t.Fatalf("got %v", err)
	}
}

func TestPromoteRejectsMixedPlatforms(t *testing.T) {
	root := newRoot()
	root.SetArgs([]string{"promote", "--dry-run", "internal", "appstore"})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected mixed-platform error")
	}
	if msg := ternerrors.MessageOf(err); !strings.Contains(msg, "iOS stages") {
		t.Fatalf("msg=%q", msg)
	}
}

func TestInitCommand(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\ndependencies:\n  flutter:\n    sdk: flutter\n"), 0o644)
	_ = os.WriteFile(filepath.Join(dir, ".metadata"), []byte("version: 1\n"), 0o644)
	root := newRoot()
	root.SetArgs([]string{"--dir", dir, "init", "--github-actions=false"})
	out := captureStdout(t, func() {
		if err := root.Execute(); err != nil {
			t.Fatal(err)
		}
	})
	if out == "" {
		t.Fatal("init should print a message")
	}
	if _, err := os.Stat(filepath.Join(dir, "Ternfile")); err != nil {
		t.Fatalf("Ternfile not created: %v", err)
	}
}
