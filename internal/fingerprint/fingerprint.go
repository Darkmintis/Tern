package fingerprint

import (
	"crypto/sha256"
	"encoding/hex"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/darkmintis/Tern/internal/config"
)

// Input for a platform build fingerprint.
type Input struct {
	ProjectRoot string
	Platform    config.Platform
	Mode        config.Mode
	Kind        string // aab, apk, ipa
	Flavor      string
	Scheme      string
}

// Compute hashes relevant sources + lockfiles for skip-rebuild decisions.
func Compute(in Input) (string, error) {
	h := sha256.New()
	_, _ = io.WriteString(h, "platform="+string(in.Platform)+"\n")
	_, _ = io.WriteString(h, "mode="+string(in.Mode)+"\n")
	_, _ = io.WriteString(h, "kind="+in.Kind+"\n")
	_, _ = io.WriteString(h, "flavor="+in.Flavor+"\n")
	_, _ = io.WriteString(h, "scheme="+in.Scheme+"\n")

	files := []string{
		filepath.Join(in.ProjectRoot, "pubspec.yaml"),
		filepath.Join(in.ProjectRoot, "pubspec.lock"),
	}
	switch in.Platform {
	case config.PlatformAndroid:
		files = append(files,
			filepath.Join(in.ProjectRoot, "android", "app", "build.gradle"),
			filepath.Join(in.ProjectRoot, "android", "app", "build.gradle.kts"),
			filepath.Join(in.ProjectRoot, "android", "gradle", "wrapper", "gradle-wrapper.properties"),
			filepath.Join(in.ProjectRoot, "android", "key.properties"),
		)
	case config.PlatformIOS:
		files = append(files,
			filepath.Join(in.ProjectRoot, "ios", "Podfile"),
			filepath.Join(in.ProjectRoot, "ios", "Podfile.lock"),
		)
	}

	// Hash lib/ and platform dirs (file paths + contents for regular files under size cap).
	dirs := []string{filepath.Join(in.ProjectRoot, "lib")}
	if in.Platform == config.PlatformAndroid {
		dirs = append(dirs, filepath.Join(in.ProjectRoot, "android"))
	}
	if in.Platform == config.PlatformIOS {
		dirs = append(dirs, filepath.Join(in.ProjectRoot, "ios"))
	}
	for _, d := range dirs {
		_ = filepath.Walk(d, func(path string, info os.FileInfo, err error) error {
			if err != nil || info == nil || info.IsDir() {
				return nil
			}
			// Skip heavy/generated noise.
			rel, _ := filepath.Rel(in.ProjectRoot, path)
			if strings.Contains(rel, "build/") || strings.Contains(rel, ".dart_tool/") ||
				strings.Contains(rel, "Pods/") || strings.HasSuffix(rel, ".tern/") {
				return nil
			}
			files = append(files, path)
			return nil
		})
	}

	sort.Strings(files)
	seen := map[string]bool{}
	for _, f := range files {
		if seen[f] {
			continue
		}
		seen[f] = true
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		_, _ = io.WriteString(h, f+"\n")
		_, _ = h.Write(data)
		_, _ = io.WriteString(h, "\n")
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

// Lockfiles returns paths used for dependency-skip decisions.
func Lockfiles(projectRoot string) []string {
	return []string{
		filepath.Join(projectRoot, "pubspec.lock"),
		filepath.Join(projectRoot, "ios", "Podfile.lock"),
		filepath.Join(projectRoot, "android", "gradle", "wrapper", "gradle-wrapper.properties"),
	}
}

// LockfileHash hashes existing lockfiles only.
func LockfileHash(projectRoot string) (string, error) {
	h := sha256.New()
	for _, f := range Lockfiles(projectRoot) {
		data, err := os.ReadFile(f)
		if err != nil {
			continue
		}
		_, _ = io.WriteString(h, f+"\n")
		_, _ = h.Write(data)
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}
