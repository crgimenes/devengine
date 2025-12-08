// Command filo-cli runs Filo scripts with optional database context.
// Without database flags, it runs pure Filo scripts.
// With --filo-package=eav, it requires database, user, and workspace.
//
// Usage:
//
//	echo '(+ 1 2)' | filo-cli
//	filo-cli --script-file script.filo --filo-package math
//	filo-cli --db db.sqlite --email user@example.com --workspace-id 1 --filo-package eav
package main

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"slices"
	"strings"
	"time"

	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/filo"
	"github.com/crgimenes/devengine/filoeav"
	"github.com/crgimenes/devengine/filomath"
)

const (
	defaultStepLimit      = 100000
	defaultRecursionLimit = 128
	defaultTimeoutSeconds = 30
)

// packageInfo describes a Filo extension package.
type packageInfo struct {
	name        string
	requiresDB  bool
	description string
}

// availablePackages lists all supported extension packages.
var availablePackages = []packageInfo{
	{name: "eav", requiresDB: true, description: "EAV data access builtins"},
	{name: "math", requiresDB: false, description: "Advanced math functions"},
}

func main() {
	os.Exit(run(os.Args[1:], os.Stdin, os.Stdout, os.Stderr))
}

// run executes the CLI logic and returns the exit code.
// Separated from main() for testability.
func run(args []string, stdin io.Reader, stdout, stderr io.Writer) int {
	fs := flag.NewFlagSet("filo-cli", flag.ContinueOnError)
	fs.SetOutput(stderr)

	var (
		dbPath         string
		email          string
		workspaceID    int64
		workspaceRef   string
		scriptFile     string
		packages       string
		stepLimit      int
		recursionLimit int
		timeoutSeconds int
	)

	fs.StringVar(&dbPath, "db", "", "Path to SQLite database file (required for eav package)")
	fs.StringVar(&email, "email", "", "Email of user to load (required for eav package)")
	fs.Int64Var(&workspaceID, "workspace-id", 0, "Workspace ID (required for eav package)")
	fs.StringVar(&workspaceRef, "workspace-ref", "", "Workspace reference ID (alternative to --workspace-id)")
	fs.StringVar(&scriptFile, "script-file", "", "Path to Filo script file (if omitted, reads from stdin)")
	fs.StringVar(&packages, "filo-package", "", "Comma-separated list of extension packages (eav, math)")
	fs.IntVar(&stepLimit, "step-limit", defaultStepLimit, "Maximum evaluation steps")
	fs.IntVar(&recursionLimit, "recursion-limit", defaultRecursionLimit, "Maximum recursion depth")
	fs.IntVar(&timeoutSeconds, "timeout", defaultTimeoutSeconds, "Script execution timeout in seconds")

	if err := fs.Parse(args); err != nil {
		return 1
	}

	// Parse requested packages
	var requestedPackages []string
	if packages != "" {
		for p := range strings.SplitSeq(packages, ",") {
			p = strings.TrimSpace(p)
			if p != "" {
				requestedPackages = append(requestedPackages, p)
			}
		}
	}

	// Check if any package requires database
	needsDB := slices.ContainsFunc(requestedPackages, requiresDatabase)

	// Validate database-related flags only if needed
	if needsDB {
		if dbPath == "" {
			fmt.Fprintln(stderr, "error: --db is required when using eav package")
			return 1
		}
		if email == "" {
			fmt.Fprintln(stderr, "error: --email is required when using eav package")
			return 1
		}
		if workspaceID == 0 && workspaceRef == "" {
			fmt.Fprintln(stderr, "error: --workspace-id or --workspace-ref is required when using eav package")
			return 1
		}
	}

	// Load script first (before potentially expensive DB operations)
	var script string
	if scriptFile != "" {
		data, err := os.ReadFile(scriptFile)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to read script file: %v\n", err)
			return 1
		}
		script = string(data)
	} else {
		data, err := io.ReadAll(stdin)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to read script from stdin: %v\n", err)
			return 1
		}
		script = string(data)
	}

	if strings.TrimSpace(script) == "" {
		fmt.Fprintln(stderr, "error: empty script")
		return 1
	}

	// Create Filo engine
	engine := filo.NewEngine()

	// Set up globals (may be populated with DB context)
	globals := make(map[string]filo.Value)

	// Initialize database context if needed
	var storage *db.SQLite
	var user *db.User
	var workspace *db.EAVWorkspace

	if needsDB {
		var err error
		storage, err = db.NewWithPath(dbPath)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to open database: %v\n", err)
			return 1
		}
		defer storage.Close()

		user, err = storage.GetUserByEmail(email)
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to load user: %v\n", err)
			return 1
		}
		if !user.Enabled {
			fmt.Fprintf(stderr, "error: user %q is not enabled\n", email)
			return 1
		}

		if workspaceID > 0 {
			workspace, err = storage.GetEAVWorkspace(workspaceID)
		} else {
			workspace, err = storage.GetEAVWorkspaceByReferenceID(workspaceRef)
		}
		if err != nil {
			fmt.Fprintf(stderr, "error: failed to load workspace: %v\n", err)
			return 1
		}

		// Add database context to globals
		globals["user-id"] = filo.VNum(float64(user.ID))
		globals["user-email"] = filo.VString(user.Email)
		globals["user-name"] = filo.VString(user.Username)
		globals["workspace-id"] = filo.VNum(float64(workspace.ID))
		globals["workspace-ref"] = filo.VString(workspace.ReferenceID)
	}

	// Register requested packages
	for _, pkg := range requestedPackages {
		if err := registerPackage(engine, storage, pkg, workspace, user); err != nil {
			fmt.Fprintf(stderr, "error: %v\n", err)
			return 1
		}
	}

	// Configure evaluation
	cfg := filo.EvalConfig{
		StepLimit:      stepLimit,
		RecursionLimit: recursionLimit,
		Timeout:        time.Duration(timeoutSeconds) * time.Second,
	}

	// Execute script
	ctx := context.Background()
	result, _, err := engine.RunScript(ctx, script, globals, cfg)
	if err != nil {
		fmt.Fprintf(stderr, "error: script execution failed: %v\n", err)
		return 1
	}

	// Print result
	fmt.Fprintln(stdout, formatResult(result))
	return 0
}

// requiresDatabase checks if a package name requires database access.
func requiresDatabase(pkg string) bool {
	for _, p := range availablePackages {
		if p.name == pkg {
			return p.requiresDB
		}
	}
	return false
}

// registerPackage registers a Filo extension package by name.
func registerPackage(engine *filo.Engine, storage *db.SQLite, pkg string, workspace *db.EAVWorkspace, user *db.User) error {
	switch pkg {
	case "eav":
		if storage == nil || workspace == nil || user == nil {
			return fmt.Errorf("eav package requires database context")
		}
		filoeav.RegisterEAVBuiltins(engine, storage, filoeav.Config{
			WorkspaceID: workspace.ID,
			UserID:      user.ID,
		})
	case "math":
		filomath.RegisterMathBuiltins(engine)
	default:
		return fmt.Errorf("unknown filo package: %q (available: eav, math)", pkg)
	}
	return nil
}

// formatResult converts a Filo value to a human-readable string.
func formatResult(v filo.Value) string {
	switch v.Kind {
	case filo.KNumber:
		// Remove trailing zeros for cleaner output
		return strings.TrimRight(strings.TrimRight(fmt.Sprintf("%f", v.Num), "0"), ".")
	case filo.KBool:
		if v.Bool {
			return "#t"
		}
		return "#f"
	case filo.KString:
		return v.Str
	case filo.KList:
		var items []string
		for _, item := range v.List {
			items = append(items, formatResult(item))
		}
		return "(" + strings.Join(items, " ") + ")"
	case filo.KTuple:
		var items []string
		for _, item := range v.Tup {
			items = append(items, formatResult(item))
		}
		return "(values " + strings.Join(items, " ") + ")"
	case filo.KFunc:
		return "<fn>"
	default:
		return v.String()
	}
}
