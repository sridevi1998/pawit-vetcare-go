package database

import (
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationFiles embed.FS

type Migration struct {
	Name string
	SQL  string
}

func Migrations(direction string) ([]Migration, error) {
	if direction != "up" && direction != "down" {
		return nil, fmt.Errorf("migration direction must be up or down, got %q", direction)
	}

	entries, err := fs.ReadDir(migrationFiles, "migrations")
	if err != nil {
		return nil, err
	}

	suffix := "." + direction + ".sql"
	migrations := make([]Migration, 0, len(entries))
	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, suffix) {
			continue
		}
		contents, err := migrationFiles.ReadFile("migrations/" + name)
		if err != nil {
			return nil, err
		}
		migrations = append(migrations, Migration{Name: name, SQL: string(contents)})
	}

	sort.Slice(migrations, func(i, j int) bool {
		if direction == "down" {
			return migrations[i].Name > migrations[j].Name
		}
		return migrations[i].Name < migrations[j].Name
	})

	return migrations, nil
}
