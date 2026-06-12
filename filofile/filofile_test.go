package filofile

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
	"github.com/crgimenes/filo"
)

func newTestStore(t *testing.T) db.Store {
	t.Helper()
	path := filepath.Join(t.TempDir(), "test.db")
	s, err := sqlite.NewWithPath(path)
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	err = sqlite.RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}
	return s
}

func insertFile(t *testing.T, s db.Store, userID int64, filename, original, hash string, size int64, mime string) {
	t.Helper()
	now := time.Now().UTC().Format(time.RFC3339)
	err := s.Exec(`INSERT INTO filemanager_files
		(user_id, original_filename, filename, filesize, filetype, filehash, filetag, filedescription, processed, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, '', '', 0, ?, ?)`,
		userID, original, filename, size, mime, hash, now, now)
	if err != nil {
		t.Fatalf("insert file: %v", err)
	}
}

func TestFileExists(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	u, _ := s.CreateUser("owner", "owner@example.com", "h", true)
	insertFile(t, s, u.ID, "stored-abc", "photo.jpg", "deadbeef", 1024, "image/jpeg")

	c := NewContext(s)

	got, err := c.fileExists(context.Background(), []filo.Value{filo.VString("stored-abc")})
	if err != nil {
		t.Fatalf("fileExists: %v", err)
	}
	if !got.Bool {
		t.Error("fileExists existing = false")
	}

	got, err = c.fileExists(context.Background(), []filo.Value{filo.VString("missing")})
	if err != nil {
		t.Fatalf("fileExists missing: %v", err)
	}
	if got.Bool {
		t.Error("fileExists missing = true")
	}
}

func TestFileMetadata(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	u, _ := s.CreateUser("owner", "owner@example.com", "h", true)
	insertFile(t, s, u.ID, "stored-xyz", "doc.pdf", "abcd1234", 2048, "application/pdf")

	c := NewContext(s)
	arg := []filo.Value{filo.VString("stored-xyz")}

	name, _ := c.fileOriginalName(context.Background(), arg)
	if name.Str != "doc.pdf" {
		t.Errorf("originalName = %q", name.Str)
	}
	size, _ := c.fileSize(context.Background(), arg)
	if int64(size.Num) != 2048 {
		t.Errorf("size = %v", size.Num)
	}
	hash, _ := c.fileHash(context.Background(), arg)
	if hash.Str != "abcd1234" {
		t.Errorf("hash = %q", hash.Str)
	}
	mime, _ := c.fileType(context.Background(), arg)
	if mime.Str != "application/pdf" {
		t.Errorf("type = %q", mime.Str)
	}
}

func TestRejectsBadArity(t *testing.T) {
	s := newTestStore(t)
	defer s.Close()

	c := NewContext(s)
	_, err := c.fileExists(context.Background(), nil)
	if err == nil {
		t.Fatal("expected arity error")
	}
}
