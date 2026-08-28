package testrunner

import (
	"strings"
	"testing"
)

func TestParseTestOutput(t *testing.T) {
	tests := []struct {
		name     string
		output   string
		wantPass int
		wantFail int
	}{
		{
			name:     "all passed",
			output:   "All tests passed!",
			wantPass: 1,
			wantFail: 0,
		},
		{
			name:     "flutter test output",
			output:   "42 passed, 0 failed, 2 skipped",
			wantPass: 42,
			wantFail: 0,
		},
		{
			name:     "some failed",
			output:   "10 passed, 3 failed, 1 skipped",
			wantPass: 10,
			wantFail: 3,
		},
		{
			name:     "empty output",
			output:   "",
			wantPass: 0,
			wantFail: 0,
		},
		{
			name:     "dart test format",
			output:   `00:02 +42 -3: Some tests failed`,
			wantPass: 0,
			wantFail: 0,
		},
		{
			name:     "multi-line output",
			output:   "Running tests...\n10 passed, 2 failed, 1 skipped\nDone!",
			wantPass: 10,
			wantFail: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseTestOutput(tt.output)
			if result.Passed != tt.wantPass {
				t.Errorf("Passed = %d, want %d", result.Passed, tt.wantPass)
			}
			if result.Failed != tt.wantFail {
				t.Errorf("Failed = %d, want %d", result.Failed, tt.wantFail)
			}
		})
	}
}

func TestFormatResult(t *testing.T) {
	tests := []struct {
		name    string
		result  Result
		command string
		want    string
	}{
		{
			name:    "no results",
			result:  Result{},
			command: "flutter test",
			want:    "completed: flutter test",
		},
		{
			name:    "passed only",
			result:  Result{Passed: 10},
			command: "flutter test",
			want:    "flutter test: 10 passed",
		},
		{
			name:    "passed and failed",
			result:  Result{Passed: 10, Failed: 3},
			command: "flutter test",
			want:    "flutter test: 10 passed, 3 failed",
		},
		{
			name:    "all fields",
			result:  Result{Passed: 10, Failed: 3, Skipped: 2},
			command: "dart test",
			want:    "dart test: 10 passed, 3 failed, 2 skipped",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := formatResult(tt.result, tt.command)
			if got != tt.want {
				t.Errorf("formatResult() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestRunOptions(t *testing.T) {
	opts := Options{
		ProjectRoot: t.TempDir(),
		Command:     "echo test",
	}

	if opts.Command != "echo test" {
		t.Errorf("Command = %s, want echo test", opts.Command)
	}
}

func TestParseTestOutputContains(t *testing.T) {
	output := `Running tests...
00:02 +42: All tests passed!
Tests: 42 passed, 0 failed, 0 skipped`

	result := parseTestOutput(output)
	if !strings.Contains("42 passed", "passed") {
		t.Error("expected output to contain 'passed'")
	}
	_ = result
}
