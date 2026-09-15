package output

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"
)

// Mode controls human vs JSON event stream.
type Mode string

const (
	ModeHuman Mode = "human"
	ModeJSON  Mode = "json"
)

// Event is a machine-readable lane step event (agent-ready).
type Event struct {
	Type          string `json:"type"` // step_start | step_end | lane_start | lane_end | doctor | validate | error
	Lane          string `json:"lane,omitempty"`
	Step          string `json:"step,omitempty"`
	Status        string `json:"status,omitempty"` // ok | error | dry_run | skipped
	Message       string `json:"message,omitempty"`
	Hint          string `json:"hint,omitempty"`
	Detail        string `json:"detail,omitempty"` // verbose error chain (cause + stderr)
	DurationMs    int64  `json:"duration_ms,omitempty"`
	ErrorClass    string `json:"error_class,omitempty"`
	ParallelGroup string `json:"parallel_group,omitempty"`
	TS            string `json:"ts"`
}

// Emitter writes human logs and optional JSON lines.
type Emitter struct {
	Mode      Mode
	Out       io.Writer
	Logger    *slog.Logger
	Formatter *HumanFormatter
	doctorBuf []doctorCheck // accumulates doctor checks for grouped output
}

// doctorCheck mirrors the doctor.Check type for display purposes.
type doctorCheck struct {
	Name    string
	OK      bool
	Message string
	Hint    string
}

func New(mode Mode) *Emitter {
	w := os.Stdout
	level := slog.LevelInfo
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return &Emitter{
		Mode:      mode,
		Out:       w,
		Logger:    slog.New(handler),
		Formatter: NewHumanFormatter(os.Stderr),
	}
}

func (e *Emitter) Emit(ev Event) {
	if ev.TS == "" {
		ev.TS = time.Now().UTC().Format(time.RFC3339)
	}
	if e.Mode == ModeJSON {
		_ = json.NewEncoder(e.Out).Encode(ev)
		return
	}
	if e.Formatter == nil {
		e.Formatter = NewHumanFormatter(os.Stderr)
	}
	switch ev.Type {
	case "lane_start":
		e.Formatter.FormatLaneStart(ev.Lane)
	case "lane_end":
		e.Formatter.FormatLaneEnd(ev.Lane, ev.Status, ev.DurationMs)
	case "step_start":
		// Step start is a no-op in human mode; we show result at step_end.
	case "step_end":
		e.Formatter.FormatStepEnd(ev.Step, ev.Status, ev.Message, ev.DurationMs)
	case "doctor":
		// Buffer doctor checks for grouped output.
		e.doctorBuf = append(e.doctorBuf, doctorCheck{
			Name:    extractDoctorName(ev.Message),
			OK:      ev.Status == "ok",
			Message: extractDoctorMessage(ev.Message),
			Hint:    ev.Hint,
		})
	case "error":
		if e.Formatter != nil && e.Formatter.Verbose && ev.Detail != "" {
			e.Formatter.FormatErrorDetail(ev.ErrorClass, ev.Message, ev.Detail)
		} else {
			e.Formatter.FormatError(ev.ErrorClass, ev.Message)
		}
	case "validate":
		e.Formatter.FormatValidate(ev.Status, ev.Message)
	case "ship_start":
		e.Formatter.FormatShipStart(ev.Message)
	case "ship_end":
		e.Formatter.FormatShipEnd(ev.Status, ev.Message, ev.DurationMs)
	case "promote_plan", "promote_start", "promote_end":
		e.Formatter.FormatPromote(ev.Status, ev.Message)
	default:
		e.Formatter.FormatGeneric(ev.Type, ev.Message)
	}
}

// FlushDoctor outputs all buffered doctor checks as a grouped report.
func (e *Emitter) FlushDoctor() {
	if len(e.doctorBuf) > 0 {
		e.Formatter.FormatDoctor(e.doctorBuf)
		e.doctorBuf = nil
	}
}

// extractDoctorName pulls the check name from a "name: message" string.
// Handles "env:ANDROID_KEYSTORE: present" → name="env:ANDROID_KEYSTORE".
func extractDoctorName(msg string) string {
	// Split on ": " (colon-space) to get [name, message].
	name, _, _ := strings.Cut(msg, ": ")
	// For env:VAR entries, include the var name in the display label.
	if prefix, varName, ok := strings.Cut(name, ":"); ok && prefix == "env" {
		return "env:" + varName
	}
	return name
}

// extractDoctorMessage pulls the message portion after "name: ".
func extractDoctorMessage(msg string) string {
	_, message, _ := strings.Cut(msg, ": ")
	// For env:VAR entries, skip past "env:VAR: " to get the actual value.
	if prefix, rest, ok := strings.Cut(message, ": "); ok {
		_ = prefix
		return rest
	}
	return message
}
