package main

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

const width, height = 1280, 820

func TestWorkspaceShowsShell(t *testing.T) {
	p := newPreview()
	tt := ui.NewTester(p.view, width, height)
	for _, s := range []string{"Field notes", "Files", "Tags", "Backlinks", "Synced", "Search notes or run a command"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
}

func TestClosingLastTabShowsEmptyState(t *testing.T) {
	p := newPreview()
	p.ws.tabs = p.ws.tabs[:1]
	tt := ui.NewTester(p.view, width, height)
	if err := tt.Click("Close Nota roadmap"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("No note open") {
		t.Errorf("after closing the last tab: %q", tt.Texts())
	}
}

func TestPaletteFindsNotes(t *testing.T) {
	p := newPreview()
	tt := ui.NewTester(p.view, width, height)
	tt.Key(ui.Cmd, ui.KeyK)
	if !p.ws.palette {
		t.Fatal("Ctrl+K did not open the palette")
	}
	tt.Type("local")
	if !tt.HasText("Local-first software") {
		t.Errorf("palette shows %q", tt.Texts())
	}
	tt.Key(0, ui.KeyEnter)
	if p.ws.palette || p.ws.current() != "reading/Local-first software.md" {
		t.Errorf("after Enter: palette open %v, current %q", p.ws.palette, p.ws.current())
	}
}

func TestPaletteArrowsMoveSelection(t *testing.T) {
	p := newPreview()
	tt := ui.NewTester(p.view, width, height)
	tt.Key(ui.Cmd, ui.KeyK)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyDown)
	tt.Key(0, ui.KeyUp)
	if p.ws.paletteSel != 1 {
		t.Errorf("selection %d after down, down, up; want 1", p.ws.paletteSel)
	}
}

func TestSettingsOpens(t *testing.T) {
	p := newPreview()
	tt := ui.NewTester(p.view, width, height)
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if !p.ws.settings || !tt.HasText("Appearance") {
		t.Errorf("settings open %v, texts %q", p.ws.settings, tt.Texts())
	}
}
