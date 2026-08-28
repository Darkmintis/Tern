package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/artifacts"
	"github.com/darkmintis/Tern/internal/cache"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/doctor"
	"github.com/darkmintis/Tern/internal/engine"
	"github.com/darkmintis/Tern/internal/history"
	initcmd "github.com/darkmintis/Tern/internal/initcmd"
	"github.com/darkmintis/Tern/internal/projectmeta"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/store"
	"github.com/darkmintis/Tern/internal/upload"
	"github.com/darkmintis/Tern/internal/validate"
	"github.com/darkmintis/Tern/internal/version"
	"github.com/spf13/cobra"
)

func cmdVersion() *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print tern version",
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Println(version.Version)
		},
	}
}

func cmdInit(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	var withWorkflow bool
	c := &cobra.Command{
		Use:   "init",
		Short: "Detect project type and scaffold a Ternfile (+ optional GHA workflow)",
		RunE: func(cmd *cobra.Command, args []string) error {
			res, err := initcmd.Run(g.dir, reg, withWorkflow)
			if err != nil {
				return err
			}
			fmt.Println(res.Message)
			for _, f := range res.Created {
				fmt.Println("created:", f)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&withWorkflow, "github-actions", true, "also write .github/workflows/tern-release.yml")
	return c
}

func cmdDoctor(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "doctor",
		Short: "Validate secrets, signing material, and toolchains",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, _ := config.Load(g.dir)
			_, err := doctor.Run(doctor.Options{
				ProjectRoot: g.dir,
				Config:      cfg,
				Registry:    reg,
				Emitter:     emitter(g),
			})
			return err
		},
	}
}

func cmdValidate(g *globalFlags) *cobra.Command {
	var target, artifact, platform string
	c := &cobra.Command{
		Use:   "validate",
		Short: "Pre-release checks before upload/ship (version, artifact, credentials)",
		RunE: func(cmd *cobra.Command, args []string) error {
			_, err := validate.Run(validate.Options{
				ProjectRoot: g.dir,
				Platform:    config.Platform(platform),
				Artifact:    artifact,
				Target:      target,
				Force:       g.force,
				Emitter:     emitter(g),
			})
			return err
		},
	}
	c.Flags().StringVar(&target, "to", "play_store", "upload target: play_store|testflight|app_store")
	c.Flags().StringVar(&artifact, "artifact", "last", "artifact path or 'last'")
	c.Flags().StringVar(&platform, "platform", "android", "android|ios")
	return c
}

func cmdLanes(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "lanes",
		Short: "List lanes in Ternfile",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := config.Load(g.dir)
			if err != nil {
				return err
			}
			names := make([]string, 0, len(cfg.Lanes))
			for n := range cfg.Lanes {
				names = append(names, n)
			}
			sort.Strings(names)
			for _, n := range names {
				fmt.Println(n)
			}
			return nil
		},
	}
}

func cmdRun(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "run [lane]",
		Short: "Run a named lane",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLane(g, reg, args[0])
		},
	}
}

func cmdBuild(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	return &cobra.Command{
		Use:   "build",
		Short: "Run the default build lane",
		RunE: func(cmd *cobra.Command, args []string) error {
			return runLane(g, reg, "build")
		},
	}
}

func cmdShip(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	var to, track, platform, releaseName, notes, notesFile, notesLocale string
	var rollout float64
	c := &cobra.Command{
		Use:   "ship [artifact]",
		Short: "Upload a saved artifact without rebuilding (default: last)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			from := "last"
			if len(args) == 1 {
				from = args[0]
			}
			spec := releasemeta.DefaultSpec()
			if releaseName != "" {
				strategy, custom, err := releasemeta.ParseNameToken(releaseName)
				if err != nil {
					return err
				}
				spec.NameStrategy = strategy
				spec.NameCustom = custom
			}
			if notesFile != "" {
				spec.NotesMode = releasemeta.NotesFile
				spec.NotesFile = notesFile
			} else if notes != "" {
				mode, text, file, err := releasemeta.ParseNotesToken(notes)
				if err != nil {
					return err
				}
				spec.NotesMode = mode
				spec.NotesText = text
				spec.NotesFile = file
			}
			if notesLocale != "" {
				spec.NotesLocale = notesLocale
			}
			if rollout != 0 {
				norm, err := config.NormalizeRollout(rollout)
				if err != nil {
					return err
				}
				rollout = norm
			}
			eng := engine.New(reg)
			return eng.Ship(context.Background(), engine.ShipOptions{
				ProjectRoot: g.dir,
				Platform:    config.Platform(platform),
				From:        from,
				Target:      to,
				Track:       track,
				Rollout:     rollout,
				DryRun:      g.dryRun,
				Force:       g.force,
				Yes:         g.yes,
				ReleaseSpec: spec,
				Emitter:     emitter(g),
			})
		},
	}
	c.Flags().StringVar(&to, "to", "play_store", "play_store|testflight|app_store")
	c.Flags().StringVar(&track, "track", "internal", "Play track (android)")
	c.Flags().Float64Var(&rollout, "rollout", 0, "Play staged rollout: 10 or 0.1 for 10%; 0 = full")
	c.Flags().StringVar(&platform, "platform", "", "android|ios (inferred from --to if empty)")
	c.Flags().StringVar(&releaseName, "release-name", "", "version|version_build|… or custom title")
	c.Flags().StringVar(&notes, "notes", "", "default|none|text, or omit for Bug fixes and improvements.")
	c.Flags().StringVar(&notesFile, "notes-file", "", "path to release notes file")
	c.Flags().StringVar(&notesLocale, "notes-locale", "", "Play notes locale (default en-US)")
	return c
}

func cmdPromote(g *globalFlags) *cobra.Command {
	var rollout float64
	var releaseVersion string
	c := &cobra.Command{
		Use:   "promote <source> <target>",
		Short: "Promote an existing release between tracks (internal, alpha, beta, production) or testflight → appstore",
		Example: `  tern promote internal production
  tern promote closed production
  tern promote testflight appstore`,
		Args: cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			if rollout != 0 {
				norm, err := config.NormalizeRollout(rollout)
				if err != nil {
					return err
				}
				rollout = norm
			}
			client := upload.NewClient()
			return client.Promote(context.Background(), upload.PromoteOpts{
				ProjectRoot:    g.dir,
				Source:         args[0],
				Target:         args[1],
				Rollout:        rollout,
				ReleaseVersion: releaseVersion,
				DryRun:         g.dryRun,
				Yes:            g.yes,
				Emitter:        emitter(g),
			})
		},
	}
	c.Flags().Float64Var(&rollout, "rollout", 0, "staged rollout on the target track: 10 or 0.1 for 10%; 0 = full (only Play)")
	c.Flags().StringVar(&releaseVersion, "release-version", "", "iOS marketing version string for the App Store entry (e.g. 1.2.3)")
	return c
}

func cmdNotes(g *globalFlags) *cobra.Command {
	var releaseName, notes, notesFile, notesLocale string
	c := &cobra.Command{
		Use:   "notes",
		Short: "Preview resolved store release name and notes",
		RunE: func(cmd *cobra.Command, args []string) error {
			spec := releasemeta.DefaultSpec()
			if releaseName != "" {
				strategy, custom, err := releasemeta.ParseNameToken(releaseName)
				if err != nil {
					return err
				}
				spec.NameStrategy = strategy
				spec.NameCustom = custom
			}
			if notesFile != "" {
				spec.NotesMode = releasemeta.NotesFile
				spec.NotesFile = notesFile
			} else if notes != "" {
				mode, text, file, err := releasemeta.ParseNotesToken(notes)
				if err != nil {
					return err
				}
				spec.NotesMode = mode
				spec.NotesText = text
				spec.NotesFile = file
			}
			if notesLocale != "" {
				spec.NotesLocale = notesLocale
			}
			resolved, err := releasemeta.Resolve(g.dir, spec)
			if err != nil {
				return err
			}
			if g.json {
				enc := json.NewEncoder(os.Stdout)
				enc.SetIndent("", "  ")
				return enc.Encode(map[string]string{
					"name":         resolved.Name,
					"notes":        resolved.Notes,
					"notes_locale": resolved.NotesLocale,
					"version":      resolved.Version,
					"marketing":    resolved.Marketing,
					"build":        resolved.Build,
				})
			}
			fmt.Printf("name: %s\n", resolved.Name)
			fmt.Printf("locale: %s\n", resolved.NotesLocale)
			fmt.Printf("version: %s\n", resolved.Version)
			fmt.Printf("notes:\n%s\n", resolved.Notes)
			return nil
		},
	}
	c.Flags().StringVar(&releaseName, "release-name", "", "version|version_build|… or custom title")
	c.Flags().StringVar(&notes, "notes", "", "default|none|literal text")
	c.Flags().StringVar(&notesFile, "notes-file", "", "path to release notes file")
	c.Flags().StringVar(&notesLocale, "notes-locale", "", "notes locale (default en-US)")
	return c
}

func cmdStatus(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show local version vs last releases on each track",
		RunE: func(cmd *cobra.Command, args []string) error {
			local, err := projectmeta.FlutterVersion(g.dir)
			if err != nil {
				local = "unknown"
			}
			parts := strings.SplitN(local, "+", 2)
			versionStr := local
			buildStr := ""
			if len(parts) == 2 {
				versionStr = parts[0]
				buildStr = parts[1]
			}

			h, herr := history.Load(g.dir)
			if herr != nil {
				h = history.History{}
			}

			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintf(tw, "LOCAL\t%s (build %s)\n", versionStr, buildStr)
			fmt.Fprintln(tw)

			tracks := map[string]*history.Record{}
			for i := len(h.Releases) - 1; i >= 0; i-- {
				r := h.Releases[i]
				key := string(r.Platform) + ":" + r.Track
				if _, ok := tracks[key]; !ok {
					tracks[key] = &r
				}
			}

			if len(tracks) == 0 {
				fmt.Fprintln(tw, "TRACKS\tno releases recorded yet")
			} else {
				keys := make([]string, 0, len(tracks))
				for k := range tracks {
					keys = append(keys, k)
				}
				sort.Strings(keys)
				for _, k := range keys {
					r := tracks[k]
					ago := timeSince(r.ReleasedAt)
					fmt.Fprintf(tw, "%s:%s\tv%s+%d\t%s ago\n", r.Platform, r.Track, r.Version, r.Build, ago)
				}
			}
			tw.Flush()
			return nil
		},
	}
}

func timeSince(t time.Time) string {
	d := time.Since(t)
	switch {
	case d < time.Minute:
		return "just now"
	case d < time.Hour:
		return fmt.Sprintf("%dm", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd", int(d.Hours()/24))
	}
}

func cmdHistory(g *globalFlags) *cobra.Command {
	return &cobra.Command{
		Use:   "history",
		Short: "Show release history",
		RunE: func(cmd *cobra.Command, args []string) error {
			h, err := history.Load(g.dir)
			if err != nil {
				return err
			}
			if len(h.Releases) == 0 {
				fmt.Println("no releases recorded yet")
				return nil
			}
			tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
			fmt.Fprintln(tw, "VERSION\tPLATFORM\tTRACK\tRELEASED\tARTIFACT")
			for i := len(h.Releases) - 1; i >= 0; i-- {
				r := h.Releases[i]
				artifact := r.ArtifactPath
				if len(artifact) > 40 {
					artifact = "..." + artifact[len(artifact)-37:]
				}
				fmt.Fprintf(tw, "v%s+%d\t%s\t%s\t%s\t%s\n",
					r.Version, r.Build, r.Platform, r.Track,
					r.ReleasedAt.Format("2006-01-02 15:04"), artifact)
			}
			tw.Flush()
			return nil
		},
	}
}

func cmdArtifacts(g *globalFlags) *cobra.Command {
	var clean bool
	c := &cobra.Command{
		Use:   "artifacts",
		Short: "List saved build artifacts",
		RunE: func(cmd *cobra.Command, args []string) error {
			if clean {
				return cleanArtifacts(g.dir)
			}
			return listArtifacts(g.dir)
		},
	}
	c.Flags().BoolVar(&clean, "clean", false, "remove old artifacts")
	return c
}

func listArtifacts(root string) error {
	dir := artifacts.Dir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("no artifacts saved yet")
			return nil
		}
		return err
	}
	tw := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "PLATFORM\tVERSION\tSIZE\tBUILT\tARTIFACT")
	count := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".json") {
			continue
		}
		data, err := os.ReadFile(dir + "/" + e.Name())
		if err != nil {
			continue
		}
		var rec artifacts.Record
		if err := json.Unmarshal(data, &rec); err != nil {
			continue
		}
		size := formatSize(rec.SizeBytes)
		artifact := rec.Path
		if len(artifact) > 50 {
			artifact = "..." + artifact[len(artifact)-47:]
		}
		version := rec.Version
		if parts := strings.SplitN(version, "+", 2); len(parts) == 2 {
			version = parts[0] + "+" + parts[1]
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n",
			rec.Platform, version, size,
			rec.BuiltAt.Format("2006-01-02 15:04"), artifact)
		count++
	}
	if count == 0 {
		fmt.Println("no artifacts saved yet")
		return nil
	}
	tw.Flush()
	return nil
}

func cleanArtifacts(root string) error {
	dir := artifacts.Dir(root)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			fmt.Println("nothing to clean")
			return nil
		}
		return err
	}
	removed := 0
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		path := dir + "/" + e.Name()
		if strings.HasSuffix(e.Name(), ".json") {
			data, err := os.ReadFile(path)
			if err == nil {
				var rec artifacts.Record
				if err := json.Unmarshal(data, &rec); err == nil {
					os.Remove(rec.Path)
				}
			}
		}
		os.Remove(path)
		removed++
	}
	if removed == 0 {
		fmt.Println("nothing to clean")
	} else {
		fmt.Printf("cleaned %d artifact(s)\n", removed)
	}
	return nil
}

func formatSize(b int64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1fGB", float64(b)/(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1fMB", float64(b)/(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1fKB", float64(b)/(1<<10))
	default:
		return fmt.Sprintf("%dB", b)
	}
}

func cmdCreate(g *globalFlags) *cobra.Command {
	var appName, packageName, teamID, username, platform string
	c := &cobra.Command{
		Use:   "create",
		Short: "Create a new app on Play Store or App Store Connect",
		Example: `  tern create --platform android --app-name "My App" --package com.example.myapp
  tern create --platform ios --app-name "My App" --package com.example.myapp --team-id ABC123`,
		RunE: func(cmd *cobra.Command, args []string) error {
			p := config.Platform(platform)
			if p == "" {
				p = config.PlatformAndroid
			}
			res, err := store.CreateApp(context.Background(), store.CreateOptions{
				Platform:    p,
				ProjectRoot: g.dir,
				AppName:     appName,
				PackageName: packageName,
				TeamID:      teamID,
				Username:    username,
				DryRun:      g.dryRun,
			}, emitter(g))
			if err != nil {
				return err
			}
			fmt.Println(res.Message)
			return nil
		},
	}
	c.Flags().StringVar(&platform, "platform", "android", "android|ios")
	c.Flags().StringVar(&appName, "app-name", "", "app display name")
	c.Flags().StringVar(&packageName, "package", "", "bundle ID (com.example.app)")
	c.Flags().StringVar(&teamID, "team-id", "", "Apple Team ID (iOS only)")
	c.Flags().StringVar(&username, "username", "", "Apple username (iOS only)")
	return c
}

func cmdRollback(g *globalFlags, reg *adapter.Registry) *cobra.Command {
	var to, track, platform string
	var rollout float64
	c := &cobra.Command{
		Use:   "rollback",
		Short: "Re-upload the last built artifact to rollback a release",
		Example: `  tern rollback                     # re-upload last artifact to internal
  tern rollback --track production  # promote last to production
  tern rollback --to v1.2.1         # rollback to specific version`,
		RunE: func(cmd *cobra.Command, args []string) error {
			if rollout != 0 {
				norm, err := config.NormalizeRollout(rollout)
				if err != nil {
					return err
				}
				rollout = norm
			}
			rec, err := history.Last(g.dir)
			if err != nil {
				return err
			}
			if rec == nil {
				return fmt.Errorf("no releases recorded yet; nothing to rollback")
			}
			if to != "" {
				for i := len(history.History{}.Releases) - 1; i >= 0; i-- {
					_ = i
				}
				h, herr := history.Load(g.dir)
				if herr != nil {
					return herr
				}
				found := false
				for i := len(h.Releases) - 1; i >= 0; i-- {
					if h.Releases[i].Version == strings.TrimPrefix(to, "v") {
						rec = &h.Releases[i]
						found = true
						break
					}
				}
				if !found {
					return fmt.Errorf("version %s not found in release history", to)
				}
			}
			if platform == "" {
				platform = string(rec.Platform)
			}
			if track == "" {
				track = rec.Track
			}
			eng := engine.New(reg)
			return eng.Ship(context.Background(), engine.ShipOptions{
				ProjectRoot: g.dir,
				Platform:    config.Platform(platform),
				From:        rec.ArtifactPath,
				Target:      rec.Target,
				Track:       track,
				Rollout:     rollout,
				DryRun:      g.dryRun,
				Force:       g.force,
				Yes:         g.yes,
				ReleaseSpec: releasemeta.DefaultSpec(),
				Emitter:     emitter(g),
			})
		},
	}
	c.Flags().StringVar(&to, "to", "", "rollback to specific version (e.g. v1.2.1)")
	c.Flags().StringVar(&track, "track", "", "target track (default: same as last release)")
	c.Flags().StringVar(&platform, "platform", "", "android|ios (default: same as last release)")
	c.Flags().Float64Var(&rollout, "rollout", 0, "staged rollout on target track: 10 or 0.1 for 10%%; 0 = full")
	return c
}

func cmdCache() *cobra.Command {
	var gha bool
	var out string
	c := &cobra.Command{
		Use:   "cache",
		Short: "Show or emit CI cache config for pub/Gradle/CocoaPods",
		RunE: func(cmd *cobra.Command, args []string) error {
			if !gha {
				fmt.Println(cache.Explain())
				return nil
			}
			msg, err := cache.WriteGHAFragment(out)
			if err != nil {
				return err
			}
			if out == "" || out == "-" {
				fmt.Print(msg)
			} else {
				fmt.Println(msg)
			}
			return nil
		},
	}
	c.Flags().BoolVar(&gha, "github-actions", false, "emit actions/cache YAML fragment")
	c.Flags().StringVarP(&out, "output", "o", "-", "write fragment to file (- for stdout)")
	return c
}

func runLane(g *globalFlags, reg *adapter.Registry, name string) error {
	cfg, err := config.Load(g.dir)
	if err != nil {
		return err
	}
	eng := engine.New(reg)
	return eng.RunLane(context.Background(), cfg, name, engine.Options{
		ProjectRoot: g.dir,
		DryRun:      g.dryRun,
		Force:       g.force,
		Yes:         g.yes,
		Clean:       g.clean,
		Parallel:    g.resolveParallel(),
		Emitter:     emitter(g),
	})
}
