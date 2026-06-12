package sqlite

import (
	"testing"
)

func TestListUsersOrdering(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	for _, name := range []string{"first", "second", "third"} {
		_, err := s.CreateUser(name, name+"@example.com", "h", false)
		if err != nil {
			t.Fatalf("CreateUser(%s): %v", name, err)
		}
	}

	users, total, err := s.ListUsers(10, 0)
	if err != nil {
		t.Fatalf("ListUsers: %v", err)
	}
	if total != 3 {
		t.Fatalf("total = %d, want 3", total)
	}
	if len(users) != 3 {
		t.Fatalf("len(users) = %d, want 3", len(users))
	}
	// id DESC ⇒ third comes first
	if users[0].Username != "third" || users[2].Username != "first" {
		t.Fatalf("ordering wrong: %v", []string{users[0].Username, users[1].Username, users[2].Username})
	}
}

func TestListUsersPagination(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	for i := range 5 {
		_, err := s.CreateUser(string(rune('a'+i))+"user", "", "h", false)
		if err != nil {
			t.Fatalf("CreateUser: %v", err)
		}
	}

	first, total, _ := s.ListUsers(2, 0)
	if total != 5 {
		t.Fatalf("total = %d, want 5", total)
	}
	if len(first) != 2 {
		t.Fatalf("first page len = %d, want 2", len(first))
	}

	second, _, _ := s.ListUsers(2, 2)
	if len(second) != 2 {
		t.Fatalf("second page len = %d, want 2", len(second))
	}
	if first[0].ID == second[0].ID {
		t.Fatal("pages overlap")
	}

	third, _, _ := s.ListUsers(2, 4)
	if len(third) != 1 {
		t.Fatalf("third page len = %d, want 1", len(third))
	}
}

func TestGetUserByRefID(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	u, _ := s.CreateUser("alice", "a@example.com", "h", false)

	got, err := s.GetUserByRefID(u.ReferenceID)
	if err != nil {
		t.Fatalf("GetUserByRefID: %v", err)
	}
	if got == nil || got.ID != u.ID {
		t.Fatalf("got = %+v, want user with ID %d", got, u.ID)
	}

	missing, err := s.GetUserByRefID("does-not-exist")
	if err != nil {
		t.Fatalf("GetUserByRefID(missing): %v", err)
	}
	if missing != nil {
		t.Fatal("missing user returned non-nil")
	}
}

func TestUpdateUserSysop(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	admin, _ := s.CreateUser("admin", "admin@example.com", "h", true)
	bob, _ := s.CreateUser("bob", "bob@example.com", "h", false)

	// admin promotes bob
	err := s.UpdateUserSysop(bob.ID, true, admin.ID)
	if err != nil {
		t.Fatalf("UpdateUserSysop(promote): %v", err)
	}
	reloaded, _ := s.GetUserByID(bob.ID)
	if !reloaded.Sysop {
		t.Fatal("bob is not sysop after promotion")
	}

	// admin tries to demote themselves — rejected
	err = s.UpdateUserSysop(admin.ID, false, admin.ID)
	if err == nil {
		t.Fatal("admin demoting themselves was accepted")
	}
	stillAdmin, _ := s.GetUserByID(admin.ID)
	if !stillAdmin.Sysop {
		t.Fatal("admin was demoted despite the guard")
	}
}

func TestUpdateUserEnabled(t *testing.T) {
	t.Parallel()

	s := initTestDB(t)
	defer s.Close()

	admin, _ := s.CreateUser("admin", "", "h", true)
	carol, _ := s.CreateUser("carol", "", "h", false)

	err := s.UpdateUserEnabled(carol.ID, false, admin.ID)
	if err != nil {
		t.Fatalf("UpdateUserEnabled(disable carol): %v", err)
	}
	reloaded, _ := s.GetUserByID(carol.ID)
	if reloaded.Enabled {
		t.Fatal("carol still enabled after disable")
	}

	err = s.UpdateUserEnabled(admin.ID, false, admin.ID)
	if err == nil {
		t.Fatal("admin disabling themselves was accepted")
	}
}
