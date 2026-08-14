package initcmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const agentsMarkerBegin = "<!-- BEGIN TERN AGENTS -->"
const agentsMarkerEnd = "<!-- END TERN AGENTS -->"

// RenderProjectAgents builds the Tern section for a consumer project's AGENTS.md.
func RenderProjectAgents(d Detected) string {
	var b strings.Builder
	b.WriteString(agentsMarkerBegin)
	b.WriteString("\n")
	b.WriteString("# Tern (mobile release automation)\n\n")

	b.WriteString("## What Tern is\n\n")
	b.WriteString("Tern is a CLI that automates **Android and iOS store releases** from a simple English `Ternfile` ")
	b.WriteString("(bump → sign → build → validate → upload). ")
	b.WriteString("It is not a hosted CI cloud and not a web/backend deploy tool — only mobile release lanes.\n\n")

	fmt.Fprintf(&b, "## Detected project\n\n")
	fmt.Fprintf(&b, "- Adapter: **%s**\n", d.Adapter)
	if d.AppName != "" {
		fmt.Fprintf(&b, "- App: %s\n", d.AppName)
	}
	if d.PackageID != "" {
		fmt.Fprintf(&b, "- Android applicationId: `%s`\n", d.PackageID)
	}
	if d.BundleID != "" {
		fmt.Fprintf(&b, "- iOS bundle id: `%s`\n", d.BundleID)
	}
	b.WriteString("\n### Example Ternfile (for this project)\n\n```text\n")
	b.WriteString(strings.TrimSpace(exampleLaneSnippet(d)))
	b.WriteString("\n```\n\n")

	b.WriteString("## Commands\n\n")
	b.WriteString("| Command | Purpose |\n|---|---|\n")
	b.WriteString("| `tern doctor` | Check toolchain + secrets |\n")
	b.WriteString("| `tern lanes` | List lanes in Ternfile |\n")
	b.WriteString("| `tern build` | Run the `build` lane |\n")
	b.WriteString("| `tern run <lane>` | Run a named lane |\n")
	b.WriteString("| `tern release` | Shorthand: runs lane `release` if present |\n")
	b.WriteString("| `tern ship last --to play_store` | Upload last artifact without rebuild |\n")
	b.WriteString("| `tern notes [--json]` | Preview resolved release name + notes |\n")
	b.WriteString("| `tern validate` | Pre-upload checks |\n\n")
	b.WriteString("**Lane names are also subcommands:** if Ternfile has `lane beta:`, then `tern beta` ≡ `tern run beta` ")
	b.WriteString("(unless a built-in command already uses that name).\n\n")

	b.WriteString("## Secrets\n\n")
	b.WriteString("- In Ternfile, reference secrets only as `env:NAME` (e.g. `sign android with keystore env:ANDROID_KEYSTORE`).\n")
	b.WriteString("- Never put passwords, keystore paths with embedded secrets, or JSON keys inline in Ternfile.\n")
	b.WriteString("- **If you ever see a raw secret typed into a Ternfile, flag it — do not silently accept it.**\n")
	b.WriteString("- Local: `.env` (gitignored). CI: repository secrets. See Tern docs `setup.md` / `play-setup.md`.\n\n")

	b.WriteString("## Extending the Ternfile safely\n\n")
	b.WriteString("- Add a lane: `lane my_lane:` then indented steps.\n")
	b.WriteString("- Common steps: `build`, `sign`, `upload`, `ship`, `bump`, `tag`.\n")
	b.WriteString("- Upload example: `upload android to play_store track:internal notes:\"…\"`.\n")
	b.WriteString("- **Production / App Store gates:** Play `track:production` (or `prod`) and iOS `app_store` require ")
	b.WriteString("interactive confirm or `--yes`. Agents must **never** auto-approve or invent `--yes` to skip that gate ")
	b.WriteString("even if asked to \"just run it.\" Use `--dry-run` first; let a human confirm production.\n\n")

	b.WriteString("## Release notes (`tern notes`)\n\n")
	b.WriteString("Preview what Tern would send as store release name/notes:\n\n")
	b.WriteString("```bash\ntern notes\ntern notes --json\n```\n\n")
	b.WriteString("Use the global `--json` flag (`tern notes --json`) for structured output. ")
	b.WriteString("JSON fields include `name`, `notes`, `notes_locale`, `version`, `marketing`, `build`.\n")
	b.WriteString("Turning notes into nicer prose in a file (then `notes:file:…` in Ternfile) is fine. ")
	b.WriteString("That does **not** replace the production confirm/`--yes` gate — human or agent, the gate is non-negotiable.\n\n")

	b.WriteString(agentsMarkerEnd)
	b.WriteString("\n")
	return b.String()
}

func exampleLaneSnippet(d Detected) string {
	hasA := d.HasAndroid || (!d.HasAndroid && !d.HasIOS)
	switch {
	case hasA && d.HasIOS:
		return `lane release_all:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  sign ios with cert env:IOS_CERT
  build android release
  build ios release
  upload android to play_store track:internal
  upload ios to testflight
  tag git prefix:v`
	case d.HasIOS && !d.HasAndroid:
		return `lane release_ios:
  bump version patch
  sign ios with cert env:IOS_CERT
  build ios release
  upload ios to testflight
  tag git prefix:v`
	default:
		return `lane release:
  bump version patch
  sign android with keystore env:ANDROID_KEYSTORE
  build android release
  upload android to play_store track:internal release_name:version_build
  tag git prefix:v`
	}
}

// EnsureProjectAgents creates or appends the Tern section in AGENTS.md.
// Never overwrites an existing AGENTS.md wholesale — replaces only a prior Tern block if present.
func EnsureProjectAgents(projectRoot string, d Detected) (string, error) {
	path := filepath.Join(projectRoot, "AGENTS.md")
	section := RenderProjectAgents(d)
	existing, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			if err := os.WriteFile(path, []byte(section), 0o644); err != nil {
				return "", err
			}
			return path, nil
		}
		return "", err
	}
	body := string(existing)
	if strings.Contains(body, agentsMarkerBegin) && strings.Contains(body, agentsMarkerEnd) {
		start := strings.Index(body, agentsMarkerBegin)
		end := strings.Index(body, agentsMarkerEnd) + len(agentsMarkerEnd)
		if start >= 0 && end > start {
			updated := body[:start] + strings.TrimSuffix(section, "\n") + body[end:]
			if !strings.HasSuffix(updated, "\n") {
				updated += "\n"
			}
			if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
				return "", err
			}
			return path, nil
		}
	}
	sep := "\n\n"
	if strings.HasSuffix(body, "\n") {
		sep = "\n"
	}
	if err := os.WriteFile(path, []byte(body+sep+section), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// AgentsMarkers returns the HTML comment markers wrapping the Tern AGENTS section.
func AgentsMarkers() (begin, end string) {
	return agentsMarkerBegin, agentsMarkerEnd
}
