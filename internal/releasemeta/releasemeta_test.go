package releasemeta_test

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/darkmintis/Tern/internal/releasemeta"
)

func TestResolveNameStrategies(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: cool_app\nversion: 1.2.3+9\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	cases := []struct {
		strategy releasemeta.NameStrategy
		want  string
	}{
		{releasemeta.NameVersion, "1.2.3"},
		{releasemeta.NameVersionBuild, "1.2.3 (9)"},
		{releasemeta.NameVersionCode, "9"},
		{releasemeta.NameSemverPlus, "1.2.3+9"},
		{releasemeta.NameNameVersion, "Cool app 1.2.3"},
		{releasemeta.NameNone, ""},
	}
	for _, tc := range cases {
		t.Run(string(tc.strategy), func(t *testing.T) {
			got, err := releasemeta.Resolve(dir, releasemeta.Spec{
				NameStrategy: tc.strategy,
				NotesMode:    releasemeta.NotesDefault,
			})
			if err != nil {
				t.Fatal(err)
			}
			if got.Name != tc.want {
				t.Fatalf("got %q want %q", got.Name, tc.want)
			}
			if got.Notes != releasemeta.DefaultNotes {
				t.Fatalf("notes %q", got.Notes)
			}
		})
	}
}

func TestResolveCustomAndFileNotes(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "pubspec.yaml"), []byte("name: x\nversion: 2.0.0+1\n"), 0o644)
	notesPath := filepath.Join(dir, "NOTES.md")
	_ = os.WriteFile(notesPath, []byte("  Fixed crash on launch.\n"), 0o644)

	got, err := releasemeta.Resolve(dir, releasemeta.Spec{
		NameStrategy: releasemeta.NameCustom,
		NameCustom:   "Hotfix March",
		NotesMode:    releasemeta.NotesFile,
		NotesFile:    "NOTES.md",
	})
	if err != nil {
		t.Fatal(err)
	}
	if got.Name != "Hotfix March" || got.Notes != "Fixed crash on launch." {
		t.Fatalf("%+v", got)
	}
}

func TestParseTokens(t *testing.T) {
	s, c, err := releasemeta.ParseNameToken("version_build")
	if err != nil || s != releasemeta.NameVersionBuild || c != "" {
		t.Fatalf("%v %q %v", s, c, err)
	}
	s, c, err = releasemeta.ParseNameToken("Hotfix")
	if err != nil || s != releasemeta.NameCustom || c != "Hotfix" {
		t.Fatalf("%v %q %v", s, c, err)
	}
	m, text, file, err := releasemeta.ParseNotesToken("file:notes.txt")
	if err != nil || m != releasemeta.NotesFile || file != "notes.txt" {
		t.Fatalf("%v %q %q %v", m, text, file, err)
	}
}
