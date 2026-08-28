package main

import (
	"github.com/darkmintis/Tern/internal/adapter"
	"github.com/darkmintis/Tern/internal/adapter/flutter"
	"github.com/darkmintis/Tern/internal/adapter/kmp"
	"github.com/darkmintis/Tern/internal/adapter/native"
	"github.com/darkmintis/Tern/internal/adapter/reactnative"
	"github.com/darkmintis/Tern/internal/dotenv"
	execx "github.com/darkmintis/Tern/internal/exec"
	"github.com/darkmintis/Tern/internal/output"
	"github.com/spf13/cobra"
)

type globalFlags struct {
	json     bool
	dryRun   bool
	dir      string
	force    bool
	yes      bool
	clean    bool
	verbose  bool
	parallel bool
}

func newRoot() *cobra.Command {
	g := &globalFlags{}
	reg := defaultRegistry()

	root := &cobra.Command{
		Use:           "tern",
		Short:         "Optimized mobile release engine — build, validate, ship",
		SilenceErrors: true,
		SilenceUsage:  true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			execx.SetVerbose(g.verbose)
			_ = dotenv.LoadProject(g.dir)
		},
	}
	root.PersistentFlags().BoolVar(&g.json, "json", false, "emit machine-readable JSON events")
	root.PersistentFlags().BoolVar(&g.dryRun, "dry-run", false, "print what would run without executing builds/uploads")
	root.PersistentFlags().StringVar(&g.dir, "dir", ".", "project root directory")
	root.PersistentFlags().BoolVar(&g.force, "force", false, "allow upload/ship despite validation failures")
	root.PersistentFlags().BoolVarP(&g.yes, "yes", "y", false, "confirm production uploads without prompting (required in CI)")
	root.PersistentFlags().BoolVar(&g.clean, "clean", false, "run flutter clean before builds")
	root.PersistentFlags().BoolVarP(&g.verbose, "verbose", "v", false, "stream full command logs; print full stderr on failure")
	root.PersistentFlags().BoolVar(&g.parallel, "parallel", false, "run multi-platform builds in parallel (default: sequential)")

	root.AddCommand(cmdVersion())
	root.AddCommand(cmdInit(g, reg))
	root.AddCommand(cmdDoctor(g, reg))
	root.AddCommand(cmdValidate(g))
	root.AddCommand(cmdLanes(g))
	root.AddCommand(cmdRun(g, reg))
	root.AddCommand(cmdBuild(g, reg))
	root.AddCommand(cmdShip(g, reg))
	root.AddCommand(cmdPromote(g))
	root.AddCommand(cmdNotes(g))
	root.AddCommand(cmdCache())
	root.AddCommand(cmdStatus(g))
	root.AddCommand(cmdHistory(g))
	root.AddCommand(cmdArtifacts(g))
	root.AddCommand(cmdRollback(g, reg))
	root.AddCommand(cmdCreate(g))

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

// resolveParallel returns *true when --parallel is set, nil otherwise (sequential default).
func (g *globalFlags) resolveParallel() *bool {
	if g.parallel {
		v := true
		return &v
	}
	return nil
}
