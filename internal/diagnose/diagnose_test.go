package diagnose_test

import (
	"testing"

	"github.com/darkmintis/Tern/internal/diagnose"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
)

func TestMatchCatalog(t *testing.T) {
	cases := []struct {
		name    string
		stderr  string
		problem string
		class   ternerrors.Class
	}{
		{"licenses", "You have not accepted the license agreements of the following SDK components:\nlicenses not accepted", "Android SDK licenses not accepted", ternerrors.ClassBuild},
		{"sdk", "SDK location not found. Define ANDROID_HOME", "Android SDK / ANDROID_HOME not configured", ternerrors.ClassBuild},
		{"jdk", "Unsupported class file major version 61", "JDK version incompatible with Android Gradle Plugin", ternerrors.ClassBuild},
		{"keystore_pw", "Keystore was tampered with, or password was incorrect", "Android keystore password or alias is wrong", ternerrors.ClassSign},
		{"unsigned", "SigningConfig \"release\" is missing required property", "Release build is not signed", ternerrors.ClassSign},
		{"plugin", "Error: Error when reading 'pubspec.yaml': package:foo failed to compile", "Flutter plugin or dependency failed to compile", ternerrors.ClassBuild},
		{"pods", "Error output from CocoaPods:\nError installing FirebaseCore\npod install failed", "CocoaPods install failed", ternerrors.ClassBuild},
		{"play403", "googleapi: Error 403: The caller does not have permission", "Play Console denied access (permission or package)", ternerrors.ClassUpload},
		{"pkg", "googleapi: Error 404: Package not found: com.example.app.", "App package not found in Play Console", ternerrors.ClassUpload},
		{"vc", "APK specifies a version code that has already been used", "Play versionCode already used", ternerrors.ClassUpload},
		{"net", "Post \"https://androidpublisher.googleapis.com\": dial tcp: i/o timeout", "Network error talking to Google/Apple store APIs", ternerrors.ClassUpload},
		{"ios", "No profiles for 'com.example.app' were found", "iOS code signing or provisioning failed", ternerrors.ClassSign},
		{"asc", "Unable to authenticate with App Store Connect API key", "App Store Connect API authentication failed", ternerrors.ClassUpload},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := diagnose.Match(tc.stderr)
			if f == nil {
				t.Fatal("expected match")
			}
			if f.Problem != tc.problem || f.Class != tc.class || f.Hint == "" {
				t.Fatalf("%+v", f)
			}
		})
	}
}

func TestMatchNoHit(t *testing.T) {
	if diagnose.Match("something totally unrelated xyz") != nil {
		t.Fatal("expected nil")
	}
}
