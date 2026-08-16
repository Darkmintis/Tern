package play

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

func artifact(t *testing.T, name, content string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestUploadValidationBranches(t *testing.T) {
	dir := t.TempDir()
	aab := artifact(t, "app.aab", "aab")
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", filepath.Join(dir, "missing.json"))

	cases := []struct {
		name string
		req  UploadRequest
		want string
	}{
		{"empty artifact", UploadRequest{PackageName: "com.x"}, "empty artifact"},
		{"empty package", UploadRequest{ArtifactPath: aab}, "empty package name"},
		{"missing file", UploadRequest{ArtifactPath: filepath.Join(dir, "nope.aab"), PackageName: "com.x"}, "play: artifact"},
		{"directory", UploadRequest{ArtifactPath: dir, PackageName: "com.x"}, "is a directory"},
		{"wrong extension", UploadRequest{ArtifactPath: artifact(t, "app.txt", "x"), PackageName: "com.x"}, "expected .aab or .apk"},
		{"missing creds file", UploadRequest{ArtifactPath: aab, PackageName: "com.x"}, "credentials file missing"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			_, err := (APIClient{}).Upload(context.Background(), tc.req)
			if err == nil {
				t.Fatal("expected error")
			}
			if !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("got %v, want contains %q", err, tc.want)
			}
		})
	}
}

func TestUploadRequiresCredentials(t *testing.T) {
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	_, err := (APIClient{}).Upload(context.Background(), UploadRequest{
		ArtifactPath: artifact(t, "app.aab", "aab"),
		PackageName:  "com.x",
	})
	if err == nil {
		t.Fatal("expected credentials error")
	}
	if hint := ternerrors.HintOf(err); hint == "" {
		t.Fatalf("expected hint, got %v", err)
	}
}

func TestUploadEmptyArtifactBeforeNetwork(t *testing.T) {
	_, err := (APIClient{}).Upload(context.Background(), UploadRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	if class, ok := ternerrors.AsClass(err); !ok || class != ternerrors.ClassUpload {
		t.Fatalf("class=%q", class)
	}
}

func TestClassifyUpload(t *testing.T) {
	if err := classifyUpload("fallback", nil); err != nil {
		t.Fatalf("nil error must return nil, got %v", err)
	}

	cases := []struct {
		name  string
		text  string
		class ternerrors.Class
	}{
		{"permission denied", "googleapi: Error 403: caller does not have permission", ternerrors.ClassUpload},
		{"network", "Post ... dial tcp: connection refused", ternerrors.ClassUpload},
		{"version code", "Version code 42 has already been used", ternerrors.ClassUpload},
		{"unknown fallback", "some random failure", ternerrors.ClassUpload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := classifyUpload("fallback msg", fmt.Errorf("%s", tc.text))
			if err == nil {
				t.Fatal("expected error")
			}
			class, ok := ternerrors.AsClass(err)
			if !ok || class != tc.class {
				t.Fatalf("class=%q", class)
			}
			if tc.name == "unknown fallback" {
				if msg := ternerrors.MessageOf(err); msg != "fallback msg" {
					t.Fatalf("fallback message=%q", msg)
				}
			}
		})
	}
}
