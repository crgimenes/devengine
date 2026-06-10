package templates

import (
	"strings"
	"testing"

	"github.com/crgimenes/devengine/config"
	"github.com/crgimenes/devengine/db"
)

// A render that dies mid-execution must not leak partial output: the caller
// needs a clean writer to send a proper 500.
func TestExecuteTemplateNoPartialOutputOnError(t *testing.T) {
	var sb strings.Builder
	// tools_invites.go.tmpl needs several fields; an empty struct fails.
	err := ExecuteTemplate(&sb, "tools_invites.go.tmpl", struct{}{})
	if err == nil {
		t.Fatal("expected render error")
	}
	if sb.Len() != 0 {
		t.Fatalf("partial output leaked: %d bytes", sb.Len())
	}
}

func TestExecuteTemplateWritesOnSuccess(t *testing.T) {
	var sb strings.Builder
	data := struct {
		Authed  bool
		User    db.User
		Error   string
		Message string
		Config  config.Config
	}{}
	err := ExecuteTemplate(&sb, "login.go.tmpl", data)
	if err != nil {
		t.Fatalf("ExecuteTemplate: %v", err)
	}
	if sb.Len() == 0 {
		t.Fatal("no output written")
	}
}
