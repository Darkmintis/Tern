package testrunner

import (
	"context"
	"fmt"
	"os/exec"
	"strings"

	ternerrors "github.com/darkmintis/Tern/internal/errors"
	"github.com/darkmintis/Tern/internal/output"
)

// Options for running tests.
type Options struct {
	ProjectRoot string
	Command     string // custom test command, empty = "flutter test"
	Platform    string // android|ios|all
	Verbose     bool
}

// Result of a test run.
type Result struct {
	Passed  int
	Failed  int
	Skipped int
	Message string
}

// Run executes tests and returns results.
func Run(ctx context.Context, opts Options, em *output.Emitter) (Result, error) {
	if em == nil {
		em = output.New(output.ModeHuman)
	}

	command := opts.Command
	if command == "" {
		command = "flutter test"
	}

	em.Emit(output.Event{Type: "test_start", Message: fmt.Sprintf("running: %s", command)})

	parts := strings.Fields(command)
	cmd := exec.CommandContext(ctx, parts[0], parts[1:]...)
	cmd.Dir = opts.ProjectRoot

	var stdout, stderr strings.Builder
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	testOutput := stdout.String() + stderr.String()

	result := parseTestOutput(testOutput)
	result.Message = formatResult(result, command)

	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			if exitErr.ExitCode() == 1 {
				em.Emit(output.Event{Type: "test_end", Status: "failed", Message: result.Message})
				return result, ternerrors.NewHint(ternerrors.ClassBuild, "tests failed",
					"fix failing tests before releasing")
			}
		}
		em.Emit(output.Event{Type: "test_end", Status: "error", Message: err.Error()})
		return result, ternerrors.Wrap(ternerrors.ClassBuild, "test execution failed", err)
	}

	em.Emit(output.Event{Type: "test_end", Status: "ok", Message: result.Message})
	return result, nil
}

// parseTestOutput parses flutter test / dart test output.
func parseTestOutput(testOutput string) Result {
	result := Result{}
	lines := strings.Split(testOutput, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		// Flutter test output: "X tests passed, Y failed, Z skipped"
		if strings.Contains(line, "tests passed") || strings.Contains(line, "All tests passed") {
			fmt.Sscanf(line, "%d passed", &result.Passed)
		}
		if strings.Contains(line, "failed") {
			fmt.Sscanf(line, "%d failed", &result.Failed)
		}
		if strings.Contains(line, "skipped") {
			fmt.Sscanf(line, "%d skipped", &result.Skipped)
		}
	}
	return result
}

// formatResult creates a human-readable result message.
func formatResult(result Result, command string) string {
	if result.Passed == 0 && result.Failed == 0 {
		return fmt.Sprintf("completed: %s", command)
	}
	parts := []string{}
	if result.Passed > 0 {
		parts = append(parts, fmt.Sprintf("%d passed", result.Passed))
	}
	if result.Failed > 0 {
		parts = append(parts, fmt.Sprintf("%d failed", result.Failed))
	}
	if result.Skipped > 0 {
		parts = append(parts, fmt.Sprintf("%d skipped", result.Skipped))
	}
	return fmt.Sprintf("%s: %s", command, strings.Join(parts, ", "))
}
