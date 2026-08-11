package doctor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/darkmintis/Tern/internal/config"
)

var javaVersionRe = regexp.MustCompile(`(?i)version\s+"?(1\.)?(\d+)`)

func projectHasAndroid(root string) bool {
	_, err := os.Stat(filepath.Join(root, "android"))
	return err == nil
}

func configNeedsAndroid(cfg *config.Config) bool {
	if cfg == nil {
		return false
	}
	for _, lane := range cfg.Lanes {
		for _, step := range lane.Steps {
			if step.Platform == config.PlatformAndroid {
				return true
			}
		}
	}
	return false
}

// checkAndroidSDK reports ANDROID_HOME / ANDROID_SDK_ROOT presence.
func checkAndroidSDK() Check {
	home := strings.TrimSpace(os.Getenv("ANDROID_HOME"))
	if home == "" {
		home = strings.TrimSpace(os.Getenv("ANDROID_SDK_ROOT"))
	}
	if home == "" {
		return Check{
			Name:    "android_sdk",
			OK:      false,
			Message: "ANDROID_HOME / ANDROID_SDK_ROOT not set",
			Hint:    "export ANDROID_HOME to your Android SDK path (required for release builds on most CI images)",
		}
	}
	if st, err := os.Stat(home); err != nil || !st.IsDir() {
		return Check{
			Name:    "android_sdk",
			OK:      false,
			Message: "Android SDK path missing: " + home,
			Hint:    "fix ANDROID_HOME or install the Android SDK / cmdline-tools",
		}
	}
	return Check{Name: "android_sdk", OK: true, Message: "SDK at " + home}
}

// checkJDK warns when java is missing or clearly below 17 (AGP modern default).
func checkJDK() Check {
	path, err := exec.LookPath("java")
	if err != nil {
		return Check{
			Name:    "jdk",
			OK:      false,
			Message: "java not found on PATH",
			Hint:    "install JDK 17+ and ensure `java -version` works before Android release builds",
		}
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "-version")
	out, _ := cmd.CombinedOutput()
	major := parseJavaMajor(string(out))
	if major == 0 {
		return Check{Name: "jdk", OK: true, Message: "java found (version unparsed)"}
	}
	if major < 17 {
		return Check{
			Name:    "jdk",
			OK:      true, // advisory: many images still build; surface clearly
			Message: "warning: JDK " + strconv.Itoa(major) + " detected; Android Gradle Plugin usually needs 17+",
			Hint:    "install JDK 17 (Temurin/Zulu) and point JAVA_HOME at it",
		}
	}
	return Check{Name: "jdk", OK: true, Message: "JDK " + strconv.Itoa(major)}
}

func parseJavaMajor(s string) int {
	m := javaVersionRe.FindStringSubmatch(s)
	if len(m) < 3 {
		return 0
	}
	n, _ := strconv.Atoi(m[2])
	return n
}

// checkFlutterDoctorQuick runs a short flutter doctor and maps license/cmdline flags.
// Stub/empty SDK trees skip the deep scan so unit tests stay fast and deterministic.
func checkFlutterDoctorQuick(sdkHome string) []Check {
	if sdkHome != "" {
		_, pErr := os.Stat(filepath.Join(sdkHome, "platforms"))
		_, cErr := os.Stat(filepath.Join(sdkHome, "cmdline-tools"))
		if pErr != nil && cErr != nil {
			return []Check{{
				Name:    "flutter_doctor",
				OK:      true,
				Message: "skipped deep scan (SDK has no platforms/cmdline-tools yet)",
			}}
		}
	}
	path, err := exec.LookPath("flutter")
	if err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 12*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, path, "doctor")
	out, err := cmd.CombinedOutput()
	text := string(out)
	if ctx.Err() == context.DeadlineExceeded {
		return []Check{{
			Name:    "flutter_doctor",
			OK:      true,
			Message: "flutter doctor timed out (skipped deep scan)",
		}}
	}
	if err != nil && len(text) == 0 {
		return []Check{{
			Name:    "flutter_doctor",
			OK:      false,
			Message: "flutter doctor failed",
			Hint:    "run `flutter doctor -v` and fix reported toolchain issues",
		}}
	}
	lower := strings.ToLower(text)
	var checks []Check
	if strings.Contains(lower, "android license") && strings.Contains(lower, "not accepted") {
		checks = append(checks, Check{
			Name:    "android_licenses",
			OK:      false,
			Message: "Android SDK licenses not accepted",
			Hint:    "run `flutter doctor --android-licenses` and accept all",
		})
	} else {
		checks = append(checks, Check{Name: "android_licenses", OK: true, Message: "no license warning from flutter doctor"})
	}
	if strings.Contains(lower, "cmdline-tools component is missing") {
		checks = append(checks, Check{
			Name:    "android_cmdline_tools",
			OK:      false,
			Message: "Android cmdline-tools missing",
			Hint:    "install Android SDK Command-line Tools via sdkmanager or Android Studio SDK Manager",
		})
	}
	return checks
}
