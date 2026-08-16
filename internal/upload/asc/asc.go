package asc

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/diagnose"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
)

// Env vars for App Store Connect API key upload via xcrun altool.
const (
	EnvAPIKeyID    = "APP_STORE_CONNECT_API_KEY_ID"
	EnvAPIIssuerID = "APP_STORE_CONNECT_API_ISSUER_ID"
	EnvAPIKeyPath  = "APP_STORE_CONNECT_API_KEY_PATH" // path to AuthKey_XXX.p8
)

// UploadRequest for App Store Connect / TestFlight.
type UploadRequest struct {
	ArtifactPath string
	TestFlight   bool
	// WhatsNew / ReleaseName are resolved by Tern; altool uploads the IPA only.
	// Notes are persisted by the upload package for operators until ASC localization API lands.
	WhatsNew    string
	ReleaseName string
}

// Client uploads to ASC.
type Client interface {
	Upload(ctx context.Context, req UploadRequest) (string, error)
	// Lookup returns the newest succeeded TestFlight build for the app.
	Lookup(ctx context.Context, req LookupRequest) (SourceBuild, error)
	// Promote references an existing TestFlight build in the App Store version
	// without triggering a new archive/upload.
	Promote(ctx context.Context, req PromoteRequest) (string, error)
}

// APIClient uploads IPAs with xcrun altool (API key auth) and drives the
// App Store Connect REST API for promote.
type APIClient struct {
	Runner execx.Runner
	// BaseURL overrides the ASC API root (tests). Empty means the production API.
	BaseURL string
	// HTTPClient overrides the default client (tests).
	HTTPClient *http.Client
}

func (c APIClient) Upload(ctx context.Context, req UploadRequest) (string, error) {
	if req.ArtifactPath == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "asc: empty artifact")
	}
	ipa, err := resolveIPA(req.ArtifactPath)
	if err != nil {
		return "", err
	}

	keyID := strings.TrimSpace(os.Getenv(EnvAPIKeyID))
	issuer := strings.TrimSpace(os.Getenv(EnvAPIIssuerID))
	keyPath := strings.TrimSpace(os.Getenv(EnvAPIKeyPath))
	if keyID == "" || issuer == "" {
		return "", ternerrors.NewHint(ternerrors.ClassUpload,
			"asc: set APP_STORE_CONNECT_API_KEY_ID and APP_STORE_CONNECT_API_ISSUER_ID",
			"App Store Connect → Users and Access → Integrations → Team Keys → create API key")
	}
	if keyPath != "" {
		if err := ensureAltoolPrivateKey(keyID, keyPath); err != nil {
			return "", err
		}
	}

	runner := c.Runner
	if runner == nil {
		runner = &execx.RealRunner{Stdout: os.Stdout, Stderr: os.Stderr}
	}
	if _, err := execx.LookPath("xcrun"); err != nil {
		return "", ternerrors.WrapHint(ternerrors.ClassUpload,
			"xcrun not found (macOS required for IPA upload)",
			"run iOS uploads on a Mac with Xcode Command Line Tools installed", err)
	}

	args := []string{
		"altool", "--upload-app",
		"-f", ipa,
		"-t", "ios",
		"--apiKey", keyID,
		"--apiIssuer", issuer,
	}
	if _, err := runner.Run(ctx, "", "xcrun", args...); err != nil {
		text := ternerrors.StderrOf(err)
		if text == "" {
			text = err.Error()
		}
		if classified := diagnose.Classify(ternerrors.ClassUpload, "asc: altool upload failed", text, err); classified != nil {
			return "", classified
		}
		return "", ternerrors.WrapHint(ternerrors.ClassUpload, "App Store Connect upload failed",
			"verify API key env vars and that the IPA uploads with `xcrun altool` manually", err)
	}
	dest := "app_store"
	if req.TestFlight {
		dest = "testflight"
	}
	return fmt.Sprintf("uploaded %s to %s via altool", filepath.Base(ipa), dest), nil
}

func resolveIPA(path string) (string, error) {
	info, err := os.Stat(path)
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassUpload, "asc: artifact", err)
	}
	if !info.IsDir() {
		if strings.EqualFold(filepath.Ext(path), ".ipa") {
			return path, nil
		}
		return "", ternerrors.New(ternerrors.ClassUpload, "asc: expected .ipa file")
	}
	var found string
	_ = filepath.Walk(path, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".ipa") {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", ternerrors.New(ternerrors.ClassUpload, "asc: no .ipa found under "+path)
	}
	return found, nil
}

// ensureAltoolPrivateKey copies/links the .p8 into a location altool searches.
func ensureAltoolPrivateKey(keyID, keyPath string) error {
	if _, err := os.Stat(keyPath); err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "asc: API key .p8", err)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "asc: home dir", err)
	}
	dir := filepath.Join(home, ".appstoreconnect", "private_keys")
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "asc: private_keys dir", err)
	}
	dest := filepath.Join(dir, fmt.Sprintf("AuthKey_%s.p8", keyID))
	if _, err := os.Stat(dest); err == nil {
		return nil
	}
	data, err := os.ReadFile(keyPath)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "asc: read .p8", err)
	}
	if err := os.WriteFile(dest, data, 0o600); err != nil {
		return ternerrors.Wrap(ternerrors.ClassUpload, "asc: install .p8", err)
	}
	return nil
}
