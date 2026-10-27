package keymap

import "testing"

func TestParseAndLookup(t *testing.T) {
	src := []byte(`[
  // quick open
  { "key": "ctrl+o", "command": "quickOpen", },
  { "key": "ctrl+shift+f", "command": "search" }
]`)
	got, err := Parse(src)
	if err != nil || len(got) != 2 || got[0].Command != "quickOpen" {
		t.Fatalf("%v %#v", err, got)
	}
	ch, ok := ParseChord("ctrl+shift+f")
	if !ok || !ch.Ctrl || !ch.Shift || ch.Key != "f" {
		t.Fatalf("%+v", ch)
	}
	cmd, ok := Lookup(got, ch, true)
	if !ok || cmd != "search" {
		t.Fatalf("%q %v", cmd, ok)
	}
	cmd, ok = Lookup(Default(), Chord{Ctrl: true, Key: "p"}, true)
	if !ok || cmd != "commandPalette" {
		t.Fatalf("default %q", cmd)
	}
}
