package store

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/secrets"
)

// CreateOptions for app creation.
type CreateOptions struct {
	Platform    config.Platform
	ProjectRoot string
	AppName     string
	PackageName string // Android bundle ID or iOS bundle ID
	TeamID      string // Apple Team ID
	Username    string // Apple username (email)
	Track       string // Play track (default: internal)
	DryRun      bool
}

// Result of app creation.
type Result struct {
	Message string
	AppID   string
}

// CreateApp creates a new app on the target store.
func CreateApp(ctx context.Context, opts CreateOptions, em *output.Emitter) (Result, error) {
	if em == nil {
		em = output.New(output.ModeHuman)
	}

	em.Emit(output.Event{Type: "create_app_start", Message: fmt.Sprintf("creating %s app: %s", opts.Platform, opts.AppName)})

	switch opts.Platform {
	case config.PlatformAndroid:
		return createPlayStoreApp(ctx, opts, em)
	case config.PlatformIOS:
		return createAppStoreApp(ctx, opts, em)
	default:
		return Result{}, ternerrors.New(ternerrors.ClassUpload, "unsupported platform for app creation")
	}
}

// createPlayStoreApp creates an app on Google Play Console.
func createPlayStoreApp(ctx context.Context, opts CreateOptions, em *output.Emitter) (Result, error) {
	if opts.DryRun {
		return Result{
			Message: fmt.Sprintf("dry-run: would create Play Store app '%s' (%s)", opts.AppName, opts.PackageName),
			AppID:   opts.PackageName,
		}, nil
	}

	// For Play Store, we need the service account JSON
	saPath := os.Getenv("GOOGLE_PLAY_SERVICE_ACCOUNT_JSON")
	if saPath == "" {
		saPath = filepath.Join(opts.ProjectRoot, "secrets", "google-play-service-account.json")
	}

	if _, err := os.Stat(saPath); os.IsNotExist(err) {
		return Result{}, ternerrors.NewHint(ternerrors.ClassUpload, "Play Console service account not found",
			"set GOOGLE_PLAY_SERVICE_ACCOUNT_JSON env or place service-account.json in secrets/")
	}

	// Use the Tern Play client to verify access
	// App creation on Play Console requires manual setup first
	return Result{
		Message: fmt.Sprintf("Play Store app '%s' ready for upload (app must be created manually on play.google.com/console first)", opts.AppName),
		AppID:   opts.PackageName,
	}, nil
}

// createAppStoreApp creates an app on App Store Connect.
func createAppStoreApp(ctx context.Context, opts CreateOptions, em *output.Emitter) (Result, error) {
	if opts.DryRun {
		return Result{
			Message: fmt.Sprintf("dry-run: would create App Store app '%s' (%s)", opts.AppName, opts.PackageName),
			AppID:   opts.PackageName,
		}, nil
	}

	// Check for required tools
	if err := checkFastlane(); err != nil {
		return Result{}, err
	}

	teamID := opts.TeamID
	if teamID == "" {
		teamID = os.Getenv("APPLE_TEAM_ID")
	}
	if teamID == "" {
		if v, err := secrets.ResolveEnv("IOS_TEAM_ID"); err == nil {
			teamID = v
		}
	}

	username := opts.Username
	if username == "" {
		username = os.Getenv("APPLE_USERNAME")
	}
	if username == "" {
		if v, err := secrets.ResolveEnv("APPLE_USERNAME"); err == nil {
			username = v
		}
	}

	// Use fastlane produce to create the app
	args := []string{
		"produce",
		"--app_name", opts.AppName,
		"--app_identifier", opts.PackageName,
		"--team_id", teamID,
		"--username", username,
		"--skip_binary_upload",
		"--skip_screenshots",
	}

	cmd := exec.CommandContext(ctx, "fastlane", args...)
	cmd.Dir = opts.ProjectRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		output := stdout.String() + stderr.String()
		return Result{}, ternerrors.WrapHint(ternerrors.ClassUpload, "App Store app creation failed",
			"ensure fastlane is installed and Apple credentials are configured", fmt.Errorf("%s: %s", err, output))
	}

	return Result{
		Message: fmt.Sprintf("App Store app '%s' created successfully", opts.AppName),
		AppID:   opts.PackageName,
	}, nil
}

// checkFastlane verifies fastlane is installed.
func checkFastlane() error {
	if _, err := exec.LookPath("fastlane"); err != nil {
		return ternerrors.NewHint(ternerrors.ClassUpload, "fastlane not installed",
			"install fastlane: gem install fastlane")
	}
	return nil
}

// VerifyAppExists checks if an app exists on the target store.
func VerifyAppExists(ctx context.Context, platform config.Platform, packageName string) (bool, error) {
	switch platform {
	case config.PlatformAndroid:
		return verifyPlayStoreApp(ctx, packageName)
	case config.PlatformIOS:
		return verifyAppStoreApp(ctx, packageName)
	default:
		return false, ternerrors.New(ternerrors.ClassUpload, "unsupported platform")
	}
}

// verifyPlayStoreApp checks if an app exists on Play Console.
func verifyPlayStoreApp(ctx context.Context, packageName string) (bool, error) {
	// This would use the Play Developer API to check
	// For now, return true and let the upload fail if not found
	return true, nil
}

// verifyAppStoreApp checks if an app exists on App Store Connect.
func verifyAppStoreApp(ctx context.Context, bundleID string) (bool, error) {
	// This would use the App Store Connect API
	// For now, return true and let the upload fail if not found
	return true, nil
}
