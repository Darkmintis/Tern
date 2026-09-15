package output

import (
	"fmt"
	"io"
	"strings"
	"time"
)

// ANSI color codes for terminal output.
const (
	colorReset  = "\033[0m"
	colorDim    = "\033[2m"
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorCyan   = "\033[36m"
	colorBold   = "\033[1m"
)

// Icons used in human-readable output.
const (
	iconCheck = "\u2713" // ✓
	iconCross = "\u2717" // ✗
	iconArrow = "\u2192" // →
	iconSkip  = "\u2014" // —
)

// HumanFormatter writes styled, developer-friendly output to a terminal.
type HumanFormatter struct {
	w         io.Writer
	startTime time.Time
	Verbose   bool
}

// NewHumanFormatter returns a formatter that writes to w.
func NewHumanFormatter(w io.Writer) *HumanFormatter {
	return &HumanFormatter{w: w, startTime: time.Now()}
}

// SetVerbose enables verbose mode for detailed error output.
func (f *HumanFormatter) SetVerbose(v bool) {
	f.Verbose = v
}

// FormatDoctor prints a full doctor report with grouped checks.
func (f *HumanFormatter) FormatDoctor(checks []doctorCheck) {
	f.println("")
	f.printHeader("Tern Doctor")
	f.println("")

	// Group checks by category.
	var toolchain, signing, secrets, other []doctorCheck
	for _, c := range checks {
		switch {
		case isToolchainCheck(c.Name):
			toolchain = append(toolchain, c)
		case isSigningCheck(c.Name):
			signing = append(signing, c)
		case strings.HasPrefix(c.Name, "env:"):
			secrets = append(secrets, c)
		default:
			other = append(other, c)
		}
	}

	if len(toolchain) > 0 {
		f.printSection("Toolchain")
		for _, c := range toolchain {
			f.printCheck(c)
		}
		f.println("")
	}
	if len(signing) > 0 {
		f.printSection("Signing")
		for _, c := range signing {
			f.printCheck(c)
		}
		f.println("")
	}
	if len(secrets) > 0 {
		f.printSection("Secrets")
		for _, c := range secrets {
			f.printCheck(c)
		}
		f.println("")
	}
	if len(other) > 0 {
		f.printSection("Other")
		for _, c := range other {
			f.printCheck(c)
		}
		f.println("")
	}

	// Summary line.
	allOK := true
	for _, c := range checks {
		if !c.OK {
			allOK = false
			break
		}
	}
	if allOK {
		f.printSuccess(fmt.Sprintf("All %d checks passed", len(checks)))
	} else {
		f.printError("One or more checks failed — fix above issues and re-run: tern doctor")
	}
	f.println("")
}

// FormatLaneStart prints the lane header.
func (f *HumanFormatter) FormatLaneStart(lane string) {
	f.println("")
	f.printHeader(fmt.Sprintf("Lane: %s", lane))
	f.println("")
}

// FormatLaneEnd prints the lane summary.
func (f *HumanFormatter) FormatLaneEnd(lane, status string, durationMs int64) {
	f.println("")
	if status == "ok" {
		f.printSuccess(fmt.Sprintf("Lane %s completed in %s", lane, formatDuration(durationMs)))
	} else {
		f.printError(fmt.Sprintf("Lane %s failed after %s", lane, formatDuration(durationMs)))
	}
	f.println("")
}

// FormatStepStart prints a step being executed.
func (f *HumanFormatter) FormatStepStart(step string) {
	f.printStepPending(step)
}

// FormatStepEnd prints a completed step result.
func (f *HumanFormatter) FormatStepEnd(step, status, message string, durationMs int64) {
	dimMsg := ""
	if message != "" {
		dimMsg = fmt.Sprintf(" %s%s%s", colorDim, message, colorReset)
	}
	dur := ""
	if durationMs > 0 {
		dur = fmt.Sprintf(" %s(%s)%s", colorDim, formatDuration(durationMs), colorReset)
	}

	switch status {
	case "ok":
		f.println(fmt.Sprintf("  %s%s%s %s%s%s%s", colorGreen, iconCheck, colorReset, colorBold, step, colorReset, dimMsg+dur))
	case "dry_run":
		f.println(fmt.Sprintf("  %s%s%s %s%s%s%s%s", colorYellow, iconSkip, colorReset, colorDim, step, colorReset, colorYellow+" (dry run)"+colorReset, dimMsg))
	case "skipped":
		f.println(fmt.Sprintf("  %s%s%s %s%s%s", colorDim, iconSkip, colorReset, step, colorDim, " (skipped)"+colorReset))
	default:
		f.println(fmt.Sprintf("  %s%s%s %s%s%s%s", colorRed, iconCross, colorReset, colorBold, step, colorReset, dimMsg+dur))
	}
}

// FormatError prints an error event.
func (f *HumanFormatter) FormatError(class, message string) {
	f.println(fmt.Sprintf("\n  %s%s ERROR: %s%s", colorRed, iconCross, message, colorReset))
}

// FormatErrorDetail prints a full error with cause and stderr in verbose mode.
func (f *HumanFormatter) FormatErrorDetail(class, message, detail string) {
	f.println(fmt.Sprintf("\n  %s%s ERROR: %s%s", colorRed, iconCross, message, colorReset))
	if detail != "" {
		f.println(fmt.Sprintf("  %s%s%s", colorDim, detail, colorReset))
	}
}

// FormatValidate prints a validation result.
func (f *HumanFormatter) FormatValidate(status, message string) {
	switch status {
	case "ok":
		f.println(fmt.Sprintf("  %s%s%s %s", colorGreen, iconCheck, colorReset, message))
	case "skipped":
		f.println(fmt.Sprintf("  %s%s%s %s (skipped)", colorDim, iconSkip, colorReset, message))
	default:
		f.println(fmt.Sprintf("  %s%s%s %s", colorRed, iconCross, colorReset, message))
	}
}

// FormatGeneric prints a generic event.
func (f *HumanFormatter) FormatGeneric(evType, message string) {
	f.println(fmt.Sprintf("  %s", message))
}

// FormatShipStart prints ship info.
func (f *HumanFormatter) FormatShipStart(message string) {
	f.println(fmt.Sprintf("  %s%s%s %s", colorCyan, iconArrow, colorReset, message))
}

// FormatShipEnd prints ship result.
func (f *HumanFormatter) FormatShipEnd(status, message string, durationMs int64) {
	switch status {
	case "ok":
		f.println(fmt.Sprintf("  %s%s%s %s%s%s", colorGreen, iconCheck, colorReset, message, colorDim, formatDuration(durationMs)))
	default:
		f.println(fmt.Sprintf("  %s%s%s %s", colorRed, iconCross, colorReset, message))
	}
}

// FormatPromote prints promotion info.
func (f *HumanFormatter) FormatPromote(status, message string) {
	switch status {
	case "ok":
		f.println(fmt.Sprintf("  %s%s%s %s", colorGreen, iconCheck, colorReset, message))
	case "dry_run":
		f.println(fmt.Sprintf("  %s%s%s %s%s", colorYellow, iconSkip, colorReset, message, " (dry run)"))
	default:
		f.println(fmt.Sprintf("  %s%s%s %s", colorRed, iconCross, colorReset, message))
	}
}

// --- helpers ---

func (f *HumanFormatter) println(s string) {
	_, _ = fmt.Fprintln(f.w, s)
}

func (f *HumanFormatter) printHeader(s string) {
	_, _ = fmt.Fprintf(f.w, "  %s%s%s\n", colorBold, s, colorReset)
}

func (f *HumanFormatter) printSection(s string) {
	_, _ = fmt.Fprintf(f.w, "  %s%s%s\n", colorDim, s, colorReset)
}

func (f *HumanFormatter) printCheck(c doctorCheck) {
	icon := fmt.Sprintf("%s%s%s", colorGreen, iconCheck, colorReset)
	if !c.OK {
		icon = fmt.Sprintf("%s%s%s", colorRed, iconCross, colorReset)
	}

	name := padRight(c.Name, 30)
	msg := c.Message
	if c.Hint != "" && !c.OK {
		msg += fmt.Sprintf("\n      %s%s%s", colorDim, c.Hint, colorReset)
	}

	_, _ = fmt.Fprintf(f.w, "    %s %s%s%s %s\n", icon, colorBold, name, colorReset, msg)
}

func (f *HumanFormatter) printStepPending(step string) {
	// Just a placeholder; the real output comes in FormatStepEnd.
}

func (f *HumanFormatter) printSuccess(msg string) {
	_, _ = fmt.Fprintf(f.w, "  %s%s%s %s%s%s\n", colorGreen, iconCheck, colorReset, colorGreen, msg, colorReset)
}

func (f *HumanFormatter) printError(msg string) {
	_, _ = fmt.Fprintf(f.w, "  %s%s%s %s%s%s\n", colorRed, iconCross, colorReset, colorRed, msg, colorReset)
}

func isToolchainCheck(name string) bool {
	switch name {
	case "adapter", "flutter", "flutter_doctor", "android_sdk", "jdk", "android_licenses", "xcodebuild", "android_package":
		return true
	}
	return false
}

func isSigningCheck(name string) bool {
	switch name {
	case "android_signing_gradle", "sync_certs":
		return true
	}
	return false
}

func padRight(s string, width int) string {
	if len(s) >= width {
		return s
	}
	return s + strings.Repeat(" ", width-len(s))
}

func formatDuration(ms int64) string {
	d := time.Duration(ms) * time.Millisecond
	if d < time.Second {
		return fmt.Sprintf("%dms", ms)
	}
	return fmt.Sprintf("%.1fs", d.Seconds())
}
