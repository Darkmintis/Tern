package upload

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

// Options for store upload.
type Options struct {
	Platform    config.Platform
	Target      string // play_store | testflight | app_store
	Track       string
	// Rollout is Play staged rollout fraction in (0,1); 0 means full/completed.
	Rollout     float64
	Artifact    string
	ProjectRoot string
	PackageName string
	DryRun      bool
	// Release meta (resolved before Upload when Spec is set, or pass Resolved).
	ReleaseSpec releasemeta.Spec
	Release     *releasemeta.Resolved
}

// Client uploads artifacts to stores.
type Client struct {
	Play play.Client
	ASC  asc.Client
}

func NewClient() *Client {
	return &Client{
		Play: play.APIClient{},
		ASC:  asc.APIClient{},
	}
}

// SpecFromStep maps Ternfile step fields into a release Spec.
func SpecFromStep(step config.Step) releasemeta.Spec {
	s := releasemeta.DefaultSpec()
	if step.ReleaseNameStrategy != "" {
		s.NameStrategy = releasemeta.NameStrategy(step.ReleaseNameStrategy)
	}
	if step.ReleaseNameCustom != "" {
		s.NameStrategy = releasemeta.NameCustom
		s.NameCustom = step.ReleaseNameCustom
	}
	if step.NotesMode != "" {
		s.NotesMode = releasemeta.NotesMode(step.NotesMode)
	}
	if step.NotesText != "" {
		s.NotesMode = releasemeta.NotesText
		s.NotesText = step.NotesText
	}
	if step.NotesFile != "" {
		s.NotesMode = releasemeta.NotesFile
		s.NotesFile = step.NotesFile
	}
	if step.NotesLocale != "" {
		s.NotesLocale = step.NotesLocale
	}
	return s
}

// Upload dispatches to Play or App Store Connect.
func (c *Client) Upload(ctx context.Context, opts Options) (string, error) {
	root := opts.ProjectRoot
	rel := opts.Release
	if rel == nil {
		resolved, err := releasemeta.Resolve(root, opts.ReleaseSpec)
		if err != nil {
			return "", err
		}
		rel = &resolved
	}

	if opts.DryRun {
		roll := ""
		if opts.Rollout > 0 && opts.Rollout < 1 {
			roll = fmt.Sprintf(" rollout:%.0f%%", opts.Rollout*100)
		}
		return fmt.Sprintf("dry-run: would upload %s to %s track:%s%s artifact:%s name=%q notes=%q",
			opts.Platform, opts.Target, opts.Track, roll, opts.Artifact, rel.Name, truncate(rel.Notes, 60)), nil
	}
	if opts.Artifact == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "no artifact to upload — run a build step first")
	}
	switch opts.Target {
	case "play_store":
		track := opts.Track
		if track == "" {
			track = "internal"
		}
		pkg := opts.PackageName
		if pkg == "" && root != "" {
			var err error
			pkg, err = projectmeta.AndroidPackageID(root)
			if err != nil {
				return "", err
			}
		}
		return c.Play.Upload(ctx, play.UploadRequest{
			ArtifactPath:       opts.Artifact,
			Track:              track,
			PackageName:        pkg,
			ReleaseName:        rel.Name,
			ReleaseNotes:       rel.Notes,
			ReleaseNotesLocale: rel.NotesLocale,
			UserFraction:       opts.Rollout,
		})
	case "testflight", "app_store":
		msg, err := c.ASC.Upload(ctx, asc.UploadRequest{
			ArtifactPath: opts.Artifact,
			TestFlight:   opts.Target == "testflight",
			WhatsNew:     rel.Notes,
			ReleaseName:  rel.Name,
		})
		if err != nil {
			return "", err
		}
		// Persist resolved What's New for operators / future ASC localization API.
		if root != "" && (rel.Notes != "" || rel.Name != "") {
			_ = writeIOSReleaseMeta(root, *rel)
		}
		if rel.Notes != "" {
			msg += " (What's New saved under .tern/artifacts/ios-release-meta.json; set in App Store Connect if needed)"
		}
		return msg, nil
	default:
		return "", ternerrors.New(ternerrors.ClassUpload, "unknown upload target: "+opts.Target)
	}
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func writeIOSReleaseMeta(root string, rel releasemeta.Resolved) error {
	dir := filepath.Join(root, ".tern", "artifacts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	body := fmt.Sprintf("name=%s\nlocale=%s\nnotes:\n%s\n", rel.Name, rel.NotesLocale, rel.Notes)
	return os.WriteFile(filepath.Join(dir, "ios-release-meta.txt"), []byte(body), 0o644)
}
