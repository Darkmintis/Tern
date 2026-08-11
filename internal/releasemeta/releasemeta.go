package releasemeta

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/projectmeta"
)

// DefaultNotes is the generic Play/App Store copy most teams ship with.
const DefaultNotes = "Bug fixes and improvements."

// DefaultLocale for store release notes.
const DefaultLocale = "en-US"

// NameStrategy is how Tern derives the store release name.
type NameStrategy string

const (
	// NameVersion — marketing version only: 1.2.3 (default, most common on Play).
	NameVersion NameStrategy = "version"
	// NameVersionBuild — 1.2.3 (45) Play Console style.
	NameVersionBuild NameStrategy = "version_build"
	// NameVersionCode — build number only: 45.
	NameVersionCode NameStrategy = "version_code"
	// NameSemverPlus — keep pubspec form: 1.2.3+45.
	NameSemverPlus NameStrategy = "semver_plus"
	// NameNameVersion — AppName 1.2.3.
	NameNameVersion NameStrategy = "name_version"
	// NameDate — UTC date: 2026-08-11.
	NameDate NameStrategy = "date"
	// NameVersionDate — 1.2.3 · 2026-08-11.
	NameVersionDate NameStrategy = "version_date"
	// NameGitTag — current git tag / describe.
	NameGitTag NameStrategy = "git_tag"
	// NameGitSHA — short commit SHA.
	NameGitSHA NameStrategy = "git_sha"
	// NameCustom — literal NameCustom.
	NameCustom NameStrategy = "custom"
	// NameNone — omit release name on the store API.
	NameNone NameStrategy = "none"
)

// NotesMode controls release notes body.
type NotesMode string

const (
	NotesDefault NotesMode = "default" // DefaultNotes
	NotesNone    NotesMode = "none"
	NotesText    NotesMode = "text"
	NotesFile    NotesMode = "file"
)

// Spec is the unresolved Ternfile/CLI intent.
type Spec struct {
	NameStrategy NameStrategy
	NameCustom   string
	NotesMode    NotesMode
	NotesText    string
	NotesFile    string
	NotesLocale  string
}

// Resolved is ready for Play / ASC APIs.
type Resolved struct {
	Name        string // empty if NameNone
	Notes       string // empty if NotesNone
	NotesLocale string
	Version     string // raw project version (e.g. 1.2.3+45)
	Marketing   string // 1.2.3
	Build       string // 45
}

// DefaultSpec returns production-friendly defaults.
func DefaultSpec() Spec {
	return Spec{
		NameStrategy: NameVersion,
		NotesMode:    NotesDefault,
		NotesLocale:  DefaultLocale,
	}
}

// Resolve builds store release name + notes from the project.
func Resolve(projectRoot string, spec Spec) (Resolved, error) {
	if spec.NameStrategy == "" {
		spec.NameStrategy = NameVersion
	}
	if spec.NotesMode == "" {
		spec.NotesMode = NotesDefault
	}
	if spec.NotesLocale == "" {
		spec.NotesLocale = DefaultLocale
	}

	ver, err := projectmeta.FlutterVersion(projectRoot)
	if err != nil {
		ver = ""
	}
	marketing, build := SplitVersion(ver)

	out := Resolved{
		Version:     ver,
		Marketing:   marketing,
		Build:       build,
		NotesLocale: spec.NotesLocale,
	}

	switch spec.NameStrategy {
	case NameNone:
		out.Name = ""
	case NameCustom:
		out.Name = strings.TrimSpace(spec.NameCustom)
		if out.Name == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name custom requires a non-empty value")
		}
	case NameVersion:
		if marketing == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:version needs a project version")
		}
		out.Name = marketing
	case NameVersionBuild:
		if marketing == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:version_build needs a project version")
		}
		if build != "" {
			out.Name = fmt.Sprintf("%s (%s)", marketing, build)
		} else {
			out.Name = marketing
		}
	case NameVersionCode:
		if build == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:version_code needs a +build in version")
		}
		out.Name = build
	case NameSemverPlus:
		if ver == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:semver_plus needs a project version")
		}
		out.Name = ver
	case NameNameVersion:
		if marketing == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:name_version needs a project version")
		}
		app := projectmeta.AppDisplayName(projectRoot)
		if app == "" {
			app = "App"
		}
		out.Name = app + " " + marketing
	case NameDate:
		out.Name = time.Now().UTC().Format("2006-01-02")
	case NameVersionDate:
		if marketing == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:version_date needs a project version")
		}
		out.Name = marketing + " · " + time.Now().UTC().Format("2006-01-02")
	case NameGitTag:
		tag, gerr := gitOutput(projectRoot, "describe", "--tags", "--always")
		if gerr != nil || tag == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:git_tag requires a git tag or describe")
		}
		out.Name = tag
	case NameGitSHA:
		sha, gerr := gitOutput(projectRoot, "rev-parse", "--short", "HEAD")
		if gerr != nil || sha == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release_name:git_sha requires a git commit")
		}
		out.Name = sha
	default:
		return Resolved{}, ternerrors.New(ternerrors.ClassConfig,
			fmt.Sprintf("unknown release_name strategy %q (use version|version_build|version_code|semver_plus|name_version|date|version_date|git_tag|git_sha|custom|none)", spec.NameStrategy))
	}

	switch spec.NotesMode {
	case NotesNone:
		out.Notes = ""
	case NotesDefault:
		out.Notes = DefaultNotes
	case NotesText:
		out.Notes = strings.TrimSpace(spec.NotesText)
		if out.Notes == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "notes text is empty")
		}
	case NotesFile:
		path := spec.NotesFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(projectRoot, path)
		}
		data, rerr := os.ReadFile(path)
		if rerr != nil {
			return Resolved{}, ternerrors.Wrap(ternerrors.ClassConfig, "reading release notes file", rerr)
		}
		out.Notes = strings.TrimSpace(string(data))
		if out.Notes == "" {
			return Resolved{}, ternerrors.New(ternerrors.ClassConfig, "release notes file is empty: "+spec.NotesFile)
		}
	default:
		return Resolved{}, ternerrors.New(ternerrors.ClassConfig,
			fmt.Sprintf("unknown notes mode %q (use default|none|text|file)", spec.NotesMode))
	}

	return out, nil
}

// SplitVersion splits 1.2.3+45 into marketing + build.
func SplitVersion(ver string) (marketing, build string) {
	ver = strings.TrimSpace(ver)
	if ver == "" {
		return "", ""
	}
	if i := strings.IndexByte(ver, '+'); i >= 0 {
		return ver[:i], ver[i+1:]
	}
	return ver, ""
}

// ParseNameToken parses release_name:VALUE from Ternfile extras.
func ParseNameToken(value string) (strategy NameStrategy, custom string, err error) {
	value = strings.TrimSpace(value)
	if value == "" {
		return NameVersion, "", nil
	}
	switch NameStrategy(value) {
	case NameVersion, NameVersionBuild, NameVersionCode, NameSemverPlus,
		NameNameVersion, NameDate, NameVersionDate, NameGitTag, NameGitSHA, NameNone:
		return NameStrategy(value), "", nil
	case NameCustom:
		return NameCustom, "", ternerrors.New(ternerrors.ClassConfig, `use release_name:"Your title" for a custom name`)
	default:
		return NameCustom, value, nil
	}
}

// ParseNotesToken parses notes:VALUE from Ternfile extras.
func ParseNotesToken(value string) (mode NotesMode, text, file string, err error) {
	value = strings.TrimSpace(value)
	switch {
	case value == "" || value == "default":
		return NotesDefault, "", "", nil
	case value == "none":
		return NotesNone, "", "", nil
	case strings.HasPrefix(value, "file:"):
		f := strings.TrimPrefix(value, "file:")
		if f == "" {
			return "", "", "", ternerrors.New(ternerrors.ClassConfig, "notes:file: requires a path")
		}
		return NotesFile, "", f, nil
	default:
		return NotesText, value, "", nil
	}
}

func gitOutput(dir string, args ...string) (string, error) {
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}
