package main

import (
	_ "embed"
	"fmt"
	"os"
)

// appMainTemplate is the starter main.go written into a fresh app folder.
//
//go:embed appmain.txt
var appMainTemplate string

// initFiloTemplate is the starter init.filo. dbFile becomes the
// (set DBFile ...) value so the config matches the database this command
// initializes.
func initFiloTemplate(dbFile string) string {
	return fmt.Sprintf(`;; Application configuration, read at startup by main.go.
;; Each (set Key value) overrides a built-in default.

(set Address ":3000")
(set BaseURL "http://localhost:3000")
(set SiteTitle "My App")
(set DBFile %q)

;; UI language. The engine ships English (en-US) and a pt-BR dictionary.
(set Locale "en-US")
`, dbFile)
}

// scaffold writes init.filo and main.go into the current directory when they
// are absent, turning a fresh `go mod init`'d folder into a runnable app.
// Existing files are never overwritten — running this in an established app
// only reports that it kept them, so it is safe to re-run for a new sysop.
func scaffold(dbFile string) error {
	files := []struct {
		name    string
		content string
	}{
		{"init.filo", initFiloTemplate(dbFile)},
		{"main.go", appMainTemplate},
	}
	for _, f := range files {
		err := writeIfAbsent(f.name, f.content)
		if err != nil {
			return err
		}
	}
	return nil
}

func writeIfAbsent(name, content string) error {
	_, err := os.Stat(name)
	if err == nil {
		fmt.Printf("kept existing %s\n", name)
		return nil
	}
	if !os.IsNotExist(err) {
		return fmt.Errorf("stat %s: %w", name, err)
	}
	err = os.WriteFile(name, []byte(content), 0o600)
	if err != nil {
		return fmt.Errorf("write %s: %w", name, err)
	}
	fmt.Printf("created %s\n", name)
	return nil
}
