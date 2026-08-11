package flutter

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/darkmintis/Tern/internal/config"
)

func expectedArtifact(root string, platform config.Platform, mode config.Mode, artifactKind, flavor string) (kind, path string) {
	switch platform {
	case config.PlatformAndroid:
		wantAPK := mode == config.ModeDebug || artifactKind == "apk"
		if wantAPK {
			name := "app-debug.apk"
			if mode != config.ModeDebug {
				name = "app-release.apk"
				if flavor != "" {
					name = fmt.Sprintf("app-%s-release.apk", flavor)
				}
			} else if flavor != "" {
				name = fmt.Sprintf("app-%s-debug.apk", flavor)
			}
			return "apk", filepath.Join(root, "build", "app", "outputs", "flutter-apk", name)
		}
		if flavor != "" {
			// e.g. prodRelease/app-prod-release.aab
			dir := flavor + "Release"
			name := fmt.Sprintf("app-%s-release.aab", flavor)
			return "aab", filepath.Join(root, "build", "app", "outputs", "bundle", dir, name)
		}
		return "aab", filepath.Join(root, "build", "app", "outputs", "bundle", "release", "app-release.aab")
	case config.PlatformIOS:
		return "ipa", filepath.Join(root, "build", "ios", "ipa")
	default:
		return "", ""
	}
}

func findIPA(dirOrFile string) (string, error) {
	info, err := os.Stat(dirOrFile)
	if err == nil && !info.IsDir() && strings.EqualFold(filepath.Ext(dirOrFile), ".ipa") {
		return dirOrFile, nil
	}
	dir := dirOrFile
	var found string
	_ = filepath.Walk(dir, func(p string, fi os.FileInfo, err error) error {
		if err != nil || fi == nil || fi.IsDir() {
			return nil
		}
		if strings.EqualFold(filepath.Ext(p), ".ipa") {
			found = p
			return filepath.SkipAll
		}
		return nil
	})
	if found == "" {
		return "", fmt.Errorf("no ipa under %s", dir)
	}
	return found, nil
}

func resolveAndroidArtifact(preferred, root, kind, flavor string) (string, error) {
	if _, err := os.Stat(preferred); err == nil {
		return preferred, nil
	}
	// Same directory as preferred.
	dir := filepath.Dir(preferred)
	if found := newestWithExt(dir, filepath.Ext(preferred)); found != "" {
		return found, nil
	}
	// Broad search under build/app/outputs.
	ext := ".aab"
	if kind == "apk" {
		ext = ".apk"
	}
	bases := []string{
		filepath.Join(root, "build", "app", "outputs", "bundle"),
		filepath.Join(root, "build", "app", "outputs", "flutter-apk"),
		filepath.Join(root, "build", "app", "outputs", "apk"),
	}
	var candidates []string
	for _, base := range bases {
		_ = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
			if err != nil || fi == nil || fi.IsDir() {
				return nil
			}
			if !strings.EqualFold(filepath.Ext(p), ext) {
				return nil
			}
			if flavor != "" && !strings.Contains(strings.ToLower(p), strings.ToLower(flavor)) {
				return nil
			}
			candidates = append(candidates, p)
			return nil
		})
	}
	if len(candidates) == 0 && flavor != "" {
		// Retry without flavor filter.
		for _, base := range bases {
			_ = filepath.Walk(base, func(p string, fi os.FileInfo, err error) error {
				if err != nil || fi == nil || fi.IsDir() {
					return nil
				}
				if strings.EqualFold(filepath.Ext(p), ext) {
					candidates = append(candidates, p)
				}
				return nil
			})
		}
	}
	if len(candidates) == 0 {
		return "", fmt.Errorf("artifact not found: %s", preferred)
	}
	newest := candidates[0]
	var newestMod int64
	for _, c := range candidates {
		fi, err := os.Stat(c)
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() >= newestMod {
			newestMod = fi.ModTime().UnixNano()
			newest = c
		}
	}
	return newest, nil
}

func newestWithExt(dir, ext string) string {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return ""
	}
	var best string
	var bestMod int64
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ext) {
			continue
		}
		p := filepath.Join(dir, e.Name())
		fi, err := e.Info()
		if err != nil {
			continue
		}
		if fi.ModTime().UnixNano() >= bestMod {
			bestMod = fi.ModTime().UnixNano()
			best = p
		}
	}
	return best
}
