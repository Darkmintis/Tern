package ternerrors

import (
	"errors"
	"fmt"
	"strings"
)

// Class identifies a stable error category for CLI exit codes and JSON output.
type Class string

const (
	ClassConfig Class = "ConfigError"
	ClassDoctor Class = "DoctorError"
	ClassBuild  Class = "BuildError"
	ClassSign   Class = "SignError"
	ClassUpload Class = "UploadError"
	ClassExec   Class = "ExecError"
)

// Error is a classified Tern error with optional user-facing hint.
type Error struct {
	Class   Class
	Message string
	Hint    string
	Err     error
	// Stderr is a truncated command stderr tail (optional).
	Stderr string
}

func (e *Error) Error() string {
	if e.Err != nil {
		return fmt.Sprintf("%s: %s: %v", e.Class, e.Message, e.Err)
	}
	return fmt.Sprintf("%s: %s", e.Class, e.Message)
}

func (e *Error) Unwrap() error { return e.Err }

func Wrap(class Class, msg string, err error) error {
	return &Error{Class: class, Message: msg, Err: err, Stderr: StderrOf(err)}
}

func WrapHint(class Class, msg, hint string, err error) error {
	return &Error{Class: class, Message: msg, Hint: hint, Err: err, Stderr: StderrOf(err)}
}

func WrapStderr(class Class, msg, stderr string, err error) error {
	return &Error{Class: class, Message: msg, Err: err, Stderr: TruncateStderr(stderr)}
}

func New(class Class, msg string) error {
	return &Error{Class: class, Message: msg}
}

func NewHint(class Class, msg, hint string) error {
	return &Error{Class: class, Message: msg, Hint: hint}
}

func AsClass(err error) (Class, bool) {
	var te *Error
	if errors.As(err, &te) {
		return te.Class, true
	}
	return "", false
}

// HintOf returns a remediation hint if present.
func HintOf(err error) string {
	var te *Error
	if errors.As(err, &te) {
		return te.Hint
	}
	return ""
}

// MessageOf returns the short Tern message without class prefix.
func MessageOf(err error) string {
	var te *Error
	if errors.As(err, &te) {
		return te.Message
	}
	if err == nil {
		return ""
	}
	return err.Error()
}

// StderrOf returns captured command stderr if present.
func StderrOf(err error) string {
	var te *Error
	if errors.As(err, &te) {
		return te.Stderr
	}
	return ""
}

// DetailOf returns the full error chain for verbose/debug output.
// Format: "message | hint | cause | stderr" (omits empty sections).
func DetailOf(err error) string {
	var te *Error
	if !errors.As(err, &te) {
		if err == nil {
			return ""
		}
		return err.Error()
	}
	var parts []string
	if te.Message != "" {
		parts = append(parts, te.Message)
	}
	if te.Hint != "" {
		parts = append(parts, "hint: "+te.Hint)
	}
	if te.Err != nil {
		parts = append(parts, "cause: "+te.Err.Error())
	}
	if te.Stderr != "" {
		// Show last 500 chars of stderr for context.
		stderr := te.Stderr
		if len(stderr) > 500 {
			stderr = "..." + stderr[len(stderr)-500:]
		}
		parts = append(parts, "stderr:\n"+stderr)
	}
	return strings.Join(parts, "\n  ")
}

const maxStderr = 16 * 1024

// TruncateStderr keeps the last maxStderr bytes for diagnosis.
func TruncateStderr(s string) string {
	if len(s) <= maxStderr {
		return s
	}
	return s[len(s)-maxStderr:]
}

// ExitCode maps error class to process exit code.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	class, ok := AsClass(err)
	if !ok {
		return 1
	}
	switch class {
	case ClassConfig:
		return 2
	case ClassDoctor:
		return 3
	case ClassBuild:
		return 4
	case ClassSign:
		return 5
	case ClassUpload:
		return 6
	case ClassExec:
		return 7
	default:
		return 1
	}
}
