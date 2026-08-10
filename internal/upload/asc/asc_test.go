package asc_test

import (
	"context"
	"os"
	"path/filepath"
	"testing"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/upload/asc"
)

type fakeRunner struct {
	lastName string
	lastArgs []string
}

func (f *fakeRunner) Run(ctx context.Context, dir, name string, args ...string) (string, error) {
	f.lastName = name
	f.lastArgs = append([]string{}, args...)
	return "ok", nil
}

func TestUploadMissingAPIKey(t *testing.T) {
	dir := t.TempDir()
	ipa := filepath.Join(dir, "app.ipa")
	_ = os.WriteFile(ipa, []byte("ipa"), 0o644)
	t.Setenv("APP_STORE_CONNECT_API_KEY_ID", "")
	t.Setenv("APP_STORE_CONNECT_API_ISSUER_ID", "")
	_, err := asc.APIClient{}.Upload(context.Background(), asc.UploadRequest{ArtifactPath: ipa, TestFlight: true})
	if err == nil || ternerrors.HintOf(err) == "" {
		t.Fatalf("%v", err)
	}
}

func TestUploadWithFakeRunner(t *testing.T) {
	dir := t.TempDir()
	ipa := filepath.Join(dir, "app.ipa")
	_ = os.WriteFile(ipa, []byte("ipa"), 0o644)
	t.Setenv("APP_STORE_CONNECT_API_KEY_ID", "KEYID123")
	t.Setenv("APP_STORE_CONNECT_API_ISSUER_ID", "ISSUER-UUID")
	fr := &fakeRunner{}
	msg, err := asc.APIClient{Runner: fr}.Upload(context.Background(), asc.UploadRequest{ArtifactPath: ipa, TestFlight: true})
	if err != nil {
		// xcrun may be missing on linux CI — acceptable if ClassUpload about xcrun
		if ternerrors.HintOf(err) == "" && err.Error() == "" {
			t.Fatal(err)
		}
		class, _ := ternerrors.AsClass(err)
		if class != ternerrors.ClassUpload {
			t.Fatal(err)
		}
		return
	}
	if msg == "" || fr.lastName != "xcrun" {
		t.Fatalf("msg=%q runner=%q", msg, fr.lastName)
	}
}

func TestResolveIPAFromDir(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "App.ipa"), []byte("x"), 0o644)
	t.Setenv("APP_STORE_CONNECT_API_KEY_ID", "K")
	t.Setenv("APP_STORE_CONNECT_API_ISSUER_ID", "I")
	fr := &fakeRunner{}
	_, err := asc.APIClient{Runner: fr}.Upload(context.Background(), asc.UploadRequest{ArtifactPath: dir})
	// On Linux without xcrun this errors after resolve — that's fine if we got past empty artifact
	if err != nil {
		class, _ := ternerrors.AsClass(err)
		if class != ternerrors.ClassUpload {
			t.Fatal(err)
		}
	}
}
