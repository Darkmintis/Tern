package main

import (
	"context"
	"fmt"
	"os"
	"sort"
	"strings"

	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/adapter/kmp"
	"github.com/darkmintis/Tern/internal/adapter/native"
	"github.com/darkmintis/Tern/internal/adapter/reactnative"
	"github.com/darkmintis/Tern/internal/cache"
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/doctor"
	"github.com/darkmintis/Tern/internal/engine"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	initcmd "github.com/darkmintis/Tern/internal/initcmd"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/darkmintis/Tern/internal/releasemeta"
	"github.com/darkmintis/Tern/internal/validate"
	"github.com/darkmintis/Tern/internal/version"
	"github.com/spf13/cobra"
)

func main() {
	root := newRoot()
	// Lane shorthand: `tern <lane>` → `tern run <lane>` when not a known command.
	if rewritten := rewriteLaneShorthand(root, os.Args); rewritten != nil {
		os.Args = rewritten
	}
	if err := root.Execute(); err != nil {
		code := ternerrors.ExitCode(err)
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		if hint := ternerrors.HintOf(err); hint != "" {
			fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
		}
		os.Exit(code)
	}
}

func rewriteLaneShorthand(root *cobra.Command, args []string) []string {
	if len(args) < 2 {
		return nil
	}
	first := args[1]
	if strings.HasPrefix(first, "-") {
		return nil
	}
	cmd, _, err := root.Find([]string{first})
	if err == nil && cmd != root && cmd.Name() == first {
		return nil // known subcommand
	}
	// Treat as lane name.
	out := []string{args[0], "run", first}
	out = append(out, args[2:]...)
	return out
}

type globalFlags struct {
	json   bool
	dryRun bool
	dir    string
	force  bool
	clean  bool
}

func newRoot() *cobra.Command {
	g := &globalFlags{}
	reg := defaultRegistry()

	root := &cobra.Command{
		Use:           "tern",
		Short:         "Optimized mobile release engine — build, validate, ship",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().BoolVar(&g.json, "json", false, "emit machine-readable JSON events")
	root.PersistentFlags().BoolVar(&g.dryRun, "dry-run", false, "print what would run without executing builds/uploads")
	root.PersistentFlags().StringVar(&g.dir, "dir", ".", "project root directory")
	root.PersistentFlags().BoolVar(&g.force, "force", false, "allow upload/ship despite validation failures")
	root.PersistentFlags().BoolVar(&g.clean, "clean", false, "run flutter clean before builds")

	root.AddCommand(cmdVersion())
	root.AddCommand(cmdInit(g, reg))
	root.AddCommand(cmdDoctor(g, reg))
	root.AddCommand(cmdValidate(g))
	root.AddCommand(cmdLanes(g))
	root.AddCommand(cmdRun(g, reg))
	root.AddCommand(cmdBuild(g, reg))
	root.AddCommand(cmdShip(g, reg))
	root.AddCommand(cmdCache())

	return root
}

func defaultRegistry() *adapter.Registry {
	_ = native.Phase
	_ = kmp.Phase
	_ = reactnative.Phase
	return adapter.NewRegistry(
		flutter.New(nil),
		native.New(nil),
		kmp.New(nil),
		reactnative.New(nil),
	)
}

func emitter(g *globalFlags) *output.Emitter {
	mode := output.ModeHuman
	if g.json {
		mode = output.ModeJSON
	}
	return output.New(mode)
}

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
			eng := engine.New(reg)
			return eng.Ship(context.Background(), engine.ShipOptions{
				ProjectRoot: g.dir,
				Platform:    config.Platform(platform),
				From:        from,
				Target:      to,
				Track:       track,
				DryRun:      g.dryRun,
				Force:       g.force,
				ReleaseSpec: spec,
				Emitter:     emitter(g),
			})
		},
	}
	c.Flags().StringVar(&to, "to", "play_store", "play_store|testflight|app_store")
	c.Flags().StringVar(&track, "track", "internal", "Play track (android)")
	c.Flags().StringVar(&platform, "platform", "", "android|ios (inferred from --to if empty)")
	c.Flags().StringVar(&releaseName, "release-name", "", "version|version_build|… or custom title")
	c.Flags().StringVar(&notes, "notes", "", "default|none|text, or omit for Bug fixes and improvements.")
	c.Flags().StringVar(&notesFile, "notes-file", "", "path to release notes file")
	c.Flags().StringVar(&notesLocale, "notes-locale", "", "Play notes locale (default en-US)")
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
		Clean:       g.clean,
		Emitter:     emitter(g),
	})
}
