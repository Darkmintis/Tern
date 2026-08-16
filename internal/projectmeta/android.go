package projectmeta

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

var (
	applicationIDRe = regexp.MustCompile(`applicationId\s*(?:=)?\s*["']([^"']+)["']`)
	namespaceRe     = regexp.MustCompile(`namespace\s*(?:=)?\s*["']([^"']+)["']`)
)

// AndroidPackageID reads applicationId / namespace from the Flutter android app gradle files.
func AndroidPackageID(projectRoot string) (string, error) {
	if v := strings.TrimSpace(os.Getenv("ANDROID_PACKAGE_NAME")); v != "" {
		return v, nil
	}
	candidates := []string{
		filepath.Join(projectRoot, "android", "app", "build.gradle"),
		filepath.Join(projectRoot, "android", "app", "build.gradle.kts"),
	}
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		s := string(data)
		if m := applicationIDRe.FindStringSubmatch(s); len(m) == 2 {
			return m[1], nil
		}
		if m := namespaceRe.FindStringSubmatch(s); len(m) == 2 {
			return m[1], nil
		}
	}
	return "", ternerrors.New(ternerrors.ClassUpload,
		"could not detect Android package name; set ANDROID_PACKAGE_NAME")
}
