package play

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/diagnose"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/option"
)

// UploadRequest for Google Play.
type UploadRequest struct {
	ArtifactPath string
	Track        string
	PackageName  string
	// ReleaseName is the Play Console release name (optional).
	ReleaseName string
	// ReleaseNotes is localized "what's new" text (optional).
	ReleaseNotes string
	// ReleaseNotesLocale defaults to en-US when notes are set.
	ReleaseNotesLocale string
	// UserFraction is a staged rollout in (0,1); 0 or >=1 means completed (100%).
	UserFraction float64
}

// Client uploads to Play Console API.
type Client interface {
	Upload(ctx context.Context, req UploadRequest) (string, error)
	// Lookup returns the newest eligible release live on a track.
	Lookup(ctx context.Context, req LookupRequest) (SourceRelease, error)
	// Promote points an existing track release at another track without re-uploading.
	Promote(ctx context.Context, req PromoteRequest) (string, error)
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

	svc, err := c.publisherService(ctx)
	if err != nil {
		return "", err
	}

	edit, err := svc.Edits.Insert(req.PackageName, &androidpublisher.AppEdit{}).Context(ctx).Do()
	if err != nil {
		return "", classifyUpload("play: create edit", err)
	}

	f, err := os.Open(req.ArtifactPath)
	if err != nil {
		return "", ternerrors.WrapHint(ternerrors.ClassUpload,
			"cannot open Play artifact",
			"ensure build produced an .aab/.apk and the path is readable", err)
	}
	defer func() {
		_ = f.Close()
	}()

	var versionCode int64
	if ext == ".aab" {
		bundle, uerr := svc.Edits.Bundles.Upload(req.PackageName, edit.Id).
			Context(ctx).Media(f).Do()
		if uerr != nil {
			return "", classifyUpload("play: upload bundle", uerr)
		}
		versionCode = bundle.VersionCode
	} else {
		apk, uerr := svc.Edits.Apks.Upload(req.PackageName, edit.Id).
			Context(ctx).Media(f).Do()
		if uerr != nil {
			return "", classifyUpload("play: upload apk", uerr)
		}
		versionCode = apk.VersionCode
	}

	rel := &androidpublisher.TrackRelease{
		Status:       "completed",
		VersionCodes: []int64{versionCode},
	}
	if frac := req.UserFraction; frac > 0 && frac < 1 {
		rel.Status = "inProgress"
		rel.UserFraction = frac
	}
	if name := strings.TrimSpace(req.ReleaseName); name != "" {
		rel.Name = name
	}
	if notes := strings.TrimSpace(req.ReleaseNotes); notes != "" {
		locale := strings.TrimSpace(req.ReleaseNotesLocale)
		if locale == "" {
			locale = "en-US"
		}
		rel.ReleaseNotes = []*androidpublisher.LocalizedText{{
			Language: locale,
			Text:     notes,
		}}
	}

	trackUpdate := &androidpublisher.Track{
		Track:    track,
		Releases: []*androidpublisher.TrackRelease{rel},
	}
	if _, err := svc.Edits.Tracks.Update(req.PackageName, edit.Id, track, trackUpdate).Context(ctx).Do(); err != nil {
		return "", classifyUpload("play: update track", err)
	}
	if _, err := svc.Edits.Commit(req.PackageName, edit.Id).Context(ctx).Do(); err != nil {
		return "", classifyUpload("play: commit edit", err)
	}

	msg := fmt.Sprintf("uploaded %s to Play track=%s package=%s versionCode=%d",
		filepath.Base(req.ArtifactPath), track, req.PackageName, versionCode)
	if rel.Status == "inProgress" {
		msg += fmt.Sprintf(" rollout=%.0f%%", rel.UserFraction*100)
	}
	if rel.Name != "" {
		msg += " name=" + rel.Name
	}
	if len(rel.ReleaseNotes) > 0 {
		msg += " notes=set"
	}
	return msg, nil
}

// publisherService builds the authenticated Play publisher client.
func (c APIClient) publisherService(ctx context.Context) (*androidpublisher.Service, error) {
	creds := c.CredentialsFile
	if creds == "" {
		creds = os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
	}
	if creds == "" {
		return nil, ternerrors.NewHint(ternerrors.ClassUpload,
			"play: set GOOGLE_APPLICATION_CREDENTIALS to a Play Console service-account JSON key",
			"Play Console → Setup → API access → service account → download JSON → export the path")
	}
	if _, err := os.Stat(creds); err != nil {
		return nil, ternerrors.WrapHint(ternerrors.ClassUpload,
			"Play credentials file missing",
			"export GOOGLE_APPLICATION_CREDENTIALS to an existing service-account JSON path", err)
	}
	svc, err := androidpublisher.NewService(ctx, option.WithAuthCredentialsFile(option.ServiceAccount, creds))
	if err != nil {
		return nil, classifyUpload("play: creating publisher client", err)
	}
	return svc, nil
}

func classifyUpload(fallback string, err error) error {
	if err == nil {
		return nil
	}
	text := err.Error()
	if classified := diagnose.Classify(ternerrors.ClassUpload, fallback, text, err); classified != nil {
		return classified
	}
	return ternerrors.WrapHint(ternerrors.ClassUpload, fallback,
		"check Play Console access, package id, and credentials — see docs/TROUBLESHOOTING.md", err)
}
