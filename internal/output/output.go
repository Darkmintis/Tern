package output

import (
	"encoding/json"
	"io"
	"log/slog"
	"os"
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
	Type       string `json:"type"` // step_start | step_end | lane_start | lane_end | doctor | error
	Lane       string `json:"lane,omitempty"`
	Step       string `json:"step,omitempty"`
	Status     string `json:"status,omitempty"` // ok | error | dry_run
	Message    string `json:"message,omitempty"`
	DurationMs int64  `json:"duration_ms,omitempty"`
	ErrorClass string `json:"error_class,omitempty"`
	TS         string `json:"ts"`
}

// Emitter writes human logs and optional JSON lines.
type Emitter struct {
	Mode   Mode
	Out    io.Writer
	Logger *slog.Logger
}

func New(mode Mode) *Emitter {
	w := os.Stdout
	level := slog.LevelInfo
	handler := slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})
	return &Emitter{
		Mode:   mode,
		Out:    w,
		Logger: slog.New(handler),
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
	switch ev.Type {
	case "lane_start":
		e.Logger.Info("lane start", "lane", ev.Lane)
	case "lane_end":
		e.Logger.Info("lane end", "lane", ev.Lane, "status", ev.Status, "duration_ms", ev.DurationMs)
	case "step_start":
		e.Logger.Info("step start", "step", ev.Step)
	case "step_end":
		e.Logger.Info("step end", "step", ev.Step, "status", ev.Status, "duration_ms", ev.DurationMs, "msg", ev.Message)
	case "doctor":
		e.Logger.Info("doctor", "status", ev.Status, "msg", ev.Message)
	case "error":
		e.Logger.Error("error", "class", ev.ErrorClass, "msg", ev.Message)
	default:
		e.Logger.Info(ev.Type, "msg", ev.Message)
	}
}
