package sqlite

import (
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/storetest"
)

// TestConformance runs the shared db.Store battery against SQLite. The same
// battery runs in db/postgres against a container; behavior differences
// between backends are bugs in one of them.
func TestConformance(t *testing.T) {
	storetest.Run(t, func(t *testing.T) db.Store {
		s := initTestDB(t)
		t.Cleanup(s.Close)
		return s
	})
}
