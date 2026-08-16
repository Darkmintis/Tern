package output

import (
	"bytes"
	"encoding/json"
	"log/slog"
	"strings"
	"testing"
	"time"
)

func jsonEmitter() (*Emitter, *bytes.Buffer) {
	var buf bytes.Buffer
	return &Emitter{Mode: ModeJSON, Out: &buf}, &buf
}

func humanEmitter() (*Emitter, *bytes.Buffer) {
	var buf bytes.Buffer
	logger := slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelInfo}))
	return &Emitter{Mode: ModeHuman, Logger: logger}, &buf
}

func TestJSONEncodesAllFields(t *testing.T) {
	e, buf := jsonEmitter()
	e.Emit(Event{
		Type: "step_end", Lane: "release", Step: "build android release",
		Status: "ok", Message: "built app.aab", Hint: "",
		DurationMs: 42, ErrorClass: "BuildError", ParallelGroup: "build",
		TS: "2026-08-16T00:00:00Z",
	})
	var got Event
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatalf("invalid json: %v", err)
	}
	if got.Type != "step_end" || got.Lane != "release" || got.Step != "build android release" ||
		got.Status != "ok" || got.Message != "built app.aab" ||
		got.DurationMs != 42 || got.ErrorClass != "BuildError" || got.ParallelGroup != "build" ||
		got.TS != "2026-08-16T00:00:00Z" {
		t.Fatalf("fields mismatch: %+v", got)
	}
}

func TestEmitSetsTimestamp(t *testing.T) {
	e, buf := jsonEmitter()
	e.Emit(Event{Type: "lane_start", Lane: "a"})
	var got Event
	if err := json.Unmarshal(buf.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.TS == "" {
		t.Fatal("expected auto timestamp")
	}
	if _, err := time.Parse(time.RFC3339, got.TS); err != nil {
		t.Fatalf("bad timestamp %q: %v", got.TS, err)
	}
}

func TestHumanEventTypes(t *testing.T) {
	e, buf := humanEmitter()
	for _, tc := range []struct {
		ev   Event
		want string
	}{
		{Event{Type: "lane_start", Lane: "x"}, "lane start"},
		{Event{Type: "lane_end", Lane: "x", Status: "ok"}, "lane end"},
		{Event{Type: "step_start", Lane: "x", Step: "build"}, "step start"},
		{Event{Type: "step_end", Lane: "x", Step: "build", Status: "ok"}, "step end"},
		{Event{Type: "doctor", Status: "ok"}, "doctor"},
		{Event{Type: "validate", Status: "ok", Message: "v"}, "validate"},
		{Event{Type: "error", ErrorClass: "ExecError", Message: "boom"}, "ERROR"},
		{Event{Type: "custom", Message: "hello"}, "custom"},
	} {
		buf.Reset()
		e.Emit(tc.ev)
		if !strings.Contains(buf.String(), tc.want) {
			t.Fatalf("type %q: log %q missing %q", tc.ev.Type, buf.String(), tc.want)
		}
	}
}

func TestEmptyEvent(t *testing.T) {
	e, buf := jsonEmitter()
	e.Emit(Event{}) // empty event must not panic
	if buf.Len() == 0 {
		t.Fatal("expected output")
	}
}

func TestNewDefaults(t *testing.T) {
	e := New(ModeJSON)
	if e.Mode != ModeJSON {
		t.Fatal("mode not set")
	}
	if e.Out == nil || e.Logger == nil {
		t.Fatal("defaults must be wired")
	}
}
