package play_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/upload/play"
)

func TestUploadValidation(t *testing.T) {
	c := play.APIClient{}
	_, err := c.Upload(context.Background(), play.UploadRequest{})
	if err == nil {
		t.Fatal("expected error")
	}
	class, _ := ternerrors.AsClass(err)
	if class != ternerrors.ClassUpload {
		t.Fatalf("%s", class)
	}

	dir := t.TempDir()
	aab := filepath.Join(dir, "app.aab")
	_ = os.WriteFile(aab, []byte("fake"), 0o644)
	t.Setenv("GOOGLE_APPLICATION_CREDENTIALS", "")
	_, err = c.Upload(context.Background(), play.UploadRequest{
		ArtifactPath: aab,
		PackageName:  "com.example.app",
		Track:        "internal",
	})
	if err == nil || ternerrors.HintOf(err) == "" {
		t.Fatalf("want hint on missing creds: %v", err)
	}
}

func TestRejectDirectoryArtifact(t *testing.T) {
	c := play.APIClient{}
	dir := t.TempDir()
	_, err := c.Upload(context.Background(), play.UploadRequest{
		ArtifactPath: dir,
		PackageName:  "com.example.app",
	})
	if err == nil {
		t.Fatal("expected directory rejection")
	}
}
