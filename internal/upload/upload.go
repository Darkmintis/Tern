package upload

import (
	"context"
	"fmt"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/upload/asc"
	"github.com/darkmintis/Tern/internal/upload/play"
)

// Options for store upload.
type Options struct {
	Platform    config.Platform
	Target      string // play_store | testflight | app_store
	Track       string
	Artifact    string
	ProjectRoot string
	PackageName string
	DryRun      bool
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

// Upload dispatches to Play or App Store Connect.
func (c *Client) Upload(ctx context.Context, opts Options) (string, error) {
	if opts.DryRun {
		return fmt.Sprintf("dry-run: would upload %s to %s track:%s artifact:%s",
			opts.Platform, opts.Target, opts.Track, opts.Artifact), nil
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
		if pkg == "" && opts.ProjectRoot != "" {
			var err error
			pkg, err = projectmeta.AndroidPackageID(opts.ProjectRoot)
			if err != nil {
				return "", err
			}
		}
		return c.Play.Upload(ctx, play.UploadRequest{
			ArtifactPath: opts.Artifact,
			Track:        track,
			PackageName:  pkg,
		})
	case "testflight", "app_store":
		return c.ASC.Upload(ctx, asc.UploadRequest{
			ArtifactPath: opts.Artifact,
			TestFlight:   opts.Target == "testflight",
		})
	default:
		return "", ternerrors.New(ternerrors.ClassUpload, "unknown upload target: "+opts.Target)
	}
}
