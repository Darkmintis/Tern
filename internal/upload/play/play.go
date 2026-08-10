package play

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

// UploadRequest for Google Play.
type UploadRequest struct {
	ArtifactPath string
	Track        string
	PackageName  string
}

// Client uploads to Play Console API.
type Client interface {
	Upload(ctx context.Context, req UploadRequest) (string, error)
}

// APIClient uploads AABs/APKs via google.golang.org/api/androidpublisher/v3.
type APIClient struct {
	// CredentialsFile overrides GOOGLE_APPLICATION_CREDENTIALS when set.
	CredentialsFile string
}

func (c APIClient) Upload(ctx context.Context, req UploadRequest) (string, error) {
	if req.ArtifactPath == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: empty artifact")
	}
	if req.PackageName == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: empty package name")
	}
	info, err := os.Stat(req.ArtifactPath)
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: artifact", err)
	}
	if info.IsDir() {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: artifact is a directory, expected .aab/.apk file")
	}
	ext := strings.ToLower(filepath.Ext(req.ArtifactPath))
	if ext != ".aab" && ext != ".apk" {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: expected .aab or .apk, got "+req.ArtifactPath)
	}

	track := req.Track
	if track == "" {
		track = "internal"
	}

	creds := c.CredentialsFile
	if creds == "" {
		creds = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if creds == "" {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"play: set GOOGLE_APPLICATION_CREDENTIALS to a Play Console service-account JSON key",
			"Play Console → Setup → API access → service account → download JSON → export the path")
	}
	if _, err := os.Stat(creds); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: credentials file", err)
	}

	svc, err := androidpublisher.NewService(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, creds))
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: creating publisher client", err)
	}

	edit, err := svc.Edits.Insert(req.PackageName, &androidpublisher.AppEdit{}).Context(ctx).Do()
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: create edit", err)
	}

	f, err := os.Open(req.ArtifactPath)
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: open artifact", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var versionCode int64
	if ext == ".aab" {
		bundle, uerr := svc.Edits.Bundles.Upload(req.PackageName, edit.Id).
			Context(ctx).Media(f).Do()
		if uerr != nil {
			return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: upload bundle", uerr)
		}
		versionCode = bundle.VersionCode
	} else {
		apk, uerr := svc.Edits.Apks.Upload(req.PackageName, edit.Id).
			Context(ctx).Media(f).Do()
		if uerr != nil {
			return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: upload apk", uerr)
		}
		versionCode = apk.VersionCode
	}

	trackUpdate := &androidpublisher.Track{
		Track: track,
		Releases: []*androidpublisher.TrackRelease{{
			Status:       "completed",
			VersionCodes: []int64{versionCode},
		}},
	}
	if _, err := svc.Edits.Tracks.Update(req.PackageName, edit.Id, track, trackUpdate).Context(ctx).Do(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: update track", err)
	}
	if _, err := svc.Edits.Commit(req.PackageName, edit.Id).Context(ctx).Do(); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "play: commit edit", err)
	}

	return fmt.Sprintf("uploaded %s to Play track=%s package=%s versionCode=%d",
		filepath.Base(req.ArtifactPath), track, req.PackageName, versionCode), nil
}
