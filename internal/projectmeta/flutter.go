package projectmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

var pubspecVersionRe = regexp.MustCompile(`(?m)^version:\s*([^\s#]+)`)

// FlutterVersion returns the version string from pubspec.yaml (e.g. 1.2.3+4).
func FlutterVersion(projectRoot string) (string, error) {
	data, err := os.ReadFile(filepath.Join(projectRoot, "pubspec.yaml"))
	if err != nil {
		return "", ternerrors.Wrap(ternerrors.ClassConfig, "reading pubspec.yaml", err)
	}
	m := pubspecVersionRe.FindSubmatch(data)
	if m == nil {
		return "", ternerrors.New(ternerrors.ClassConfig, "no version: line in pubspec.yaml")
	}
	return strings.TrimSpace(string(m[1])), nil
}

var pubspecNameRe = regexp.MustCompile(`(?m)^name:\s*([^\s#]+)`)

// AppDisplayName returns a human app name for release titles (env override, then pubspec name).
func AppDisplayName(projectRoot string) string {
	if v := strings.TrimSpace(os.Getenv("TERN_APP_NAME")); v != "" {
		return v
	}
	if v := strings.TrimSpace(os.Getenv("APP_DISPLAY_NAME")); v != "" {
		return v
	}
	data, err := os.ReadFile(filepath.Join(projectRoot, "pubspec.yaml"))
	if err != nil {
		return ""
	}
	if m := pubspecNameRe.FindSubmatch(data); len(m) == 2 {
		raw := strings.TrimSpace(string(m[1]))
		raw = strings.ReplaceAll(raw, "_", " ")
		if raw == "" {
			return ""
		}
		// Title-case first rune for nicer Play release names.
		r := []rune(raw)
		r[0] = []rune(strings.ToUpper(string(r[0])))[0]
		return string(r)
	}
	return ""
}

var (
	bundleIDPlistRe = regexp.MustCompile(`PRODUCT_BUNDLE_IDENTIFIER\s*=\s*([^;\s]+)`)
	bundleIDXmlRe   = regexp.MustCompile(`<key>CFBundleIdentifier</key>\s*<string>([^<]+)</string>`)
)

// IOSBundleID best-effort reads PRODUCT_BUNDLE_IDENTIFIER from ios project files.
func IOSBundleID(projectRoot string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("IOS_BUNDLE_ID")); v != "" {
		return v, nil
	}
	roots := []string{
		filepath.Join(projectRoot, "ios"),
	}
	var found string
	for _, root := range roots {
		_ = filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() || found != "" {
				return nil
			}
			name := info.Name()
			if name != "project.pbxproj" && name != "Info.plist" && !strings.HasSuffix(name, ".xcconfig") {
				return nil
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				return nil
			}
			s := string(data)
			if m := bundleIDPlistRe.FindStringSubmatch(s); len(m) == 2 {
				id := strings.TrimSpace(m[1])
				if !strings.Contains(id, "$(") {
					found = id
					return filepath.SkipAll
				}
			}
			if m := bundleIDXmlRe.FindStringSubmatch(s); len(m) == 2 {
				id := strings.TrimSpace(m[1])
				if !strings.Contains(id, "$(") {
					found = id
					return filepath.SkipAll
				}
			}
			return nil
		})
		if found != "" {
			return found, nil
		}
	}
	return "", ternerrors.New(ternerrors.ClassUpload,
		"could not detect iOS bundle id; set IOS_BUNDLE_ID")
}
