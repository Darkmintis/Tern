package signing

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/secrets"
)

// Manager handles keystore / cert / profile validation and signing orchestration.
type Manager struct{}

func NewManager() *Manager { return &Manager{} }

// SignOptions describe a sign step.
type SignOptions struct {
	Platform    config.Platform
	With        string // keystore | cert | keychain
	EnvRef      string
	Artifact    string
	ProjectRoot string
	DryRun      bool
	// iOS-specific options
	KeychainPath     string
	KeychainPassword string
	CertificatePath  string
	ProfilePath      string
	TeamID           string
}

// Result of a sign operation.
type Result struct {
	Artifact string
	Message  string
}

// Android keystore-related env vars (in addition to ANDROID_KEYSTORE path).
const (
	EnvKeystorePassword = "ANDROID_KEYSTORE_PASSWORD"
	EnvKeyAlias         = "ANDROID_KEY_ALIAS"
	EnvKeyPassword      = "ANDROID_KEY_PASSWORD"
)

// iOS signing env vars.
const (
	EnvIOSCert         = "IOS_CERTIFICATE"
	EnvIOSCertPass     = "IOS_CERTIFICATE_PASSWORD"
	EnvIOSProfile      = "IOS_PROVISIONING_PROFILE"
	EnvIOSTeamID       = "IOS_TEAM_ID"
	EnvIOSKeychain     = "IOS_KEYCHAIN_PATH"
	EnvIOSKeychainPass = "IOS_KEYCHAIN_PASSWORD"
)

// ValidateRefs checks env and referenced files without signing.
func (m *Manager) ValidateRefs(opts SignOptions) error {
	val, err := secrets.ResolveEnv(opts.EnvRef)
	if err != nil {
		return err
	}
	if secrets.IsWeak(val) {
		return ternerrors.NewHint(ternerrors.ClassDoctor, "weak secret for env:"+opts.EnvRef,
			"use a strong password/path from CI secrets — not placeholder values like 'password'")
	}
	if stringsLooksLikePath(val) {
		if err := secrets.FileReadable(val); err != nil {
			return ternerrors.WrapHint(ternerrors.ClassSign, "signing material missing or unreadable",
				"export env:"+opts.EnvRef+" to a readable keystore/cert path", err)
		}
	}
	if opts.Platform == config.PlatformAndroid {
		for _, name := range []string{EnvKeystorePassword, EnvKeyAlias, EnvKeyPassword} {
			if err := secrets.CheckEnvStrong(name); err != nil {
				return ternerrors.WrapHint(ternerrors.ClassSign, "android signing env incomplete",
					"set ANDROID_KEYSTORE_PASSWORD, ANDROID_KEY_ALIAS, and ANDROID_KEY_PASSWORD", err)
			}
		}
	}
	if opts.Platform == config.PlatformIOS {
		for _, name := range []string{EnvIOSCert, EnvIOSProfile, EnvIOSTeamID} {
			if err := secrets.CheckEnvStrong(name); err != nil {
				return ternerrors.WrapHint(ternerrors.ClassSign, "ios signing env incomplete",
					"set IOS_CERTIFICATE, IOS_PROVISIONING_PROFILE, and IOS_TEAM_ID", err)
			}
		}
	}
	return nil
}

func stringsLooksLikePath(v string) bool {
	if strings.HasPrefix(v, "git@") || strings.HasPrefix(v, "http://") ||
		strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "ssh://") {
		return false
	}
	return stringsHasPathSep(v) || filepath.Ext(v) != ""
}

func stringsHasPathSep(v string) bool {
	for _, c := range v {
		if c == '/' || c == '\\' {
			return true
		}
	}
	return false
}

// Sign prepares / validates signing material for the upcoming or completed build.
// Android: writes android/key.properties from env so Flutter Gradle signing works.
// iOS: imports cert to keychain, validates profile, codesign is performed by flutter build.
func (m *Manager) Sign(ctx context.Context, opts SignOptions) (Result, error) {
	_ = ctx
	if err := m.ValidateRefs(opts); err != nil {
		return Result{}, err
	}
	if opts.DryRun {
		msg := fmt.Sprintf("dry-run: would sign %s with %s via env:%s", opts.Platform, opts.With, opts.EnvRef)
		if opts.Platform == config.PlatformAndroid {
			msg = "dry-run: would write android/key.properties from ANDROID_KEYSTORE* env"
		}
		if opts.Platform == config.PlatformIOS {
			msg = "dry-run: would import certificate and validate provisioning profile"
		}
		return Result{Artifact: opts.Artifact, Message: msg}, nil
	}
	switch opts.Platform {
	case config.PlatformAndroid:
		path, err := WriteAndroidKeyProperties(opts.ProjectRoot, opts.EnvRef)
		if err != nil {
			return Result{}, err
		}
		return Result{
			Artifact: opts.Artifact,
			Message:  "wrote " + path + " for Flutter/Gradle release signing",
		}, nil
	case config.PlatformIOS:
		if err := m.setupIOSKeychain(ctx, opts); err != nil {
			return Result{}, err
		}
		if err := m.importIOSCertificate(ctx, opts); err != nil {
			return Result{}, err
		}
		if err := m.validateIOSProfile(opts); err != nil {
			return Result{}, err
		}
		return Result{
			Artifact: opts.Artifact,
			Message:  "iOS signing material imported and validated",
		}, nil
	default:
		return Result{}, ternerrors.New(ternerrors.ClassSign, "unsupported platform")
	}
}

// setupIOSKeychain creates or unlocks the macOS keychain for signing.
func (m *Manager) setupIOSKeychain(ctx context.Context, opts SignOptions) error {
	keychainPath := opts.KeychainPath
	if keychainPath == "" {
		keychainPath = os.Getenv(EnvIOSKeychain)
	}
	if keychainPath == "" {
		home, _ := os.UserHomeDir()
		keychainPath = filepath.Join(home, "Library", "Keychains", "tern-signing.keychain-db")
	}

	keychainPass := opts.KeychainPassword
	if keychainPass == "" {
		keychainPass = os.Getenv(EnvIOSKeychainPass)
	}
	if keychainPass == "" {
		keychainPass = "tern-temp-keychain"
	}

	// Create keychain if it doesn't exist
	if _, err := os.Stat(keychainPath); os.IsNotExist(err) {
		cmd := exec.CommandContext(ctx, "security", "create-keychain", "-p", keychainPass, keychainPath)
		if out, err := cmd.CombinedOutput(); err != nil {
			return ternerrors.WrapHint(ternerrors.ClassSign, "create keychain failed",
				"ensure 'security' command is available (macOS only)", fmt.Errorf("%s: %s", err, out))
		}
		// Set keychain settings
		settingsCmd := exec.CommandContext(ctx, "security", "set-keychain-settings",
			"-lut", "21600", keychainPath)
		_ = settingsCmd.Run()
	}

	// Unlock keychain
	unlockCmd := exec.CommandContext(ctx, "security", "unlock-keychain", "-p", keychainPass, keychainPath)
	if out, err := unlockCmd.CombinedOutput(); err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "unlock keychain failed",
			"check keychain password", fmt.Errorf("%s: %s", err, out))
	}

	// Add to search list
	listCmd := exec.CommandContext(ctx, "security", "list-keychains", "-d", "user", "-s", keychainPath)
	_ = listCmd.Run()

	return nil
}

// importIOSCertificate imports a .p12 certificate into the keychain.
func (m *Manager) importIOSCertificate(ctx context.Context, opts SignOptions) error {
	certPath := opts.CertificatePath
	if certPath == "" {
		certPath = os.Getenv(EnvIOSCert)
	}
	if certPath == "" {
		return ternerrors.NewHint(ternerrors.ClassSign, "no iOS certificate specified",
			"set IOS_CERTIFICATE env to .p12 file path")
	}

	certPass := os.Getenv(EnvIOSCertPass)

	keychainPath := opts.KeychainPath
	if keychainPath == "" {
		keychainPath = os.Getenv(EnvIOSKeychain)
	}
	if keychainPath == "" {
		home, _ := os.UserHomeDir()
		keychainPath = filepath.Join(home, "Library", "Keychains", "tern-signing.keychain-db")
	}

	// Import certificate
	cmd := exec.CommandContext(ctx, "security", "import", certPath,
		"-k", keychainPath,
		"-T", "/usr/bin/codesign",
		"-T", "/usr/bin/security",
		"-P", certPass,
		"-A")
	if out, err := cmd.CombinedOutput(); err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "import certificate failed",
			"ensure IOS_CERTIFICATE points to a valid .p12 file", fmt.Errorf("%s: %s", err, out))
	}

	// Set key partition list for codesign access
	partitionCmd := exec.CommandContext(ctx, "security", "set-key-partition-list",
		"-S", "apple-tool:,apple:", "-s",
		"-k", keychainPath)
	_ = partitionCmd.Run()

	return nil
}

// validateIOSProfile checks the provisioning profile exists and is not expired.
func (m *Manager) validateIOSProfile(opts SignOptions) error {
	profilePath := opts.ProfilePath
	if profilePath == "" {
		profilePath = os.Getenv(EnvIOSProfile)
	}
	if profilePath == "" {
		return ternerrors.NewHint(ternerrors.ClassSign, "no provisioning profile specified",
			"set IOS_PROVISIONING_PROFILE env to .mobileprovision file path")
	}

	if err := secrets.FileReadable(profilePath); err != nil {
		return ternerrors.WrapHint(ternerrors.ClassSign, "provisioning profile missing",
			"ensure IOS_PROVISIONING_PROFILE points to a valid .mobileprovision file", err)
	}

	// Check expiry
	if err := CheckProfileExpiry(profilePath); err != nil {
		return err
	}

	return nil
}

// WriteAndroidKeyProperties writes android/key.properties for Flutter release builds.
func WriteAndroidKeyProperties(projectRoot, keystoreEnv string) (string, error) {
	if projectRoot == "" {
		return "", ternerrors.New(ternerrors.ClassSign, "project root required for android signing")
	}
	storeFile, err := secrets.ResolveEnv(keystoreEnv)
	if err != nil {
		return "", err
	}
	storePass, err := secrets.ResolveEnv(EnvKeystorePassword)
	if err != nil {
		return "", err
	}
	alias, err := secrets.ResolveEnv(EnvKeyAlias)
	if err != nil {
		return "", err
	}
	keyPass, err := secrets.ResolveEnv(EnvKeyPassword)
	if err != nil {
		return "", err
	}
	// Prefer absolute path so Gradle resolves reliably from android/.
	if !filepath.IsAbs(storeFile) {
		abs, aerr := filepath.Abs(storeFile)
		if aerr == nil {
			storeFile = abs
		}
	}
	if err := secrets.FileReadable(storeFile); err != nil {
		return "", ternerrors.WrapHint(ternerrors.ClassSign, "keystore file missing or unreadable",
			"export ANDROID_KEYSTORE to a real .jks/.keystore path", err)
	}

	androidDir := filepath.Join(projectRoot, "android")
	if err := os.MkdirAll(androidDir, 0o755); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassSign, "android dir", err)
	}
	out := filepath.Join(androidDir, "key.properties")
	content := fmt.Sprintf(
		"storePassword=%s\nkeyPassword=%s\nkeyAlias=%s\nstoreFile=%s\n",
		storePass, keyPass, alias, storeFile,
	)
	if err := os.WriteFile(out, []byte(content), 0o600); err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassSign, "writing key.properties", err)
	}
	return out, nil
}

// CheckProfileExpiry returns an error if the profile file looks stale.
func CheckProfileExpiry(path string) error {
	if err := secrets.FileReadable(path); err != nil {
		return err
	}
	info, err := os.Stat(path)
	if err != nil {
		return ternerrors.Wrap(ternerrors.ClassDoctor, "profile stat", err)
	}
	if time.Since(info.ModTime()) > 365*24*time.Hour {
		return ternerrors.New(ternerrors.ClassDoctor, "provisioning profile may be expired: "+path)
	}
	return nil
}

// CleanupKeychain removes the temporary signing keychain.
func (m *Manager) CleanupKeychain(ctx context.Context, keychainPath string) error {
	if keychainPath == "" {
		home, _ := os.UserHomeDir()
		keychainPath = filepath.Join(home, "Library", "Keychains", "tern-signing.keychain-db")
	}
	cmd := exec.CommandContext(ctx, "security", "delete-keychain", keychainPath)
	_ = cmd.Run()
	return nil
}
