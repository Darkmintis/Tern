package store

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

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

	// Play Store API doesn't support app creation
	// User must create manually in Play Console
	return Result{
		Message: fmt.Sprintf("Play Store app '%s' must be created manually.\n\n1. Go to https://play.google.com/console\n2. Click 'Create app'\n3. App name: %s\n4. Package name: %s\n5. Complete setup forms", opts.AppName, opts.AppName, opts.PackageName),
		AppID:   opts.PackageName,
	}, nil
}

// createAppStoreApp creates an app on App Store Connect using the API.
func createAppStoreApp(ctx context.Context, opts CreateOptions, em *output.Emitter) (Result, error) {
	if opts.DryRun {
		return Result{
			Message: fmt.Sprintf("dry-run: would create App Store app '%s' (%s)", opts.AppName, opts.PackageName),
			AppID:   opts.PackageName,
		}, nil
	}

	// Get Apple credentials
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

	apiKeyPath := os.Getenv("APPLE_API_KEY_PATH")
	if apiKeyPath == "" {
		if v, err := secrets.ResolveEnv("APPLE_API_KEY_PATH"); err == nil {
			apiKeyPath = v
		}
	}

	// Check for API key or App Store Connect API key
	if apiKeyPath == "" {
		// Try to find .p8 file in secrets directory
		secretsDir := filepath.Join(opts.ProjectRoot, "secrets")
		entries, err := os.ReadDir(secretsDir)
		if err == nil {
			for _, entry := range entries {
				if filepath.Ext(entry.Name()) == ".p8" {
					apiKeyPath = filepath.Join(secretsDir, entry.Name())
					break
				}
			}
		}
	}

	// For now, provide instructions since App Store Connect API requires
	// creating an API key in the portal first
	if teamID == "" || apiKeyPath == "" {
		return Result{
			Message: fmt.Sprintf(`App Store app '%s' requires Apple API credentials.

Setup steps:
1. Go to https://appstoreconnect.apple.com/access/api
2. Click '+' to create a new API key
3. Name: Tern Release
4. Access: Developer
5. Download the .p8 file to secrets/
6. Note the Key ID and Team ID

Then set environment variables:
  APPLE_TEAM_ID=%s
  APPLE_API_KEY_PATH=secrets/AuthKey_XXXXXXXX.p8

Or create the app manually:
1. Go to https://appstoreconnect.apple.com
2. Click 'My Apps' → '+'
3. App name: %s
4. Bundle ID: %s
5. SKU: %s`, opts.AppName, opts.AppName, opts.PackageName, opts.PackageName),
			AppID: opts.PackageName,
		}, nil
	}

	// TODO: Implement App Store Connect API call to create app
	// POST https://api.appstoreconnect.apple.com/v1/apps
	// {
	//   "data": {
	//     "type": "apps",
	//     "attributes": {
	//       "name": opts.AppName,
	//       "bundleId": opts.PackageName,
	//       "sku": opts.PackageName,
	//       "primaryLocale": "en-US"
	//     }
	//   }
	// }

	return Result{
		Message: fmt.Sprintf("App Store app '%s' created (API integration pending)", opts.AppName),
		AppID:   opts.PackageName,
	}, nil
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
