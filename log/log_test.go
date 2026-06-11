package log

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"
)

func TestTextOutputCarriesCallerAndMessage(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)

	l.outputf(LevelInfo, 2, "hello %s", "world")

	out := buf.String()
	if !strings.Contains(out, "hello world") {
		t.Fatalf("message missing: %q", out)
	}
	if !strings.Contains(out, "log_test.go") {
		t.Fatalf("caller file missing: %q", out)
	}
}

func TestLevelFilters(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)
	l.SetLevel(LevelWarn)

	l.outputf(LevelInfo, 2, "dropped")
	if buf.Len() != 0 {
		t.Fatalf("info passed a warn filter: %q", buf.String())
	}
	l.outputf(LevelError, 2, "kept")
	if !strings.Contains(buf.String(), "kept") {
		t.Fatalf("error filtered out: %q", buf.String())
	}
}

func TestOutputwAppendsPairsInTextMode(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)

	l.outputw(LevelInfo, 2, "saved", "ref", "ab12", "rows", 3)

	out := buf.String()
	for _, want := range []string{"saved", "ref=", "ab12", "rows=", "3"} {
		if !strings.Contains(out, want) {
			t.Fatalf("%q missing from %q", want, out)
		}
	}
}

func TestOutputwOddPairGetsPlaceholder(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)

	l.outputw(LevelInfo, 2, "msg", "lonely")
	if !strings.Contains(buf.String(), "lonely=<missing>") {
		t.Fatalf("odd key not marked: %q", buf.String())
	}
}

func TestJSONModeEmitsValidRecords(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)
	l.SetJSON(true)

	l.outputw(LevelWarn, 2, "disk almost full", "free_mb", 12)

	var rec map[string]any
	err := json.Unmarshal(buf.Bytes(), &rec)
	if err != nil {
		t.Fatalf("output is not JSON: %v: %q", err, buf.String())
	}
	if rec["msg"] != "disk almost full" {
		t.Fatalf("msg = %v", rec["msg"])
	}
	if rec["level"] != "WARN" {
		t.Fatalf("level = %v", rec["level"])
	}
	if rec["free_mb"] != float64(12) {
		t.Fatalf("free_mb = %v", rec["free_mb"])
	}
	src, ok := rec["source"].(map[string]any)
	if !ok || !strings.Contains(src["file"].(string), "log_test.go") {
		t.Fatalf("source missing or wrong: %v", rec["source"])
	}
}

func TestJSONModeAppliesToPrintfStyleToo(t *testing.T) {
	var buf bytes.Buffer
	l := newConfigured(&buf)
	l.SetJSON(true)

	l.outputf(LevelInfo, 2, "user %s logged in", "alice")

	var rec map[string]any
	err := json.Unmarshal(buf.Bytes(), &rec)
	if err != nil {
		t.Fatalf("printf output is not JSON: %v: %q", err, buf.String())
	}
	if rec["msg"] != "user alice logged in" {
		t.Fatalf("msg = %v", rec["msg"])
	}
}

func TestSetOutputRebuildsJSONHandler(t *testing.T) {
	var first, second bytes.Buffer
	l := newConfigured(&first)
	l.SetJSON(true)
	l.SetOutput(&second)

	l.outputf(LevelInfo, 2, "after switch")
	if first.Len() != 0 {
		t.Fatalf("old writer still receiving: %q", first.String())
	}
	if !strings.Contains(second.String(), "after switch") {
		t.Fatalf("new writer empty: %q", second.String())
	}
}
