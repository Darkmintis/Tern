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
	envEx := filepath.Join(dir, ".env.example")
	if _, err := os.Stat(envEx); err != nil {
		t.Fatal("expected .env.example")
	}
	gi, _ := os.ReadFile(filepath.Join(dir, ".gitignore"))
	if !strings.Contains(string(gi), ".env") || !strings.Contains(string(gi), ".tern/") {
		t.Fatalf("gitignore missing tern entries: %s", gi)
	}
	if _, err := os.Stat(filepath.Join(dir, "secrets")); err != nil {
		t.Fatal("expected secrets/ dir")
	}
	agents, err := os.ReadFile(filepath.Join(dir, "AGENTS.md"))
	if err != nil {
		t.Fatal("expected AGENTS.md")
	}
	begin, end := AgentsMarkers()
	if !strings.Contains(string(agents), begin) || !strings.Contains(string(agents), end) {
		t.Fatalf("AGENTS.md missing Tern markers: %s", agents)
	}
	if !strings.Contains(string(agents), "env:NAME") {
		t.Fatal("AGENTS.md missing secrets convention")
	}
}

func TestEnsureProjectAgents_AppendsWithoutClobber(t *testing.T) {
	dir := t.TempDir()
	pre := "# Existing agents\n\nDo not delete me.\n"
	_ = os.WriteFile(filepath.Join(dir, "AGENTS.md"), []byte(pre), 0o644)
	d := Detected{Adapter: "flutter", HasAndroid: true, PackageID: "com.x.app"}
	path, err := EnsureProjectAgents(dir, d)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := os.ReadFile(path)
	s := string(data)
	if !strings.Contains(s, "Do not delete me") {
		t.Fatal("clobbered existing AGENTS.md")
	}
	begin, end := AgentsMarkers()
	if !strings.Contains(s, begin) || !strings.Contains(s, end) {
		t.Fatal("missing Tern section")
	}
	// Second call replaces Tern block only.
	d.AppName = "Updated"
	_, err = EnsureProjectAgents(dir, d)
	if err != nil {
		t.Fatal(err)
	}
	data2, _ := os.ReadFile(path)
	s2 := string(data2)
	if strings.Count(s2, begin) != 1 {
		t.Fatalf("duplicate Tern sections: %d", strings.Count(s2, begin))
	}
	if !strings.Contains(s2, "Updated") {
		t.Fatal("expected refreshed Tern section")
	}
	if !strings.Contains(s2, "Do not delete me") {
		t.Fatal("lost preamble on refresh")
	}
}
