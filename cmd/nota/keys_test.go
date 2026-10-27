package main

import (
	"slices"
	"testing"

	"nota/internal/keymap"
	"nota/internal/kit"
)

func TestKeyLabel(t *testing.T) {
	app := &App{bindings: keymap.Default()}
	for cmd, want := range map[string][]string{
		"quickOpen": {kit.Mod(), "O"},
		"search":    {kit.Mod(), "Shift", "F"},
		"unbound":   nil,
	} {
		if got := app.keyLabel(cmd); !slices.Equal(got, want) {
			t.Errorf("keyLabel(%q) = %q, want %q", cmd, got, want)
		}
	}
}
