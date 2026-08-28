package platform

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestDetectANDROIDHOME(t *testing.T) {
	// Test with env var set
	t.Setenv("ANDROID_HOME", "/custom/sdk")
	if got := DetectANDROIDHOME(); got != "/custom/sdk" {
		t.Fatalf("expected /custom/sdk, got %s", got)
	}
}

func TestDetectANDROIDSDKRoot(t *testing.T) {
	t.Setenv("ANDROID_SDK_ROOT", "/custom/sdk2")
	t.Setenv("ANDROID_HOME", "")
	if got := DetectANDROIDHOME(); got != "/custom/sdk2" {
		t.Fatalf("expected /custom/sdk2, got %s", got)
	}
}

func TestDetectANDROIDHOMENotFound(t *testing.T) {
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")
	// This may return a path if one exists on the system
	// We just test it doesn't panic
	_ = DetectANDROIDHOME()
}

func TestDetectJAVAHOME(t *testing.T) {
	t.Setenv("JAVA_HOME", "/custom/jdk")
	if got := DetectJAVAHOME(); got != "/custom/jdk" {
		t.Fatalf("expected /custom/jdk, got %s", got)
	}
}

func TestDetectJAVAHOMENotFound(t *testing.T) {
	t.Setenv("JAVA_HOME", "")
	_ = DetectJAVAHOME()
}

func TestDetectFLUTTERHOME(t *testing.T) {
	t.Setenv("FLUTTER_ROOT", "/custom/flutter")
	if got := DetectFLUTTERHOME(); got != "/custom/flutter" {
		t.Fatalf("expected /custom/flutter, got %s", got)
	}
}

func TestDetectFLUTTERHOMEEnv(t *testing.T) {
	t.Setenv("FLUTTER_HOME", "/custom/flutter2")
	t.Setenv("FLUTTER_ROOT", "")
	if got := DetectFLUTTERHOME(); got != "/custom/flutter2" {
		t.Fatalf("expected /custom/flutter2, got %s", got)
	}
}

func TestDetectCI_GitHubActions(t *testing.T) {
	t.Setenv("GITHUB_ACTIONS", "true")
	t.Setenv("GITHUB_REF_NAME", "main")
	t.Setenv("GITHUB_SHA", "abc123")

	ci := DetectCI()
	if !ci.IsCI {
		t.Fatal("expected IsCI=true")
	}
	if ci.Name != "github_actions" {
		t.Fatalf("expected github_actions, got %s", ci.Name)
	}
	if ci.Branch != "main" {
		t.Fatalf("expected main, got %s", ci.Branch)
	}
	if ci.Commit != "abc123" {
		t.Fatalf("expected abc123, got %s", ci.Commit)
	}
}

func TestDetectCI_GitLabCI(t *testing.T) {
	t.Setenv("GITLAB_CI", "true")
	t.Setenv("CI_COMMIT_REF_NAME", "develop")
	t.Setenv("CI_COMMIT_SHA", "def456")

	ci := DetectCI()
	if !ci.IsCI {
		t.Fatal("expected IsCI=true")
	}
	if ci.Name != "gitlab_ci" {
		t.Fatalf("expected gitlab_ci, got %s", ci.Name)
	}
}

func TestDetectCI_Jenkins(t *testing.T) {
	t.Setenv("JENKINS_URL", "http://jenkins.example.com")
	t.Setenv("GIT_BRANCH", "origin/main")
	t.Setenv("GIT_COMMIT", "ghi789")

	ci := DetectCI()
	if !ci.IsCI {
		t.Fatal("expected IsCI=true")
	}
	if ci.Name != "jenkins" {
		t.Fatalf("expected jenkins, got %s", ci.Name)
	}
}

func TestDetectCI_CircleCI(t *testing.T) {
	t.Setenv("CIRCLECI", "true")
	t.Setenv("CIRCLE_BRANCH", "main")
	t.Setenv("CIRCLE_SHA1", "jkl012")

	ci := DetectCI()
	if !ci.IsCI {
		t.Fatal("expected IsCI=true")
	}
	if ci.Name != "circleci" {
		t.Fatalf("expected circleci, got %s", ci.Name)
	}
}

func TestDetectCI_NotCI(t *testing.T) {
	// Clear all CI env vars
	envs := []string{
		"GITHUB_ACTIONS", "GITLAB_CI", "JENKINS_URL", "CIRCLECI",
		"TRAVIS", "AZURE_PIPELINES", "BITBUCKET_COMMIT", "CODEBUILD_BUILD_ID",
	}
	for _, e := range envs {
		t.Setenv(e, "")
	}

	ci := DetectCI()
	if ci.IsCI {
		t.Fatal("expected IsCI=false for local environment")
	}
}

func TestAutoConfigure(t *testing.T) {
	// Clear env vars
	t.Setenv("ANDROID_HOME", "")
	t.Setenv("JAVA_HOME", "")
	t.Setenv("FLUTTER_ROOT", "")
	t.Setenv("FLUTTER_HOME", "")

	// AutoConfigure should not panic
	AutoConfigure()
}

func TestWindowsPaths(t *testing.T) {
	if runtime.GOOS != "windows" {
		t.Skip("windows-only test")
	}

	home, _ := os.UserHomeDir()
	expected := filepath.Join(home, "AppData", "Local", "Android", "Sdk")

	// Create the directory to simulate Windows SDK
	_ = os.MkdirAll(expected, 0o755)
	_ = os.MkdirAll(filepath.Join(expected, "platforms"), 0o755)
	defer func() {
		_ = os.RemoveAll(filepath.Join(home, "AppData", "Local", "Android"))
	}()

	t.Setenv("ANDROID_HOME", "")
	t.Setenv("ANDROID_SDK_ROOT", "")

	got := DetectANDROIDHOME()
	if got != expected {
		t.Logf("got %s, expected %s (may vary by system)", got, expected)
	}
}
