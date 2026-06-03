//go:build ignore

// gen_sum regenerates the atlas.sum integrity file for every driver
// subdirectory under ./migrations. Run via `go run ./internal/migrations/gen_sum`.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"ariga.io/atlas/sql/migrate"
)

func main() {
	root := "internal/migrations"
	entries, err := os.ReadDir(root)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		// Only driver subdirectories are valid migration dirs.
		if e.Name() != "postgres" && e.Name() != "mysql" {
			continue
		}
		dir := filepath.Join(root, e.Name())
		ld, err := migrate.NewLocalDir(dir)
		if err != nil {
			fmt.Fprintf(os.Stderr, "open %s: %v\n", dir, err)
			os.Exit(1)
		}
		files, err := ld.Files()
		if err != nil {
			fmt.Fprintf(os.Stderr, "list %s: %v\n", dir, err)
			os.Exit(1)
		}
		hf, err := migrate.NewHashFile(files)
		if err != nil {
			fmt.Fprintf(os.Stderr, "hash %s: %v\n", dir, err)
			os.Exit(1)
		}
		if err := migrate.WriteSumFile(ld, hf); err != nil {
			fmt.Fprintf(os.Stderr, "write %s: %v\n", dir, err)
			os.Exit(1)
		}
		fmt.Printf("wrote %s/atlas.sum\n", dir)
	}
}
