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
)

func run(dbPath string) error {
	s, err := db.NewWithPath(dbPath)
	if err != nil {
		return fmt.Errorf("open db %q: %w", dbPath, err)
	}
	defer s.Close()

	db.Storage = s
	err = db.RunMigration()
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

	password, err := promptPassword("Password: ")
	if err != nil {
		return err
	}
	if password == "" {
		return errors.New("password is required")
	}

	confirm, err := promptPassword("Confirm password: ")
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

func promptPassword(prompt string) (string, error) {
	fmt.Print(prompt)
	defer fmt.Println()

	fd := int(os.Stdin.Fd()) // #nosec G115 -- stdin fd is small, conversion safe
	if !term.IsTerminal(fd) {
		// Non-interactive: read a line without hiding.
		line, err := bufio.NewReader(os.Stdin).ReadString('\n')
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
