package main

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

const width, height = 1280, 820

// isolate points the config dir at a temp dir, so tests leave the real
// settings and keybindings alone.
func isolate(t *testing.T) {
	t.Helper()
	cfg := t.TempDir()
	t.Setenv("AppData", cfg)         // Windows
	t.Setenv("XDG_CONFIG_HOME", cfg) // Linux
	t.Setenv("HOME", cfg)            // macOS
}

// testApp opens a two-note vault with Roadmap.md in a tab.
func testApp(t *testing.T) *App {
	t.Helper()
	isolate(t)
	dir := t.TempDir()
	notes := map[string]string{
		"Roadmap.md": "# Roadmap\n\nShip the kit. See [[Ideas]].\n",
		"Ideas.md":   "# Ideas\n\nSome #project notes.\n",
	}
	for name, body := range notes {
		if err := os.WriteFile(filepath.Join(dir, name), []byte(body), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	app := newApp()
	app.openVault(dir)
	if app.vault == nil || app.watchCancel == nil {
		t.Fatalf("open vault: %s", app.status)
	}
	t.Cleanup(app.watchCancel)
	// catchUp saves the index last; once it exists the notes are indexed
	// and nothing writes into dir.
	gob := filepath.Join(dir, ".app", "index.gob")
	for i := 0; i < 500; i++ {
		if _, err := os.Stat(gob); err == nil {
			break
		}
		time.Sleep(10 * time.Millisecond)
	}
	app.openNote("Roadmap.md")
	return app
}

func TestWelcomeWithoutVault(t *testing.T) {
	isolate(t)
	tt := ui.NewTester(newApp().view, width, height)
	if !tt.HasText("Welcome to Nota") {
		t.Errorf("welcome shows %q", tt.Texts())
	}
}

func TestWorkspaceShowsKit(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	for _, s := range []string{"Search notes or run a command", "2 notes", "AI offline"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
}

func TestPaletteFindsNotesAndCommands(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	tt.Key(ui.Cmd, ui.KeyO)
	tt.Type("ideas")
	tt.Key(0, ui.KeyEnter)
	if app.quickOpen || app.current() == nil || app.current().path != "Ideas.md" {
		t.Fatalf("after Enter on ideas: open %v, texts %q", app.quickOpen, tt.Texts())
	}
	tt.Key(ui.Cmd, ui.KeyO)
	tt.Type("settings")
	tt.Key(0, ui.KeyEnter)
	if !app.settingsOn {
		t.Errorf("Enter on settings did not open Settings; texts %q", tt.Texts())
	}
}
