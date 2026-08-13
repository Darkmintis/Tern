package doctor

import (
	"fmt"
	"os"
	"strings"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
	"github.com/darkmintis/Tern/internal/gradlecheck"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/secrets"
	"github.com/darkmintis/Tern/internal/signing"
	"github.com/darkmintis/Tern/internal/upload/asc"
)

// Check is one doctor finding.
type Check struct {
	Name    string
	OK      bool
	Message string
	Hint    string
}

// Options for doctor.
type Options struct {
	ProjectRoot string
	Config      *config.Config
	Registry    *adapter.Registry
	Emitter     *output.Emitter
}

// Run validates toolchain, secrets referenced in Ternfile, and signing material.
func Run(opts Options) ([]Check, error) {
	var checks []Check
	root := opts.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}

	var detected string
	if opts.Registry != nil {
		if ad, ok := opts.Registry.Detect(root); ok {
			detected = ad.Name()
			checks = append(checks, Check{Name: "adapter", OK: true, Message: "detected " + ad.Name()})
		} else {
			checks = append(checks, Check{
				Name: "adapter", OK: false,
				Message: "no project adapter detected",
				Hint:    "Tern v0 supports Flutter projects (pubspec.yaml + flutter SDK). Native/KMP/RN come later.",
			})
		}
	}

	switch detected {
	case "flutter":
		checks = append(checks, requireTool("flutter"))
		if projectHasAndroid(root) || configNeedsAndroid(opts.Config) {
			sdkCheck := checkAndroidSDK()
			checks = append(checks, sdkCheck)
			checks = append(checks, checkJDK())
			sdkHome := strings.TrimSpace(os.Getenv("ANDROID_HOME"))
			if sdkHome == "" {
				sdkHome = strings.TrimSpace(os.Getenv("ANDROID_SDK_ROOT"))
			}
			checks = append(checks, checkFlutterDoctorQuick(sdkHome)...)
		}
		if err := gradlecheck.FlutterAndroidSigningConfigured(root); err != nil {
			checks = append(checks, Check{
				Name: "android_signing_gradle", OK: false,
				Message: err.Error(),
				Hint:    ternerrors.HintOf(err),
			})
		} else {
			checks = append(checks, Check{Name: "android_signing_gradle", OK: true, Message: "key.properties signing wired"})
		}
		if _, err := projectmeta.AndroidPackageID(root); err != nil {
			checks = append(checks, Check{
				Name: "android_package", OK: false,
				Message: err.Error(),
				Hint:    "set ANDROID_PACKAGE_NAME or ensure applicationId is in android/app/build.gradle",
			})
		} else {
			checks = append(checks, Check{Name: "android_package", OK: true, Message: "package id detected"})
		}
	default:
		checks = append(checks, optionalTool("flutter"))
	}

	if _, err := execx.LookPath("xcodebuild"); err == nil {
		checks = append(checks, Check{Name: "xcodebuild", OK: true, Message: "found"})
	} else {
		checks = append(checks, Check{Name: "xcodebuild", OK: true, Message: "not found (ok for android-only / non-macOS)"})
	}

	if opts.Config != nil {
		seen := map[string]bool{}
		needsAndroidSign := false
		needsPlay := false
		needsASC := false
		for _, lane := range opts.Config.Lanes {
			for _, step := range lane.Steps {
				if step.Kind == config.StepSign && step.Platform == config.PlatformAndroid {
					needsAndroidSign = true
				}
				if (step.Kind == config.StepUpload || step.Kind == config.StepShip) && step.UploadTarget == "play_store" {
					needsPlay = true
				}
				if (step.Kind == config.StepUpload || step.Kind == config.StepShip) && (step.UploadTarget == "testflight" || step.UploadTarget == "app_store") {
					needsASC = true
				}
				if step.Kind == config.StepSyncCerts {
					checks = append(checks, Check{
						Name: "sync_certs", OK: false,
						Message: "sync_certs is not production-ready in v0",
						Hint:    "remove sync_certs from your Ternfile for Flutter releases; encrypted cert sync comes later",
					})
				}
				if step.EnvRef == "" || seen[step.EnvRef] {
					continue
				}
				seen[step.EnvRef] = true
				err := secrets.CheckEnvStrong(step.EnvRef)
				if err != nil {
					checks = append(checks, Check{
						Name: "env:" + step.EnvRef, OK: false, Message: err.Error(),
						Hint: "copy .env.example → .env and set this value — see docs/setup.md",
					})
					continue
				}
				val := os.Getenv(step.EnvRef)
				msg := "present"
				if looksLikePath(val) {
					if ferr := secrets.FileReadable(val); ferr != nil {
						checks = append(checks, Check{Name: "env:" + step.EnvRef, OK: false, Message: ferr.Error()})
						continue
					}
					msg = "present, file readable"
					if strings.HasSuffix(strings.ToLower(val), ".mobileprovision") {
						if perr := signing.CheckProfileExpiry(val); perr != nil {
							checks = append(checks, Check{Name: "profile:" + step.EnvRef, OK: false, Message: perr.Error()})
						}
					}
				}
				checks = append(checks, Check{Name: "env:" + step.EnvRef, OK: true, Message: msg})
			}
		}
		if needsAndroidSign {
			for _, name := range []string{signing.EnvKeystorePassword, signing.EnvKeyAlias, signing.EnvKeyPassword} {
				if err := secrets.CheckEnvStrong(name); err != nil {
					checks = append(checks, Check{
						Name: "env:" + name, OK: false, Message: err.Error(),
						Hint: "set in .env from .env.example — docs/play-setup.md (keystore section)",
					})
				} else {
					checks = append(checks, Check{Name: "env:" + name, OK: true, Message: "present"})
				}
			}
		}
		if needsPlay {
			creds := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS")
			if strings.TrimSpace(creds) == "" {
				checks = append(checks, Check{
					Name: "env:GOOGLE_APPLICATION_CREDENTIALS", OK: false,
					Message: "missing Play service-account JSON path",
					Hint:    "follow docs/play-setup.md → set GOOGLE_APPLICATION_CREDENTIALS in .env",
				})
			} else if err := secrets.FileReadable(creds); err != nil {
				checks = append(checks, Check{
					Name: "env:GOOGLE_APPLICATION_CREDENTIALS", OK: false, Message: err.Error(),
					Hint: "path must exist; put play.json under secrets/ — docs/play-setup.md",
				})
			} else {
				checks = append(checks, Check{Name: "env:GOOGLE_APPLICATION_CREDENTIALS", OK: true, Message: "present, file readable"})
			}
		}
		if needsASC {
			for _, name := range []string{asc.EnvAPIKeyID, asc.EnvAPIIssuerID} {
				if err := secrets.CheckEnvPresent(name); err != nil {
					checks = append(checks, Check{
						Name: "env:" + name, OK: false, Message: err.Error(),
						Hint: "create an App Store Connect API key and export the id/issuer",
					})
				} else {
					checks = append(checks, Check{Name: "env:" + name, OK: true, Message: "present"})
				}
			}
			if p := os.Getenv(asc.EnvAPIKeyPath); p != "" {
				if err := secrets.FileReadable(p); err != nil {
					checks = append(checks, Check{Name: "env:" + asc.EnvAPIKeyPath, OK: false, Message: err.Error()})
				} else {
					checks = append(checks, Check{Name: "env:" + asc.EnvAPIKeyPath, OK: true, Message: "present, file readable"})
				}
			} else {
				checks = append(checks, Check{
					Name: "env:" + asc.EnvAPIKeyPath, OK: false,
					Message: "missing .p8 key path",
					Hint:    "export APP_STORE_CONNECT_API_KEY_PATH=/path/to/AuthKey_XXX.p8",
				})
			}
		}
	}

	em := opts.Emitter
	if em == nil {
		em = output.New(output.ModeHuman)
	}
	allOK := true
	for _, c := range checks {
		status := "ok"
		if !c.OK {
			status = "error"
			allOK = false
		}
		msg := c.Name + ": " + c.Message
		if c.Hint != "" && !c.OK {
			msg += " | hint: " + c.Hint
		}
		em.Emit(output.Event{Type: "doctor", Status: status, Message: msg, ErrorClass: ternary(!c.OK, string(ternerrors.ClassDoctor), "")})
	}
	if !allOK {
		return checks, ternerrors.NewHint(ternerrors.ClassDoctor,
			"one or more doctor checks failed",
			"fix each failing check above, then re-run: tern doctor")
	}
	return checks, nil
}

func ternary(cond bool, a, b string) string {
	if cond {
		return a
	}
	return b
}

func requireTool(name string) Check {
	if _, err := execx.LookPath(name); err != nil {
		return Check{Name: name, OK: false, Message: fmt.Sprintf("%s not found on PATH", name), Hint: "install Flutter and ensure `flutter` is on PATH"}
	}
	return Check{Name: name, OK: true, Message: "found"}
}

func optionalTool(name string) Check {
	if _, err := execx.LookPath(name); err != nil {
		return Check{Name: name, OK: true, Message: fmt.Sprintf("%s not found (optional)", name)}
	}
	return Check{Name: name, OK: true, Message: "found"}
}

func looksLikePath(v string) bool {
	if strings.HasPrefix(v, "git@") || strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://") || strings.HasPrefix(v, "ssh://") {
		return false
	}
	return strings.Contains(v, "/") || strings.Contains(v, `\`)
}
