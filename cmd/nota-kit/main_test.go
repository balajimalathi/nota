package main

import (
	"image/png"
	"os"
	"path/filepath"
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

func TestScreens(t *testing.T) {
	p := newPreview()
	tt := ui.NewTester(p.view, width, height)
	if err := tt.Click("Welcome"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Welcome to Nota") {
		t.Errorf("welcome shows %q", tt.Texts())
	}
	if err := tt.Click("Kit"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("Colors") || !tt.HasText("primary-foreground") {
		t.Errorf("kit shows %q", tt.Texts())
	}
}

// TestSnapshots writes a PNG of each screen at 2x for review when
// NOTA_KIT_SHOTS names a folder, relative to the repository root.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("NOTA_KIT_SHOTS")
	if dir == "" {
		t.Skip("set NOTA_KIT_SHOTS=docs/design to write the screens")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join("..", "..", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	shots := []struct {
		name  string
		setup func(p *preview)
	}{
		{"workspace-light", func(p *preview) { p.ws.set.theme = 1 }},
		{"workspace-dark", func(p *preview) { p.ws.set.theme = 2 }},
		{"reading-light", func(p *preview) { p.ws.set.theme = 1; p.ws.mode = modeRead }},
		{"assistant-dark", func(p *preview) { p.ws.set.theme = 2; p.ws.pane = paneAI }},
		{"palette-light", func(p *preview) { p.ws.set.theme = 1; p.ws.palette = true }},
		{"settings-light", func(p *preview) { p.ws.set.theme = 1; p.ws.settings = true }},
		{"welcome-light", func(p *preview) { p.ws.set.theme = 1; p.screen = screenWelcome }},
		{"kit-light", func(p *preview) { p.ws.set.theme = 1; p.screen = screenKit }},
		{"kit-dark", func(p *preview) { p.ws.set.theme = 2; p.screen = screenKit }},
	}
	for _, s := range shots {
		p := newPreview()
		s.setup(p)
		tt := ui.NewTester(p.view, width, height)
		tt.SetScale(2)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, s.name+".png"))
		if err != nil {
			t.Fatal(err)
		}
		if err := png.Encode(f, tt.Image()); err != nil {
			t.Fatal(err)
		}
		f.Close()
	}
}
