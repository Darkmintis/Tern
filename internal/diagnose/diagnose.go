package diagnose

import (
	"regexp"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

// Finding is a short developer-facing diagnosis.
type Finding struct {
	Problem string
	Hint    string
	Class   ternerrors.Class
}

type rule struct {
	class   ternerrors.Class
	problem string
	hint    string
	match   func(lower string) bool
}

func containsAny(lower string, needles ...string) bool {
	for _, n := range needles {
		if strings.Contains(lower, n) {
			return true
		}
	}
	return false
}

var (
	reJDK17       = regexp.MustCompile(`(?i)(?:class file (?:major )?version 6[1-9]|jdk\s*1[7-9]|java\s*1[7-9].*required|requires jdk\s*1[7-9])`)
	reVersionCode = regexp.MustCompile(`(?i)version\s*code|versioncode`)
)

// rules are ordered: first match wins (more specific first).
var rules = []rule{
	{
		class: ternerrors.ClassBuild, problem: "Android SDK licenses not accepted",
		hint: "run `flutter doctor --android-licenses`, accept all, then retry `tern release`",
		match: func(l string) bool {
			return containsAny(l, "licenses not accepted", "sdkmanager --licenses", "android sdk license", "license acceptance")
		},
	},
	{
		class: ternerrors.ClassBuild, problem: "Android SDK / ANDROID_HOME not configured",
		hint: "set ANDROID_HOME (or ANDROID_SDK_ROOT) to your SDK path and install platform + build-tools",
		match: func(l string) bool {
			return containsAny(l, "android_home", "sdk location not found", "failed to find target with hash string", "install the missing platforms", "cmdline-tools component is missing", "unable to locate android sdk")
		},
	},
	{
		class: ternerrors.ClassBuild, problem: "JDK version incompatible with Android Gradle Plugin",
		hint: "install and use JDK 17 (or the version your AGP requires), then retry the build",
		match: func(l string) bool {
			return reJDK17.MatchString(l) || containsAny(l, "unsupported class file major version", "invalid source release")
		},
	},
	{
		class: ternerrors.ClassSign, problem: "Android keystore password or alias is wrong",
		hint: "check ANDROID_KEYSTORE_PASSWORD, ANDROID_KEY_ALIAS, and ANDROID_KEY_PASSWORD",
		match: func(l string) bool {
			return containsAny(l, "keystore was tempered with", "keystore was tampered with", "password was incorrect", "wrong password", "cannot recover key", "keytool error")
		},
	},
	{
		class: ternerrors.ClassSign, problem: "Android keystore file missing or unreadable",
		hint: "export ANDROID_KEYSTORE to a readable .jks/.keystore path and run `sign android` before release build",
		match: func(l string) bool {
			return containsAny(l, "keystore file not found", "failed to read key", "key.properties", "storefile") && containsAny(l, "not found", "does not exist", "no such file", "failed")
		},
	},
	{
		class: ternerrors.ClassSign, problem: "Release build is not signed",
		hint: "run `sign android with keystore env:ANDROID_KEYSTORE` before `build android release`",
		match: func(l string) bool {
			return containsAny(l, "signingconfig \"release\" is missing", "not signed", "unsigned")
		},
	},
	{
		class: ternerrors.ClassBuild, problem: "Flutter plugin or dependency failed to compile",
		hint: "run `flutter pub get`, check the failing plugin version in pubspec, then rebuild",
		match: func(l string) bool {
			return containsAny(l, "plugin", "pubspec", "package:") && containsAny(l, "compilation failed", "error:", "failed to compile", "resolving dependencies failed", "version solving failed")
		},
	},
	{
		class: ternerrors.ClassBuild, problem: "CocoaPods install failed",
		hint: "run `cd ios && pod install --repo-update`, fix the podspec error, then retry",
		match: func(l string) bool {
			return containsAny(l, "pod install", "cocoapods", "error installing", "podspec")
		},
	},
	{
		class: ternerrors.ClassUpload, problem: "Play Console denied access (permission or package)",
		hint: "grant the service account access to this app in Play Console → Users and permissions, and verify applicationId",
		match: func(l string) bool {
			return containsAny(l, "403", "permission denied", "caller does not have permission", "the caller does not have permission", "access denied")
		},
	},
	{
		class: ternerrors.ClassUpload, problem: "App package not found in Play Console",
		hint: "create the app in Play Console first, or set ANDROID_PACKAGE_NAME to match applicationId",
		match: func(l string) bool {
			return containsAny(l, "package not found", "app not found", "no application was found", "404") && containsAny(l, "package", "application", "app")
		},
	},
	{
		class: ternerrors.ClassUpload, problem: "Play versionCode already used",
		hint: "bump the build number (`bump version build`), rebuild, then upload again",
		match: func(l string) bool {
			return reVersionCode.MatchString(l) && containsAny(l, "already been used", "already exists", "must be higher", "higher than")
		},
	},
	{
		class: ternerrors.ClassUpload, problem: "Network error talking to Google/Apple store APIs",
		hint: "check internet/proxy/VPN, then retry the upload",
		match: func(l string) bool {
			return containsAny(l, "connection refused", "i/o timeout", "tls handshake", "no such host", "temporary failure in name resolution", "dial tcp", "context deadline exceeded", "network is unreachable")
		},
	},
	{
		class: ternerrors.ClassSign, problem: "iOS code signing or provisioning failed",
		hint: "open Xcode, set Team + provisioning for Release, then run `flutter build ipa` manually once",
		match: func(l string) bool {
			return containsAny(l, "no profiles for", "requires a development team", "code signing is required", "provisioning profile", "errsecureresources", "no signing certificate")
		},
	},
	{
		class: ternerrors.ClassUpload, problem: "App Store Connect API authentication failed",
		hint: "check APP_STORE_CONNECT_API_KEY_ID, APP_STORE_CONNECT_API_ISSUER_ID, and APP_STORE_CONNECT_API_KEY_PATH (.p8)",
		match: func(l string) bool {
			return containsAny(l, "unable to authenticate", "invalid api key", "authentication failed", "could not download", "failed to authenticate")
		},
	},
}

// Match classifies tool stderr / API error text into a short problem + hint.
// Returns nil when no known pattern matches.
func Match(text string) *Finding {
	if strings.TrimSpace(text) == "" {
		return nil
	}
	lower := strings.ToLower(text)
	for _, r := range rules {
		if r.match(lower) {
			return &Finding{Problem: r.problem, Hint: r.hint, Class: r.class}
		}
	}
	return nil
}

// Classify returns a Tern error with short problem + hint when a pattern matches;
// otherwise returns nil (caller should keep its generic error).
func Classify(class ternerrors.Class, fallbackMsg, text string, cause error) error {
	if f := Match(text); f != nil {
		c := f.Class
		if c == "" {
			c = class
		}
		return ternerrors.WrapHint(c, f.Problem, f.Hint, cause)
	}
	return nil
}
