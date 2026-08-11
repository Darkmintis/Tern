package initcmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRenderTernfile_AndroidOnly(t *testing.T) {
	body := RenderTernfile(Detected{
		Adapter:    "flutter",
		HasAndroid: true,
		PackageID:  "com.example.app",
		AppName:    "Example",
	})
	if !strings.Contains(body, "com.example.app") {
		t.Fatal("missing package id")
	}
	if !strings.Contains(body, "lane release_prod:") {
		t.Fatal("missing release_prod")
	}
	if strings.Contains(body, "lane release_ios:") {
		t.Fatal("unexpected ios lane")
	}
}

func TestRenderTernfile_Both(t *testing.T) {
	body := RenderTernfile(Detected{Adapter: "flutter", HasAndroid: true, HasIOS: true})
	if !strings.Contains(body, "lane release_all:") {
		t.Fatal("missing release_all")
	}
}

func TestRun_WritesTernfile(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: demo\nversion: 1.0.0+1\n"), 0o644)
	_ = os.MkdirAll(filepath.Join(dir, "android", "app"), 0o755)
	_ = os.WriteFile(filepath.Join(dir, "android", "app", "build.gradle"), []byte(`
android {
  defaultConfig { applicationId "com.demo.app" }
}
`), 0o644)

	res, err := Run(dir, nil, false)
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(res.Ternfile)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(data), "com.demo.app") {
		t.Fatalf("ternfile missing package: %s", data)
	}
}
