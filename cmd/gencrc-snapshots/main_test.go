package main

import (
	"bytes"
	"errors"
	"flag"
	"strings"
	"testing"
)

func TestCommandValidation(t *testing.T) {
	for _, tt := range []struct {
		name string
		args []string
		want string
	}{
		{"unexpected argument", []string{"extra"}, "unexpected argument"},
		{"invalid game", []string{"--game", "unknown"}, "unknown game"},
		{"verify cannot take roots", []string{"--verify", "--generals-root", "/unused"}, "do not supply installation roots"},
		{"verify missing output", []string{"--verify", "--output", t.TempDir()}, "snapshot directory"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			err := run(tt.args, &stdout, &stderr)
			if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("wanted %q error, got %v", tt.want, err)
			}
			if stdout.Len() != 0 {
				t.Fatalf("failed operation reported success: %s", stdout.String())
			}
		})
	}
}

func TestHelp(t *testing.T) {
	var stdout, stderr bytes.Buffer
	if err := run([]string{"--help"}, &stdout, &stderr); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error: %v", err)
	}
	for _, word := range []string{"generals-root", "zero-hour-root", "verify", "raw INI/SCB"} {
		if !strings.Contains(stderr.String(), word) {
			t.Fatalf("help does not describe %s", word)
		}
	}
}
