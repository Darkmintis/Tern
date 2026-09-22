package bump

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"github.com/darkmintis/Tern/internal/config"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// Result of a version bump.
type Result struct {
	File    string
	Old     string
	New     string
	Message string
}

// BumpVersion bumps version in pubspec.yaml (Flutter) or build.gradle when present.
func BumpVersion(projectRoot string, level config.BumpLevel, dryRun bool) (Result, error) {
	pubspec := filepath.Join(projectRoot, "pubspec.yaml")
	if _, err := os.Stat(pubspec); err == nil {
		return bumpPubspec(pubspec, level, dryRun)
	}
	gradle := filepath.Join(projectRoot, "android", "app", "build.gradle")
	if _, err := os.Stat(gradle); err == nil {
		return bumpGradleVersionName(gradle, level, dryRun)
	}
	gradleKts := filepath.Join(projectRoot, "android", "app", "build.gradle.kts")
	if _, err := os.Stat(gradleKts); err == nil {
		return bumpGradleKts(gradleKts, level, dryRun)
	}
	return Result{}, ternerrors.New(ternerrors.ClassConfig, "no pubspec.yaml or build.gradle found to bump")
}

var pubspecVersionRe = regexp.MustCompile(`(?m)^version:\s*([0-9]+)\.([0-9]+)\.([0-9]+)(\+([0-9]+))?`)

func bumpPubspec(path string, level config.BumpLevel, dryRun bool) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, ternerrors.Wrap(ternerrors.ClassConfig, "reading pubspec", err)
	}
	m := pubspecVersionRe.FindSubmatch(data)
	if m == nil {
		return Result{}, ternerrors.New(ternerrors.ClassConfig, "no version: line in pubspec.yaml")
	}
	maj, _ := strconv.Atoi(string(m[1]))
	min, _ := strconv.Atoi(string(m[2]))
	pat, _ := strconv.Atoi(string(m[3]))
	build := 0
	if len(m[5]) > 0 {
		build, _ = strconv.Atoi(string(m[5]))
	}
	old := string(m[0])
	switch level {
	case config.BumpMajor:
		maj++
		min, pat = 0, 0
		build++
	case config.BumpMinor:
		min++
		pat = 0
		build++
	case config.BumpPatch:
		pat++
		build++
	case config.BumpBuild:
		build++
	default:
		pat++
		build++
	}
	newVer := fmt.Sprintf("version: %d.%d.%d", maj, min, pat)
	if build > 0 || level == config.BumpBuild || strings.Contains(old, "+") {
		if level != config.BumpBuild && build == 0 {
			build = 1
		}
		newVer = fmt.Sprintf("version: %d.%d.%d+%d", maj, min, pat, build)
	}
	if dryRun {
		return Result{File: path, Old: old, New: newVer, Message: "dry-run: " + old + " -> " + newVer}, nil
	}
	updated := pubspecVersionRe.ReplaceAll(data, []byte(newVer))
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return Result{}, ternerrors.Wrap(ternerrors.ClassConfig, "writing pubspec", err)
	}
	return Result{File: path, Old: old, New: newVer, Message: old + " -> " + newVer}, nil
}

var gradleVersionNameRe = regexp.MustCompile(`versionName\s+"([^"]+)"`)

func bumpGradleVersionName(path string, level config.BumpLevel, dryRun bool) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	m := gradleVersionNameRe.FindSubmatch(data)
	if m == nil {
		return Result{}, ternerrors.New(ternerrors.ClassConfig, "no versionName in build.gradle")
	}
	old := string(m[1])
	parts := strings.Split(old, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	nums := make([]int, 3)
	for i := 0; i < 3; i++ {
		nums[i], _ = strconv.Atoi(parts[i])
	}
	switch level {
	case config.BumpMajor:
		nums[0]++
		nums[1], nums[2] = 0, 0
	case config.BumpMinor:
		nums[1]++
		nums[2] = 0
	default:
		nums[2]++
	}
	newVer := fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2])
	repl := fmt.Sprintf(`versionName "%s"`, newVer)
	if dryRun {
		return Result{File: path, Old: old, New: newVer, Message: "dry-run: versionName " + old + " -> " + newVer}, nil
	}
	updated := gradleVersionNameRe.ReplaceAll(data, []byte(repl))
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return Result{}, err
	}
	return Result{File: path, Old: old, New: newVer, Message: "versionName " + old + " -> " + newVer}, nil
}

var (
	gradleKtsVersionNameRe = regexp.MustCompile(`versionName\s*=\s*"([^"]+)"`)
	gradleKtsVersionCodeRe = regexp.MustCompile(`versionCode\s*=\s*(\d+)`)
)

func bumpGradleKts(path string, level config.BumpLevel, dryRun bool) (Result, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return Result{}, err
	}
	m := gradleKtsVersionNameRe.FindSubmatch(data)
	if m == nil {
		return Result{}, ternerrors.New(ternerrors.ClassConfig, "no versionName in build.gradle.kts")
	}
	oldName := string(m[1])
	parts := strings.Split(oldName, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}
	nums := make([]int, 3)
	for i := 0; i < 3; i++ {
		nums[i], _ = strconv.Atoi(parts[i])
	}
	code := 0
	if cm := gradleKtsVersionCodeRe.FindSubmatch(data); cm != nil {
		code, _ = strconv.Atoi(string(cm[1]))
	}
	switch level {
	case config.BumpMajor:
		nums[0]++
		nums[1], nums[2] = 0, 0
		code++
	case config.BumpMinor:
		nums[1]++
		nums[2] = 0
		code++
	case config.BumpBuild:
		code++
	default:
		nums[2]++
		code++
	}
	newName := fmt.Sprintf("%d.%d.%d", nums[0], nums[1], nums[2])
	msg := fmt.Sprintf("versionName %s -> %s", oldName, newName)
	if dryRun {
		return Result{File: path, Old: oldName, New: newName, Message: "dry-run: " + msg}, nil
	}
	updated := gradleKtsVersionNameRe.ReplaceAll(data, []byte(fmt.Sprintf(`versionName = "%s"`, newName)))
	if gradleKtsVersionCodeRe.Match(updated) {
		updated = gradleKtsVersionCodeRe.ReplaceAll(updated, []byte(fmt.Sprintf("versionCode = %d", code)))
		msg += fmt.Sprintf("; versionCode -> %d", code)
	}
	if err := os.WriteFile(path, updated, 0o644); err != nil {
		return Result{}, err
	}
	return Result{File: path, Old: oldName, New: newName, Message: msg}, nil
}

// TagMessage returns the git tag name for a version string.
func TagName(prefix, version string) string {
	version = strings.TrimPrefix(version, "version: ")
	version = strings.Split(version, "+")[0]
	return prefix + version
}

// BumpPastStore bumps the marketing patch and sets +build to at least storeVC+1.
// Play rejects uploads when versionCode is not strictly greater than what is
// already on the track; this is the usual interactive/CI recovery path.
func BumpPastStore(projectRoot string, storeVC int64, dryRun bool) (Result, error) {
	if storeVC < 0 {
		storeVC = 0
	}
	pubspec := filepath.Join(projectRoot, "pubspec.yaml")
	if _, err := os.Stat(pubspec); err != nil {
		return Result{}, ternerrors.New(ternerrors.ClassConfig, "no pubspec.yaml to bump past store version")
	}
	data, err := os.ReadFile(pubspec)
	if err != nil {
		return Result{}, ternerrors.Wrap(ternerrors.ClassConfig, "reading pubspec", err)
	}
	m := pubspecVersionRe.FindSubmatch(data)
	if m == nil {
		return Result{}, ternerrors.New(ternerrors.ClassConfig, "no version: line in pubspec.yaml")
	}
	maj, _ := strconv.Atoi(string(m[1]))
	min, _ := strconv.Atoi(string(m[2]))
	pat, _ := strconv.Atoi(string(m[3]))
	build := 0
	if len(m[5]) > 0 {
		build, _ = strconv.Atoi(string(m[5]))
	}
	old := string(m[0])
	pat++
	wantBuild := int(storeVC) + 1
	if build+1 > wantBuild {
		wantBuild = build + 1
	}
	newVer := fmt.Sprintf("version: %d.%d.%d+%d", maj, min, pat, wantBuild)
	if dryRun {
		return Result{File: pubspec, Old: old, New: newVer, Message: "dry-run: " + old + " -> " + newVer}, nil
	}
	updated := pubspecVersionRe.ReplaceAll(data, []byte(newVer))
	if err := os.WriteFile(pubspec, updated, 0o644); err != nil {
		return Result{}, ternerrors.Wrap(ternerrors.ClassConfig, "writing pubspec", err)
	}
	return Result{File: pubspec, Old: old, New: newVer, Message: old + " -> " + newVer}, nil
}
