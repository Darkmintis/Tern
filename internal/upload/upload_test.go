package upload_test

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

type fakePlay struct {
	called bool
	upload func(play.UploadRequest)
}

func (f *fakePlay) Upload(ctx context.Context, req play.UploadRequest) (string, error) {
	f.called = true
	if f.upload != nil {
		f.upload(req)
	}
	return "ok-play", nil
}

func (f *fakePlay) Lookup(ctx context.Context, req play.LookupRequest) (play.SourceRelease, error) {
	return play.SourceRelease{Track: req.Track, Eligible: false}, nil
}

func (f *fakePlay) Promote(ctx context.Context, req play.PromoteRequest) (string, error) {
	return "ok-play-promote", nil
}

type fakeASC struct{ called bool }

func (f *fakeASC) Upload(ctx context.Context, req asc.UploadRequest) (string, error) {
	f.called = true
	return "ok-asc", nil
}

func (f *fakeASC) Lookup(ctx context.Context, req asc.LookupRequest) (asc.SourceBuild, error) {
	return asc.SourceBuild{}, nil
}

func (f *fakeASC) Promote(ctx context.Context, req asc.PromoteRequest) (string, error) {
	return "ok-asc-promote", nil
}

func TestClientDryRun(t *testing.T) {
	c := upload.NewClient()
	msg, err := c.Upload(context.Background(), upload.Options{
		Platform: config.PlatformAndroid,
		Target:   "play_store",
		Track:    "internal",
		Artifact: "app.aab",
		DryRun:   true,
		Release:  &releasemeta.Resolved{Name: "1.0.0", Notes: releasemeta.DefaultNotes},
	})
	if err != nil || msg == "" {
		t.Fatalf("%s %v", msg, err)
	}
}

func TestClientDispatchesPlay(t *testing.T) {
	fp := &fakePlay{}
	c := &upload.Client{Play: fp, ASC: &fakeASC{}}
	msg, err := c.Upload(context.Background(), upload.Options{
		Platform:    config.PlatformAndroid,
		Target:      "play_store",
		Artifact:    "app.aab",
		PackageName: "com.example.app",
		Release:     &releasemeta.Resolved{Name: "1.0.0", Notes: "Bug fixes and improvements."},
	})
	if err != nil || !fp.called || msg != "ok-play" {
		t.Fatalf("%s %v called=%v", msg, err, fp.called)
	}
}

func TestClientDispatchesASCSavesMeta(t *testing.T) {
	dir := t.TempDir()
	fa := &fakeASC{}
	c := &upload.Client{Play: &fakePlay{}, ASC: fa}
	msg, err := c.Upload(context.Background(), upload.Options{
		Platform:    config.PlatformIOS,
		Target:      "testflight",
		Artifact:    "app.ipa",
		ProjectRoot: dir,
		Release:     &releasemeta.Resolved{Name: "1.0.0", Notes: "What's new.", NotesLocale: "en-US"},
	})
	if err != nil || !fa.called {
		t.Fatalf("%s %v called=%v", msg, err, fa.called)
	}
	data, rerr := os.ReadFile(filepath.Join(dir, ".tern", "artifacts", "ios-release-meta.txt"))
	if rerr != nil {
		t.Fatal(rerr)
	}
	if !strings.Contains(string(data), "What's new.") {
		t.Fatalf("meta missing notes: %q", data)
	}
}

func TestClientUnknownTarget(t *testing.T) {
	c := &upload.Client{Play: &fakePlay{}, ASC: &fakeASC{}}
	_, err := c.Upload(context.Background(), upload.Options{
		Platform: config.PlatformAndroid,
		Target:   "slack",
		Artifact: "app.aab",
		Release:  &releasemeta.Resolved{Name: "1.0.0"},
	})
	if err == nil || !strings.Contains(err.Error(), "unknown upload target") {
		t.Fatalf("got %v", err)
	}
}

func TestClientEmptyArtifact(t *testing.T) {
	c := &upload.Client{Play: &fakePlay{}, ASC: &fakeASC{}}
	_, err := c.Upload(context.Background(), upload.Options{
		Platform: config.PlatformAndroid,
		Target:   "play_store",
		Release:  &releasemeta.Resolved{Name: "1.0.0"},
	})
	if err == nil || !strings.Contains(err.Error(), "no artifact to upload") {
		t.Fatalf("got %v", err)
	}
}

func TestClientPlayPackageNameFromProject(t *testing.T) {
	dir := t.TempDir()
	app := filepath.Join(dir, "android", "app")
	if err := os.MkdirAll(app, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(app, "build.gradle"), []byte("android {\n    applicationId \"com.example.auto\"\n}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var gotPkg string
	fp := &fakePlay{}
	fp.upload = func(req play.UploadRequest) {
		gotPkg = req.PackageName
	}
	c := &upload.Client{Play: fp, ASC: &fakeASC{}}
	_, err := c.Upload(context.Background(), upload.Options{
		Platform:    config.PlatformAndroid,
		Target:      "play_store",
		Artifact:    "app.aab",
		ProjectRoot: dir,
		Release:     &releasemeta.Resolved{Name: "1.0.0"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if gotPkg != "com.example.auto" {
		t.Fatalf("package=%q", gotPkg)
	}
}

func TestSpecFromStep(t *testing.T) {
	spec := upload.SpecFromStep(config.Step{
		ReleaseNameStrategy: "version",
		NotesText:           "hello",
		NotesLocale:         "de-DE",
	})
	if spec.NameStrategy != releasemeta.NameVersion {
		t.Fatalf("strategy=%q", spec.NameStrategy)
	}
	if spec.NotesMode != releasemeta.NotesText || spec.NotesText != "hello" {
		t.Fatalf("notes=%+v", spec)
	}
	if spec.NotesLocale != "de-DE" {
		t.Fatalf("locale=%q", spec.NotesLocale)
	}

	custom := upload.SpecFromStep(config.Step{ReleaseNameCustom: "Custom Title"})
	if custom.NameStrategy != releasemeta.NameCustom || custom.NameCustom != "Custom Title" {
		t.Fatalf("custom=%+v", custom)
	}
}
