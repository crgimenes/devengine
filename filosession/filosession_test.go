package filosession

import (
	"context"
	"testing"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/filo"
)

func TestNilUserReturnsZeroValues(t *testing.T) {
	c := NewContext(nil)

	cases := []struct {
		name string
		fn   func(context.Context, []filo.Value) (filo.Value, error)
		want filo.Value
	}{
		{"id", c.userID, filo.VString("")},
		{"username", c.userUsername, filo.VString("")},
		{"email", c.userEmail, filo.VString("")},
		{"sysop?", c.sysop, filo.VBool(false)},
	}
	for _, tc := range cases {
		got, err := tc.fn(context.Background(), nil)
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if got.Kind != tc.want.Kind {
			t.Errorf("%s kind = %v, want %v", tc.name, got.Kind, tc.want.Kind)
		}
		if got.Str != tc.want.Str || got.Bool != tc.want.Bool {
			t.Errorf("%s got %+v, want %+v", tc.name, got, tc.want)
		}
	}
}

func TestUserPopulated(t *testing.T) {
	u := &db.User{
		ReferenceID: "abc-123",
		Username:    "alice",
		Email:       "alice@example.com",
		Sysop:       true,
	}
	c := NewContext(u)

	id, _ := c.userID(context.Background(), nil)
	if id.Str != "abc-123" {
		t.Errorf("userID = %q", id.Str)
	}
	name, _ := c.userUsername(context.Background(), nil)
	if name.Str != "alice" {
		t.Errorf("userUsername = %q", name.Str)
	}
	email, _ := c.userEmail(context.Background(), nil)
	if email.Str != "alice@example.com" {
		t.Errorf("userEmail = %q", email.Str)
	}
	sysop, _ := c.sysop(context.Background(), nil)
	if !sysop.Bool {
		t.Error("sysop = false, want true")
	}
}

func TestRejectsExtraArgs(t *testing.T) {
	c := NewContext(nil)
	_, err := c.userID(context.Background(), []filo.Value{filo.VString("x")})
	if err == nil {
		t.Fatal("expected arity error")
	}
}
