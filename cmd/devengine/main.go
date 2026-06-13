// Command devengine is the command-line companion for devengine applications.
//
// Usage:
//
//	devengine init [-db file]
//
// In a fresh, `go mod init`'d folder, `devengine init` writes a starter
// init.filo and main.go (never overwriting existing ones), creates the
// database, runs the engine migrations, and prompts for the first sysop user.
// Re-running it in an established app keeps your files and just adds another
// sysop.
package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"golang.org/x/term"

	"github.com/crgimenes/devengine/auth/basic"
	"github.com/crgimenes/devengine/db"
	"github.com/crgimenes/devengine/db/sqlite"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	cmd, args := os.Args[1], os.Args[2:]
	switch cmd {
	case "init":
		err := runInit(args)
		if err != nil {
			fmt.Fprintln(os.Stderr, "devengine init:", err)
			os.Exit(1)
		}
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "devengine: unknown command %q\n\n", cmd)
		usage()
		os.Exit(2)
	}
}

func usage() {
	fmt.Fprint(os.Stderr, `devengine — command-line companion for devengine apps

Usage:
    devengine <command> [flags]

Commands:
    init    scaffold a new app (init.filo, main.go), create the database,
            and the first sysop user

Run "devengine init -h" for its flags.
`)
}

func runInit(args []string) error {
	fs := flag.NewFlagSet("init", flag.ExitOnError)
	dbPath := fs.String("db", "app.db", "path to the SQLite database file")
	err := fs.Parse(args)
	if err != nil {
		return err
	}
	return initApp(*dbPath)
}

// initApp scaffolds the starter files, opens (creating if needed) the
// database, runs the engine migrations, and creates the first sysop user.
func initApp(dbPath string) error {
	err := scaffold(dbPath)
	if err != nil {
		return err
	}

	s, err := sqlite.NewWithPath(dbPath)
	if err != nil {
		return fmt.Errorf("open db %q: %w", dbPath, err)
	}
	defer s.Close()

	db.Storage = s
	err = sqlite.RunMigration()
	if err != nil {
		return fmt.Errorf("run migrations: %w", err)
	}

	reader := bufio.NewReader(os.Stdin)

	username, err := promptLine(reader, "Username for the new sysop: ")
	if err != nil {
		return err
	}
	username = strings.TrimSpace(username)
	if username == "" {
		return errors.New("username is required")
	}

	email, err := promptLine(reader, "Email (optional, press Enter to skip): ")
	if err != nil {
		return err
	}
	email = strings.TrimSpace(email)

	password, err := promptPassword(reader, "Password: ")
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("password is required")
	}

	confirm, err := promptPassword(reader, "Confirm password: ")
	if err != nil {
		return err
	}
	if password != confirm {
		return errors.New("passwords do not match")
	}

	hash, err := basic.HashPassword(password)
	if err != nil {
		return fmt.Errorf("hash password: %w", err)
	}

	u, err := db.Storage.CreateUser(username, email, hash, true)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}

	fmt.Printf("created sysop %q (reference_id=%s)\n", u.Username, u.ReferenceID)
	fmt.Println("\nNext: run the app with  go run .")
	return nil
}

func promptLine(r *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	line, err := r.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("read input: %w", err)
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func promptPassword(r *bufio.Reader, prompt string) (string, error) {
	fmt.Print(prompt)
	defer fmt.Println()

	fd := int(os.Stdin.Fd()) // #nosec G115 -- stdin fd is small, conversion safe
	if !term.IsTerminal(fd) {
		// Non-interactive: reuse the same buffered reader so any bytes the
		// previous prompts pre-buffered remain visible.
		line, err := r.ReadString('\n')
		if err != nil {
			return "", fmt.Errorf("read password: %w", err)
		}
		return strings.TrimRight(line, "\r\n"), nil
	}

	data, err := term.ReadPassword(fd)
	if err != nil {
		return "", fmt.Errorf("read password: %w", err)
	}
	return string(data), nil
}
