package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"time"

	"pawit-vetcare/internal/config"
	"pawit-vetcare/internal/database"
)

func main() {
	if len(os.Args) != 2 {
		usage()
		os.Exit(2)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	cfg, err := config.Load()
	if err != nil {
		slog.Error("configuration failed", "error", err)
		os.Exit(1)
	}

	pool, err := database.NewPool(ctx, cfg.DatabaseURL)
	if err != nil {
		slog.Error("database connection failed", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	runner := database.NewRunner(pool)

	switch os.Args[1] {
	case "up":
		applied, err := runner.Up(ctx)
		exitOnError("migrate up failed", err)
		writeJSON(map[string]any{"direction": "up", "applied": applied})
	case "down":
		applied, err := runner.Down(ctx)
		exitOnError("migrate down failed", err)
		writeJSON(map[string]any{"direction": "down", "applied": applied})
	case "status":
		status, err := runner.Status(ctx)
		exitOnError("migration status failed", err)
		writeJSON(map[string]any{"items": status})
	default:
		usage()
		os.Exit(2)
	}
}

func exitOnError(message string, err error) {
	if err == nil {
		return
	}
	slog.Error(message, "error", err)
	os.Exit(1)
}

func writeJSON(payload any) {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	_ = encoder.Encode(payload)
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: pawit-migrate <up|down|status>")
}
