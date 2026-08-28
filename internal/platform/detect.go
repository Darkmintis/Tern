package platform

import (
	"os"
	"path/filepath"
	"runtime"
)

// DetectANDROID_HOME finds the Android SDK from common locations.
func DetectANDROIDHOME() string {
	// 1. Check env var first
	if v := os.Getenv("ANDROID_HOME"); v != "" {
		return v
	}
	if v := os.Getenv("ANDROID_SDK_ROOT"); v != "" {
		return v
	}

	// 2. Check common locations based on OS
	home, _ := os.UserHomeDir()
	candidates := []string{}

	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			filepath.Join(home, "AppData", "Local", "Android", "Sdk"),
			"C:\\Android\\sdk",
			"C:\\Program Files\\Android\\sdk",
			"C:\\Program Files (x86)\\Android\\sdk",
		}
	case "darwin":
		candidates = []string{
			filepath.Join(home, "Library", "Android", "sdk"),
			"/opt/android-sdk",
			"/usr/local/android-sdk",
		}
	case "linux":
		candidates = []string{
			filepath.Join(home, "Android", "Sdk"),
			"/opt/android-sdk",
			"/usr/local/android-sdk",
			filepath.Join(home, "android-sdk"),
		}
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			// Verify it looks like an SDK (has platforms/ or build-tools/)
			if _, err := os.Stat(filepath.Join(p, "platforms")); err == nil {
				return p
			}
			if _, err := os.Stat(filepath.Join(p, "build-tools")); err == nil {
				return p
			}
		}
	}

	return ""
}

// DetectJAVA_HOME finds Java from common locations.
func DetectJAVAHOME() string {
	if v := os.Getenv("JAVA_HOME"); v != "" {
		return v
	}

	home, _ := os.UserHomeDir()
	candidates := []string{}

	switch runtime.GOOS {
	case "windows":
		candidates = []string{
			filepath.Join(home, "AppData", "Local", "Programs", "Eclipse Adoptium"),
			"C:\\Program Files\\Eclipse Adoptium",
			"C:\\Program Files\\Java",
			"C:\\Program Files\\Android\\Android Studio\\jbr",
		}
	case "darwin":
		candidates = []string{
			"/Library/Java/JavaVirtualMachines",
			filepath.Join(home, ".sdkman", "candidates", "java"),
		}
	case "linux":
		candidates = []string{
			"/usr/lib/jvm",
			filepath.Join(home, ".sdkman", "candidates", "java"),
		}
	}

	for _, root := range candidates {
		entries, err := os.ReadDir(root)
		if err != nil {
			continue
		}
		// Return the first valid JDK found
		for _, e := range entries {
			jdkPath := filepath.Join(root, e.Name())
			// Check for javac (indicates a JDK, not just JRE)
			if _, err := os.Stat(filepath.Join(jdkPath, "bin", "javac")); err == nil {
				return jdkPath
			}
			// Windows: check bin/javac.exe
			if _, err := os.Stat(filepath.Join(jdkPath, "bin", "javac.exe")); err == nil {
				return jdkPath
			}
		}
	}

	return ""
}

// DetectFlutter finds the Flutter SDK.
func DetectFLUTTERHOME() string {
	if v := os.Getenv("FLUTTER_ROOT"); v != "" {
		return v
	}
	if v := os.Getenv("FLUTTER_HOME"); v != "" {
		return v
	}

	// Check common locations
	home, _ := os.UserHomeDir()
	candidates := []string{
		filepath.Join(home, "flutter"),
		filepath.Join(home, "development", "flutter"),
		"/opt/flutter",
		"/usr/local/flutter",
	}

	// Also check PATH
	if p, err := filepath.Abs("flutter"); err == nil {
		candidates = append(candidates, filepath.Dir(p))
	}

	for _, p := range candidates {
		if info, err := os.Stat(p); err == nil && info.IsDir() {
			if _, err := os.Stat(filepath.Join(p, "bin", "flutter")); err == nil {
				return p
			}
			if _, err := os.Stat(filepath.Join(p, "bin", "flutter.bat")); err == nil {
				return p
			}
		}
	}

	return ""
}

// CI represents a detected CI environment.
type CI struct {
	Name     string // github_actions, gitlab_ci, jenkins, circleci, etc.
	Branch   string
	Commit   string
	PRNumber string
	IsCI     bool
}

// DetectCI identifies the CI/CD environment.
func DetectCI() CI {
	// GitHub Actions
	if v := os.Getenv("GITHUB_ACTIONS"); v != "" {
		return CI{
			Name:     "github_actions",
			Branch:   os.Getenv("GITHUB_REF_NAME"),
			Commit:   os.Getenv("GITHUB_SHA"),
			PRNumber: os.Getenv("GITHUB_PULL_REQUEST_NUMBER"),
			IsCI:     true,
		}
	}

	// GitLab CI
	if v := os.Getenv("GITLAB_CI"); v != "" {
		return CI{
			Name:     "gitlab_ci",
			Branch:   os.Getenv("CI_COMMIT_REF_NAME"),
			Commit:   os.Getenv("CI_COMMIT_SHA"),
			PRNumber: os.Getenv("CI_MERGE_REQUEST_IID"),
			IsCI:     true,
		}
	}

	// Jenkins
	if v := os.Getenv("JENKINS_URL"); v != "" {
		return CI{
			Name:   "jenkins",
			Branch: os.Getenv("GIT_BRANCH"),
			Commit: os.Getenv("GIT_COMMIT"),
			IsCI:   true,
		}
	}

	// CircleCI
	if v := os.Getenv("CIRCLECI"); v != "" {
		return CI{
			Name:   "circleci",
			Branch: os.Getenv("CIRCLE_BRANCH"),
			Commit: os.Getenv("CIRCLE_SHA1"),
			IsCI:   true,
		}
	}

	// Travis CI
	if v := os.Getenv("TRAVIS"); v != "" {
		return CI{
			Name:   "travis",
			Branch: os.Getenv("TRAVIS_BRANCH"),
			Commit: os.Getenv("TRAVIS_COMMIT"),
			IsCI:   true,
		}
	}

	// Azure Pipelines
	if v := os.Getenv("AZURE_PIPELINES"); v != "" {
		return CI{
			Name:   "azure_pipelines",
			Branch: os.Getenv("BUILD_SOURCEBRANCHNAME"),
			Commit: os.Getenv("BUILD_SOURCEVERSION"),
			IsCI:   true,
		}
	}

	// Bitbucket Pipelines
	if v := os.Getenv("BITBUCKET_COMMIT"); v != "" {
		return CI{
			Name:   "bitbucket",
			Branch: os.Getenv("BITBUCKET_BRANCH"),
			Commit: os.Getenv("BITBUCKET_COMMIT"),
			IsCI:   true,
		}
	}

	// AWS CodeBuild
	if v := os.Getenv("CODEBUILD_BUILD_ID"); v != "" {
		return CI{
			Name:   "codebuild",
			Branch: os.Getenv("CODEBUILD_WEBHOOK_HEAD_REF"),
			Commit: os.Getenv("CODEBUILD_RESOLVED_SOURCE_VERSION"),
			IsCI:   true,
		}
	}

	return CI{IsCI: false}
}

// AutoConfigure sets environment variables if not already set.
func AutoConfigure() {
	// Auto-detect ANDROID_HOME
	if os.Getenv("ANDROID_HOME") == "" {
		if sdk := DetectANDROIDHOME(); sdk != "" {
			os.Setenv("ANDROID_HOME", sdk)
		}
	}

	// Auto-detect JAVA_HOME
	if os.Getenv("JAVA_HOME") == "" {
		if jdk := DetectJAVAHOME(); jdk != "" {
			os.Setenv("JAVA_HOME", jdk)
		}
	}

	// Auto-detect FLUTTER_ROOT
	if os.Getenv("FLUTTER_ROOT") == "" && os.Getenv("FLUTTER_HOME") == "" {
		if flutter := DetectFLUTTERHOME(); flutter != "" {
			os.Setenv("FLUTTER_ROOT", flutter)
		}
	}
}
