package main

import (
	"io"
	"os"
	"strings"
	"testing"
)

func TestUsageWritesCommandSyntax(t *testing.T) {
	output := captureFileOutput(t, &os.Stderr, usage)

	if output != "usage: pawit-migrate <up|down|status>\n" {
		t.Fatalf("unexpected usage output %q", output)
	}
}

func TestWriteJSONFormatsPayload(t *testing.T) {
	payload := struct {
		Direction string `json:"direction"`
		Applied   int    `json:"applied"`
	}{
		Direction: "up",
		Applied:   2,
	}

	output := captureFileOutput(t, &os.Stdout, func() {
		writeJSON(payload)
	})

	if !strings.Contains(output, "\n  \"direction\": \"up\",\n") {
		t.Fatalf("expected indented direction field, got %q", output)
	}
	if !strings.Contains(output, "\n  \"applied\": 2\n") {
		t.Fatalf("expected indented applied field, got %q", output)
	}
}

func captureFileOutput(t *testing.T, target **os.File, fn func()) string {
	t.Helper()

	reader, writer, err := os.Pipe()
	if err != nil {
		t.Fatalf("create pipe: %v", err)
	}

	original := *target
	*target = writer
	defer func() {
		*target = original
	}()

	fn()

	if err := writer.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	output, err := io.ReadAll(reader)
	if err != nil {
		t.Fatalf("read output: %v", err)
	}

	if err := reader.Close(); err != nil {
		t.Fatalf("close reader: %v", err)
	}

	return string(output)
}
