package signing

import (
	"context"
	"fmt"
	"os"
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
	With        string // keystore | cert
	EnvRef      string
	Artifact    string
	ProjectRoot string
	DryRun      bool
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
// iOS: validates cert/profile env; codesign is performed by `flutter build ipa`.
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
		return Result{
			Artifact: opts.Artifact,
			Message:  "ios signing material validated (flutter build ipa will codesign)",
		}, nil
	default:
		return Result{}, ternerrors.New(ternerrors.ClassSign, "unsupported platform")
	}
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
