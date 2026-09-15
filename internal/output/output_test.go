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
	return &Emitter{Mode: ModeHuman, Logger: logger, Formatter: NewHumanFormatter(&buf)}, &buf
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
		{Event{Type: "lane_start", Lane: "x"}, "Lane"},
		{Event{Type: "lane_end", Lane: "x", Status: "ok"}, "completed"},
		{Event{Type: "step_end", Lane: "x", Step: "build", Status: "ok"}, "build"},
		{Event{Type: "error", ErrorClass: "ExecError", Message: "boom"}, "ERROR"},
		{Event{Type: "custom", Message: "hello"}, "hello"},
		{Event{Type: "validate", Status: "ok", Message: "v"}, "v"},
		{Event{Type: "ship_start", Message: "uploading"}, "uploading"},
		{Event{Type: "promote_plan", Status: "dry_run", Message: "promote"}, "promote"},
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

func TestFlushDoctor(t *testing.T) {
	e, buf := humanEmitter()
	e.Emit(Event{Type: "doctor", Message: "env:ANDROID_KEYSTORE: present"})
	e.Emit(Event{Type: "doctor", Message: "flutter: installed"})
	if len(e.doctorBuf) != 2 {
		t.Fatalf("expected 2 buffered, got %d", len(e.doctorBuf))
	}
	e.FlushDoctor()
	if len(e.doctorBuf) != 0 {
		t.Fatal("buffer should be empty after flush")
	}
	out := buf.String()
	if !strings.Contains(out, "ANDROID_KEYSTORE") {
		t.Fatalf("output missing check: %s", out)
	}
}

func TestFlushDoctorEmpty(t *testing.T) {
	e, _ := humanEmitter()
	e.FlushDoctor() // should not panic
}

func TestFormatDuration(t *testing.T) {
	tests := []struct {
		ms   int64
		want string
	}{
		{0, "0ms"},
		{500, "500ms"},
		{999, "999ms"},
		{1000, "1.0s"},
		{1500, "1.5s"},
		{2345, "2.3s"},
	}
	for _, tc := range tests {
		got := formatDuration(tc.ms)
		if got != tc.want {
			t.Errorf("formatDuration(%d) = %q, want %q", tc.ms, got, tc.want)
		}
	}
}
