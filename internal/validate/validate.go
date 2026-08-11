package validate

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/projectmeta"
)

// Options for release validation.
type Options struct {
	ProjectRoot string
	Platform    config.Platform // empty = infer from artifact / target
	Artifact    string
	Target      string // play_store, testflight, app_store
	Force       bool
	Emitter     *output.Emitter
}

// Result of validation.
type Result struct {
	OK      bool
	Checks  []Check
	Message string
}

// Check is one validation item.
type Check struct {
	Name    string
	OK      bool
	Message string
}

// Run performs pre-upload validation (version, credentials, artifact, package id).
func Run(opts Options) (Result, error) {
	root := opts.ProjectRoot
	if root == "" {
		root, _ = os.Getwd()
	}
	em := opts.Emitter
	if em == nil {
		em = output.New(output.ModeHuman)
	}
	var checks []Check
	fail := func(name, msg string) {
		checks = append(checks, Check{Name: name, OK: false, Message: msg})
		em.Emit(output.Event{Type: "validate", Status: "error", Message: name + ": " + msg})
	}
	pass := func(name, msg string) {
		checks = append(checks, Check{Name: name, OK: true, Message: msg})
		em.Emit(output.Event{Type: "validate", Status: "ok", Message: name + ": " + msg})
	}

	ver, err := projectmeta.FlutterVersion(root)
	if err != nil {
		fail("version", err.Error())
	} else {
		pass("version", "pubspec version "+ver)
	}

	platform := opts.Platform
	if platform == "" {
		switch opts.Target {
		case "play_store":
			platform = config.PlatformAndroid
		case "testflight", "app_store":
			platform = config.PlatformIOS
		default:
			platform = config.PlatformAndroid
		}
	}

	artPath := opts.Artifact
	var rec artifacts.Record
	if artPath == "" || artPath == "last" {
		p, r, rerr := artifacts.ResolvePath(root, platform, "last")
		if rerr != nil {
			fail("artifact", rerr.Error())
		} else {
			artPath = p
			rec = r
			pass("artifact", fmt.Sprintf("%s sha256=%s", artPath, short(rec.SHA256)))
		}
	} else {
		if verr := artifacts.Verify(artPath, "", ""); verr != nil {
			fail("artifact", verr.Error())
		} else {
			sum, size, _ := artifacts.HashFile(artPath)
			rec = artifacts.Record{Path: artPath, SHA256: sum, SizeBytes: size, Platform: platform}
			pass("artifact", fmt.Sprintf("%s (%d bytes) sha256=%s", artPath, size, short(sum)))
		}
	}

	if artPath != "" {
		ext := strings.ToLower(filepath.Ext(artPath))
		switch {
		case opts.Target == "play_store" && ext != ".aab" && ext != ".apk":
			fail("extension", "Play Store expects .aab or .apk, got "+ext)
		case (opts.Target == "testflight" || opts.Target == "app_store") && ext != ".ipa":
			fail("extension", "App Store / TestFlight expects .ipa, got "+ext)
		default:
			if ext != "" {
				pass("extension", ext)
			}
		}
		if ver != "" && rec.Version != "" {
			base := strings.Split(ver, "+")[0]
			if !strings.HasPrefix(rec.Version, base) {
				fail("version_match", fmt.Sprintf("artifact version %q vs pubspec %q", rec.Version, ver))
			} else {
				pass("version_match", "consistent with "+ver)
			}
		} else if ver != "" {
			pass("version_match", "pubspec "+ver+" (artifact metadata may omit version)")
		}
	}

	switch opts.Target {
	case "play_store":
		if path := os.Getenv("GOOGLE_APPLICATION_CREDENTIALS"); path == "" {
			fail("play_credentials", "GOOGLE_APPLICATION_CREDENTIALS unset")
		} else if _, serr := os.Stat(path); serr != nil {
			fail("play_credentials", "credentials file missing: "+path)
		} else {
			pass("play_credentials", path)
		}
		if id, ierr := projectmeta.AndroidPackageID(root); ierr != nil {
			fail("package_id", ierr.Error())
		} else {
			pass("package_id", id)
		}
	case "testflight", "app_store":
		for _, k := range []string{"APP_STORE_CONNECT_API_KEY_ID", "APP_STORE_CONNECT_API_ISSUER_ID", "APP_STORE_CONNECT_API_KEY_PATH"} {
			if os.Getenv(k) == "" {
				fail("asc_"+strings.ToLower(k), k+" unset")
			} else {
				pass("asc_"+strings.ToLower(k), "set")
			}
		}
		if bid, ierr := projectmeta.IOSBundleID(root); ierr != nil {
			fail("bundle_id", ierr.Error())
		} else {
			pass("bundle_id", bid)
		}
	}

	ok := true
	for _, c := range checks {
		if !c.OK {
			ok = false
			break
		}
	}
	res := Result{OK: ok || opts.Force, Checks: checks}
	if opts.Force && !ok {
		res.Message = "validation failed but --force set"
		return res, nil
	}
	if !ok {
		res.Message = "pre-release validation failed"
		return res, ternerrors.NewHint(ternerrors.ClassUpload, res.Message,
			"fix issues above, or pass --force to upload anyway (not recommended)")
	}
	res.Message = "validation passed"
	return res, nil
}

func short(s string) string {
	if len(s) > 12 {
		return s[:12] + "…"
	}
	return s
}
