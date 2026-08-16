package play

import (
	"context"
	"errors"
	"fmt"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
)

// SourceRelease is a release currently live on a Play track.
type SourceRelease struct {
	Track        string
	VersionCode  int64
	Status       string // completed | inProgress | draft
	Name         string
	UserFraction float64
	// Eligible is true when a completed or rolled-out release exists on the track.
	Eligible bool
}

// LookupRequest reads the newest eligible release on a track.
type LookupRequest struct {
	PackageName string
	Track       string
}

// PromoteRequest carries an existing release to another track without
// uploading any artifact.
type PromoteRequest struct {
	PackageName string
	TargetTrack string
	// Release must come from Lookup (Eligible == true).
	Release SourceRelease
	// UserFraction is a staged rollout in (0,1); 0 or >=1 means completed (100%).
	UserFraction float64
}

// Lookup returns the newest completed / rolled-out release on a track.
// An empty (ineligible) release is not an error: the track may never have
// received a release. Callers decide how to react.
func (c APIClient) Lookup(ctx context.Context, req LookupRequest) (SourceRelease, error) {
	out := SourceRelease{Track: req.Track}
	if req.PackageName == "" {
		return out, ternerrors.New(ternerrors.ClassUpload, "play: empty package name")
	}
	track := strings.TrimSpace(req.Track)
	if track == "" {
		track = "internal"
	}
	svc, editID, err := c.beginEdit(ctx, req.PackageName)
	if err != nil {
		return out, err
	}
	t, err := svc.Edits.Tracks.Get(req.PackageName, editID, track).Context(ctx).Do()
	if err != nil {
		var gerr *googleapi.Error
		if errors.As(err, &gerr) && gerr.Code == 404 {
			return out, nil // never published to this track
		}
		return out, classifyUpload("play: read track "+track, err)
	}
	if rel := newestEligible(t.Releases); rel != nil {
		out.VersionCode = maxVersionCode(rel.VersionCodes)
		out.Status = rel.Status
		out.Name = rel.Name
		out.UserFraction = rel.UserFraction
		out.Eligible = true
	}
	return out, nil
}

// Promote moves an existing release onto the target track in one edit and
// commits it. It never touches a build artifact.
func (c APIClient) Promote(ctx context.Context, req PromoteRequest) (string, error) {
	if req.PackageName == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: empty package name")
	}
	target := strings.TrimSpace(req.TargetTrack)
	if target == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "play: empty target track")
	}
	if !req.Release.Eligible || req.Release.VersionCode <= 0 {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"play: no eligible source release to promote",
			"look up the source track with play.Lookup first; a completed release must exist")
	}

	svc, editID, err := c.beginEdit(ctx, req.PackageName)
	if err != nil {
		return "", err
	}
	update := buildTrackUpdate(req)
	if _, err := svc.Edits.Tracks.Update(req.PackageName, editID, target, update).Context(ctx).Do(); err != nil {
		return "", classifyUpload("play: promote to track "+target, err)
	}
	if _, err := svc.Edits.Commit(req.PackageName, editID).Context(ctx).Do(); err != nil {
		return "", classifyUpload("play: commit promotion edit", err)
	}

	msg := fmt.Sprintf("promoted Play release to track=%s package=%s versionCode=%d",
		target, req.PackageName, req.Release.VersionCode)
	if f := req.UserFraction; f > 0 && f < 1 {
		msg += fmt.Sprintf(" rollout=%.0f%%", f*100)
	}
	if name := strings.TrimSpace(req.Release.Name); name != "" {
		msg += " name=" + name
	}
	return msg, nil
}

// buildTrackUpdate builds the target Track update: it reuses the source
// release's versionCodes and does not upload anything new.
func buildTrackUpdate(req PromoteRequest) *androidpublisher.Track {
	rel := &androidpublisher.TrackRelease{
		Status:       "completed",
		VersionCodes: []int64{req.Release.VersionCode},
	}
	if name := strings.TrimSpace(req.Release.Name); name != "" {
		rel.Name = name
	}
	if f := req.UserFraction; f > 0 && f < 1 {
		rel.Status = "inProgress"
		rel.UserFraction = f
	}
	return &androidpublisher.Track{
		Track:    req.TargetTrack,
		Releases: []*androidpublisher.TrackRelease{rel},
	}
}

// newestEligible returns the highest-versionCode release that is either fully
// rolled out (completed) or staged (inProgress). Drafts are never eligible.
func newestEligible(releases []*androidpublisher.TrackRelease) *androidpublisher.TrackRelease {
	var best *androidpublisher.TrackRelease
	var bestVC int64
	for _, r := range releases {
		if r == nil {
			continue
		}
		switch strings.ToLower(strings.ReplaceAll(r.Status, "_", "")) {
		case "completed", "inprogress":
		default:
			continue
		}
		if vc := maxVersionCode(r.VersionCodes); vc > bestVC {
			bestVC = vc
			best = r
		}
	}
	return best
}

func maxVersionCode(vc []int64) int64 {
	var out int64
	for _, v := range vc {
		if v > out {
			out = v
		}
	}
	return out
}

func (c APIClient) beginEdit(ctx context.Context, packageName string) (*androidpublisher.Service, string, error) {
	svc, err := c.publisherService(ctx)
	if err != nil {
		return nil, "", err
	}
	edit, err := svc.Edits.Insert(packageName, &androidpublisher.AppEdit{}).Context(ctx).Do()
	if err != nil {
		return nil, "", classifyUpload("play: create edit", err)
	}
	return svc, edit.Id, nil
}
