package sqlite

import (
	"path/filepath"
	"testing"

	"github.com/crgimenes/devengine/db"
)

// Soft-deleted files keep occupying disk until purged, so the quota sum must
// include them.
func TestSumFileSizesByUserIDCountsSoftDeleted(t *testing.T) {
	s, err := NewWithPath(filepath.Join(t.TempDir(), "quota.db"))
	if err != nil {
		t.Fatalf("NewWithPath: %v", err)
	}
	defer s.Close()
	err = RunMigrationOn(s)
	if err != nil {
		t.Fatalf("RunMigrationOn: %v", err)
	}

	u, err := s.CreateUser("alice", "alice@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	other, err := s.CreateUser("bob", "bob@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	save := func(userID, size int64, name string) {
		t.Helper()
		_, err := s.SaveFileMetadata(&db.File{
			UserID:           userID,
			OriginalFilename: name,
			Filename:         name,
			Filesize:         size,
			Filetype:         "image/png",
			Filehash:         name,
		})
		if err != nil {
			t.Fatalf("SaveFileMetadata(%s): %v", name, err)
		}
	}
	save(u.ID, 100, "a.png")
	save(u.ID, 250, "b.png")
	save(other.ID, 999, "c.png")

	err = s.SoftDeleteFileByUserAndFilename(u.ID, "b.png")
	if err != nil {
		t.Fatalf("SoftDeleteFileByUserAndFilename: %v", err)
	}

	got, err := s.SumFileSizesByUserID(u.ID)
	if err != nil {
		t.Fatalf("SumFileSizesByUserID: %v", err)
	}
	if got != 350 {
		t.Fatalf("sum = %d, want 350 (soft-deleted must count)", got)
	}

	empty, err := s.CreateUser("carol", "carol@example.com", "x", false)
	if err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	got, err = s.SumFileSizesByUserID(empty.ID)
	if err != nil {
		t.Fatalf("SumFileSizesByUserID(empty): %v", err)
	}
	if got != 0 {
		t.Fatalf("sum for empty user = %d, want 0", got)
	}
}
