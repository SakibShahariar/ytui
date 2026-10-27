package main

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestConfirmImport(t *testing.T) {
	tests := []struct {
		name string
		input string
		want bool
	}{
		{"yes", "y\n", true},
		{"case insensitive yes", "YES\n", true},
		{"no", "n\n", false},
		{"empty defaults to no", "\n", false},
		{"unexpected answer", "sure\n", false},
		{"yes without trailing newline", "yes", true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var output bytes.Buffer
			got, err := confirmImport(strings.NewReader(tt.input), &output, "settings.json", "/tmp/config.json")
			if err != nil {
				t.Fatalf("confirmImport() error = %v", err)
			}
			if got != tt.want {
				t.Fatalf("confirmImport() = %v, want %v", got, tt.want)
			}
			if !strings.Contains(output.String(), "settings.json") || !strings.Contains(output.String(), "/tmp/config.json") {
				t.Fatalf("confirmation prompt did not show source and destination: %q", output.String())
			}
		})
	}
}

func TestConfirmImportInputError(t *testing.T) {
	reader := errorReader{}
	var output bytes.Buffer
	if confirmed, err := confirmImport(reader, &output, "settings.json", "/tmp/config.json"); err == nil || confirmed {
		t.Fatalf("confirmImport() = (%v, %v), want (false, error)", confirmed, err)
	}
}

type errorReader struct{}

func (errorReader) Read([]byte) (int, error) { return 0, errors.New("input unavailable") }

var _ io.Reader = errorReader{}
