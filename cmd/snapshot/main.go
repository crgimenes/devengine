// Command snapshot takes a consistent backup of a devengine application:
// a SQLite snapshot via VACUUM INTO (safe under WAL with the app running)
// plus a tar.gz of the data directory (uploads). Stdlib only.
//
// Usage:
//
//	snapshot -db devengine.db -data ./data -out ./backups
package main

import (
	"archive/tar"
	"compress/gzip"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/crgimenes/devengine/db"
)

func main() {
	dbPath := flag.String("db", "devengine.db", "path to the SQLite database")
	dataPath := flag.String("data", "./data", "data directory to archive (skipped when absent)")
	outDir := flag.String("out", "./backups", "directory receiving the snapshot files")
	flag.Parse()

	err := run(*dbPath, *dataPath, *outDir, time.Now())
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(1)
	}
}

func run(dbPath, dataPath, outDir string, now time.Time) error {
	err := os.MkdirAll(outDir, 0o750)
	if err != nil {
		return fmt.Errorf("create output dir: %w", err)
	}

	stamp := now.Format("20060102-150405")
	base := filepath.Join(outDir, stamp)

	dbOut := base + ".db"
	err = snapshotDB(dbPath, dbOut)
	if err != nil {
		return err
	}
	report(dbOut)

	info, err := os.Stat(dataPath)
	if err != nil || !info.IsDir() {
		fmt.Println("data dir absent, skipping archive:", dataPath)
		return nil
	}

	tarOut := base + "-data.tar.gz"
	err = tarDir(dataPath, tarOut)
	if err != nil {
		return err
	}
	report(tarOut)
	return nil
}

// snapshotDB writes a consistent copy of the database. VACUUM INTO runs in
// its own transaction, so it is safe while the application keeps writing.
func snapshotDB(dbPath, dest string) error {
	_, err := os.Stat(dbPath)
	if err != nil {
		return fmt.Errorf("database %q: %w", dbPath, err)
	}
	s, err := db.NewWithPath(dbPath)
	if err != nil {
		return fmt.Errorf("open db %q: %w", dbPath, err)
	}
	defer s.Close()

	err = s.Exec("VACUUM INTO ?", dest)
	if err != nil {
		return fmt.Errorf("vacuum into %q: %w", dest, err)
	}
	return nil
}

// tarDir archives the directory into a tar.gz, storing paths relative to it.
func tarDir(src, dest string) error {
	out, err := os.Create(filepath.Clean(dest))
	if err != nil {
		return fmt.Errorf("create archive: %w", err)
	}
	defer out.Close()

	gz := gzip.NewWriter(out)
	tw := tar.NewWriter(gz)

	err = filepath.Walk(src, func(path string, info os.FileInfo, werr error) error {
		if werr != nil {
			return werr
		}
		rel, rerr := filepath.Rel(src, path)
		if rerr != nil {
			return rerr
		}
		if rel == "." {
			return nil
		}

		hdr, herr := tar.FileInfoHeader(info, "")
		if herr != nil {
			return herr
		}
		hdr.Name = filepath.ToSlash(rel)
		herr = tw.WriteHeader(hdr)
		if herr != nil {
			return herr
		}
		if info.IsDir() {
			return nil
		}

		f, ferr := os.Open(filepath.Clean(path))
		if ferr != nil {
			return ferr
		}
		defer f.Close()
		_, cerr := io.Copy(tw, f)
		return cerr
	})
	if err != nil {
		return fmt.Errorf("archive %q: %w", src, err)
	}

	err = tw.Close()
	if err != nil {
		return fmt.Errorf("close tar: %w", err)
	}
	err = gz.Close()
	if err != nil {
		return fmt.Errorf("close gzip: %w", err)
	}
	return out.Close()
}

func report(path string) {
	info, err := os.Stat(path)
	if err != nil {
		fmt.Println("written:", path)
		return
	}
	fmt.Printf("written: %s (%d bytes)\n", path, info.Size())
}
