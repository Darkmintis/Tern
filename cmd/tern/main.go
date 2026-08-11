package main

import (
	"fmt"
	"os"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	execx "github.com/darkmintis/Tern/internal/exec"
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
		msg := ternerrors.MessageOf(err)
		if msg == "" {
			msg = err.Error()
		}
		fmt.Fprintf(os.Stderr, "error: %s\n", msg)
		if hint := ternerrors.HintOf(err); hint != "" {
			fmt.Fprintf(os.Stderr, "hint: %s\n", hint)
		}
		if log := ternerrors.StderrOf(err); log != "" {
			if execx.Verbose() {
				fmt.Fprintf(os.Stderr, "\n--- full log ---\n%s\n", strings.TrimRight(log, "\n"))
			} else {
				fmt.Fprintf(os.Stderr, "log: re-run with --verbose (or TERN_VERBOSE=1) to see the full command output\n")
			}
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
