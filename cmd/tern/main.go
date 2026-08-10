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
	"github.com/darkmintis/Tern/internal/config"
	"github.com/darkmintis/Tern/internal/doctor"
	"github.com/darkmintis/Tern/internal/engine"
	ternerrors "github.com/darkmintis/Tern/internal/errors"
	initcmd "github.com/darkmintis/Tern/internal/initcmd"
	"github.com/darkmintis/Tern/internal/output"
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
}

func newRoot() *cobra.Command {
	g := &globalFlags{}
	reg := defaultRegistry()

	root := &cobra.Command{
		Use:           "tern",
		Short:         "Mobile release automation CLI — build, sign, upload without Ruby",
		SilenceErrors: true,
		SilenceUsage:  true,
	}
	root.PersistentFlags().BoolVar(&g.json, "json", false, "emit machine-readable JSON events")
	root.PersistentFlags().BoolVar(&g.dryRun, "dry-run", false, "print what would run without executing builds/uploads")
	root.PersistentFlags().StringVar(&g.dir, "dir", ".", "project root directory")

	root.AddCommand(cmdVersion())
	root.AddCommand(cmdInit(g, reg))
	root.AddCommand(cmdDoctor(g, reg))
	root.AddCommand(cmdLanes(g))
	root.AddCommand(cmdRun(g, reg))
	root.AddCommand(cmdBuild(g, reg))

	return root
}

func defaultRegistry() *adapter.Registry {
	// Flutter is the only active Detect/Build path in v0.
	// Native / KMP / RN packages stay imported as Phase 2–4 scaffolds.
	_ = native.Phase
	_ = kmp.Phase
	_ = reactnative.Phase
	return adapter.NewRegistry(
		flutter.New(nil),
		// Registered for discovery/docs, but Detect() is false until their phase.
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

func runLane(g *globalFlags, reg *adapter.Registry, name string) error {
	cfg, err := config.Load(g.dir)
	if err != nil {
		return err
	}
	eng := engine.New(reg)
	return eng.RunLane(context.Background(), cfg, name, engine.Options{
		ProjectRoot: g.dir,
		DryRun:      g.dryRun,
		Emitter:     emitter(g),
	})
}
