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

func run(dbPath string) error {
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

func main() {
	dbPath := flag.String("db", "devengine.db", "path to the SQLite database file")
	flag.Parse()

	err := run(*dbPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, "admin-bootstrap:", err)
		os.Exit(1)
	}
}
