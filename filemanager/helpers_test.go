package filemanager

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

func TestValidateFilename(t *testing.T) {
	valid := []string{"a.png", "relatorio_anual-2026.txt", "x"}
	for _, name := range valid {
		if err := ValidateFilename(name); err != nil {
			t.Errorf("ValidateFilename(%q) = %v, want nil", name, err)
		}
	}
	invalid := []string{"", "../etc/passwd", "a/b.png", `a\b.png`, "a:b", "a*b",
		"a?b", "a<b", "a|b", `a"b`, strings.Repeat("x", 300)}
	for _, name := range invalid {
		if err := ValidateFilename(name); err == nil {
			t.Errorf("ValidateFilename(%q) = nil, want error", name)
		}
	}
}

func TestClassifyMediaKind(t *testing.T) {
	cases := []struct {
		mime, name, want string
	}{
		{"image/png", "x.bin", "image"},
		{"video/mp4", "x.bin", "video"},
		{"audio/mpeg", "x.bin", "audio"},
		{"", "photo.JPEG", "image"},
		{"", "clip.webm", "video"},
		{"", "song.m4a", "audio"},
		{"application/pdf", "doc.pdf", "other"},
		{"", "data.csv", "other"},
	}
	for _, c := range cases {
		if got := ClassifyMediaKind(c.mime, c.name); got != c.want {
			t.Errorf("ClassifyMediaKind(%q, %q) = %q, want %q", c.mime, c.name, got, c.want)
		}
	}
}

func TestFmtBytes(t *testing.T) {
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0 B"},
		{1023, "1023 B"},
		{1 << 10, "1.0 KB"},
		{512 << 10, "512.0 KB"},
		{1 << 20, "1.0 MB"},
		{(3 << 30) / 2, "1.5 GB"},
	}
	for _, c := range cases {
		if got := fmtBytes(c.n); got != c.want {
			t.Errorf("fmtBytes(%d) = %q, want %q", c.n, got, c.want)
		}
	}
}

func TestParseRFC3339(t *testing.T) {
	if _, ok := parseRFC3339("2026-06-12T10:00:00Z"); !ok {
		t.Error("valid RFC3339 rejected")
	}
	if _, ok := parseRFC3339("12/06/2026"); ok {
		t.Error("invalid format accepted")
	}
	if _, ok := parseRFC3339(""); ok {
		t.Error("empty string accepted")
	}
}

func TestDataPathRoundTrip(t *testing.T) {
	dir := t.TempDir()
	prev := config.Cfg
	config.Cfg = &config.Config{DataPath: dir}
	t.Cleanup(func() { config.Cfg = prev })

	abs := filepath.Join(dir, "ab", "cd", "user", "file.png")
	rel, err := MakeRelativeToDataPath(abs)
	if err != nil {
		t.Fatalf("MakeRelativeToDataPath: %v", err)
	}
	if filepath.IsAbs(rel) {
		t.Fatalf("rel = %q, want relative", rel)
	}

	back, err := ResolveAbsoluteFromDataPath(rel)
	if err != nil {
		t.Fatalf("ResolveAbsoluteFromDataPath: %v", err)
	}
	if back != abs {
		t.Fatalf("round trip = %q, want %q", back, abs)
	}

	// Paths outside the data dir keep their absolute form.
	outside := filepath.Join(t.TempDir(), "other.png")
	kept, err := MakeRelativeToDataPath(outside)
	if err != nil || kept != outside {
		t.Fatalf("outside path = %q, %v; want kept absolute", kept, err)
	}
}

func TestDataFilePathShardsByReferenceID(t *testing.T) {
	dir := t.TempDir()
	prev := config.Cfg
	config.Cfg = &config.Config{DataPath: dir}
	t.Cleanup(func() { config.Cfg = prev })

	u := &db.User{ReferenceID: "abcdef1234567890abcdef"}
	got, err := DataFilePath(u)
	if err != nil {
		t.Fatalf("DataFilePath: %v", err)
	}
	if !strings.Contains(got, filepath.Join("ab", "cd")) {
		t.Fatalf("path not sharded: %q", got)
	}
	if _, err := os.Stat(got); err != nil {
		t.Fatalf("directory not created: %v", err)
	}

	if _, err := DataFilePath(nil); err == nil {
		t.Fatal("nil user accepted")
	}
	if _, err := DataFilePath(&db.User{ReferenceID: "short"}); err == nil {
		t.Fatal("short reference id accepted")
	}
}

func TestFileHash(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.bin")
	err := os.WriteFile(path, []byte("hello"), 0o600)
	if err != nil {
		t.Fatalf("write: %v", err)
	}
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer f.Close()

	got, err := FileHash(f)
	if err != nil {
		t.Fatalf("FileHash: %v", err)
	}
	// sha256("hello")
	want := "2cf24dba5fb0a30e26e83b2ac5b9e29e1b161e5c1fa7425e73043362938b9824"
	if got != want {
		t.Fatalf("hash = %q, want %q", got, want)
	}
}
