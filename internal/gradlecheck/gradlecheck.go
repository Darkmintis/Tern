package gradlecheck

import (
	"os"
	"path/filepath"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// FlutterAndroidSigningConfigured checks that the Flutter Android Gradle files
// load key.properties (required for Tern's sign step to take effect).
func FlutterAndroidSigningConfigured(projectRoot string) error {
	candidates := []string{
		filepath.Join(projectRoot, "android", "app", "build.gradle"),
		filepath.Join(projectRoot, "android", "app", "build.gradle.kts"),
	}
	var found bool
	var body string
	for _, path := range candidates {
		data, err := os.ReadFile(path)
		if err != nil {
			continue
		}
		found = true
		body = string(data)
		break
	}
	if !found {
		return ternerrors.NewHint(ternerrors.ClassDoctor,
			"android/app/build.gradle(.kts) not found",
			"run this inside a Flutter project with an android/ folder")
	}
	lower := strings.ToLower(body)
	if !strings.Contains(lower, "key.properties") {
		return ternerrors.NewHint(ternerrors.ClassDoctor,
			"android Gradle file does not reference key.properties",
			"add the standard Flutter key.properties signingConfig block so Tern's sign step can unlock release builds — see docs/getting-started.md")
	}
	if !strings.Contains(lower, "signingconfigs") && !strings.Contains(lower, "signingConfigs") {
		// kotlin dsl uses signingConfigs; groovy signingConfigs
		if !strings.Contains(body, "signingConfigs") {
			return ternerrors.NewHint(ternerrors.ClassDoctor,
				"android Gradle file has no signingConfigs block",
				"wire storeFile/storePassword from key.properties into signingConfigs.release")
		}
	}
	return nil
}
