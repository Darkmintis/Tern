package store

import (
	"context"
	"testing"

	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/output"
)

func TestCreateApp_DryRun_Android(t *testing.T) {
	em := output.New(output.ModeJSON)
	res, err := CreateApp(context.Background(), CreateOptions{
		Platform:    config.PlatformAndroid,
		ProjectRoot: t.TempDir(),
		AppName:     "Test App",
		PackageName: "com.test.app",
		DryRun:      true,
	}, em)
	if err != nil {
		t.Fatal(err)
	}
	if res.AppID != "com.test.app" {
		t.Fatalf("expected com.test.app, got %s", res.AppID)
	}
}

func TestCreateApp_DryRun_iOS(t *testing.T) {
	em := output.New(output.ModeJSON)
	res, err := CreateApp(context.Background(), CreateOptions{
		Platform:    config.PlatformIOS,
		ProjectRoot: t.TempDir(),
		AppName:     "Test App",
		PackageName: "com.test.app",
		DryRun:      true,
	}, em)
	if err != nil {
		t.Fatal(err)
	}
	if res.AppID != "com.test.app" {
		t.Fatalf("expected com.test.app, got %s", res.AppID)
	}
}

func TestCreateApp_UnsupportedPlatform(t *testing.T) {
	em := output.New(output.ModeJSON)
	_, err := CreateApp(context.Background(), CreateOptions{
		Platform:    config.Platform("windows"),
		ProjectRoot: t.TempDir(),
		DryRun:      true,
	}, em)
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}

func TestVerifyAppExists(t *testing.T) {
	// These are stubs that return true
	exists, err := VerifyAppExists(context.Background(), config.PlatformAndroid, "com.test.app")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
}

func TestVerifyAppExists_iOS(t *testing.T) {
	exists, err := VerifyAppExists(context.Background(), config.PlatformIOS, "com.test.app")
	if err != nil {
		t.Fatal(err)
	}
	if !exists {
		t.Fatal("expected exists=true")
	}
}

func TestVerifyAppExists_Unsupported(t *testing.T) {
	_, err := VerifyAppExists(context.Background(), config.Platform("linux"), "com.test.app")
	if err == nil {
		t.Fatal("expected error for unsupported platform")
	}
}
