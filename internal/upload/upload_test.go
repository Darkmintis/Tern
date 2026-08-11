package upload_test

import (
	"context"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

type fakePlay struct{ called bool }

func (f *fakePlay) Upload(ctx context.Context, req play.UploadRequest) (string, error) {
	f.called = true
	return "ok-play", nil
}

type fakeASC struct{ called bool }

func (f *fakeASC) Upload(ctx context.Context, req asc.UploadRequest) (string, error) {
	f.called = true
	return "ok-asc", nil
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
