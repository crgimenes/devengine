package log

import (
	"io"
	"testing"
)

func BenchmarkPrintf(b *testing.B) {
	l := newConfigured(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		l.outputf(LevelInfo, 3, "user %s did %s rev %d", "alice", "save", 42)
	}
}

func BenchmarkOutputwText(b *testing.B) {
	l := newConfigured(io.Discard)
	b.ReportAllocs()
	for b.Loop() {
		l.outputw(LevelInfo, 3, "saved", "ref", "ab12cd34", "rows", 42)
	}
}

func BenchmarkOutputwJSON(b *testing.B) {
	l := newConfigured(io.Discard)
	l.SetJSON(true)
	b.ReportAllocs()
	for b.Loop() {
		l.outputw(LevelInfo, 3, "saved", "ref", "ab12cd34", "rows", 42)
	}
}

func BenchmarkPrintfJSON(b *testing.B) {
	l := newConfigured(io.Discard)
	l.SetJSON(true)
	b.ReportAllocs()
	for b.Loop() {
		l.outputf(LevelInfo, 3, "user %s did %s rev %d", "alice", "save", 42)
	}
}
