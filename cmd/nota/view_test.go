package main

import (
	"image/png"
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

func TestSidebarAndSidePane(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	for _, s := range []string{"New note", "Today", "Backlinks", "Outgoing"} {
		if !tt.HasText(s) {
			t.Errorf("no %q in %q", s, tt.Texts())
		}
	}
	if err := tt.Click("Tags"); err != nil {
		t.Fatal(err)
	}
	if !tt.HasText("All tags") || !tt.HasText("project") {
		t.Errorf("tags pane shows %q", tt.Texts())
	}
}

func TestClosingLastTabShowsEmptyState(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	if err := tt.Click("Close Roadmap"); err != nil {
		t.Fatal(err)
	}
	if len(app.tabs) != 0 || !tt.HasText("Pick a note on the left") {
		t.Errorf("tabs %d, texts %q", len(app.tabs), tt.Texts())
	}
}

func TestAssistantPane(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	if err := tt.Click("Assistant"); err != nil {
		t.Fatal(err)
	}
	if app.rightTab != 4 || !tt.HasText("Title & tags") {
		t.Errorf("tab %d, texts %q", app.rightTab, tt.Texts())
	}
}

func TestSettingsPicksTheme(t *testing.T) {
	app := testApp(t)
	tt := ui.NewTester(app.view, width, height)
	if err := tt.Click("Settings"); err != nil {
		t.Fatal(err)
	}
	if !app.settingsOn || !tt.HasText("Appearance") {
		t.Fatalf("settings open %v, texts %q", app.settingsOn, tt.Texts())
	}
	if err := tt.Click("Dark"); err != nil {
		t.Fatal(err)
	}
	if app.settings.Theme != "dark" {
		t.Errorf("theme %q after clicking Dark", app.settings.Theme)
	}
}

// TestSnapshots writes the app at 2x for review when NOTA_SHOTS names a
// folder, relative to the repository root.
func TestSnapshots(t *testing.T) {
	dir := os.Getenv("NOTA_SHOTS")
	if dir == "" {
		t.Skip("set NOTA_SHOTS=docs/design to write the screens")
	}
	if !filepath.IsAbs(dir) {
		dir = filepath.Join("..", "..", dir)
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	for _, theme := range []string{"light", "dark"} {
		app := testApp(t)
		app.settings.Theme = theme
		tt := ui.NewTester(app.view, width, height)
		tt.SetScale(2)
		tt.Frame()
		f, err := os.Create(filepath.Join(dir, "app-"+theme+".png"))
		if err != nil {
			t.Fatal(err)
		}
		err = png.Encode(f, tt.Image())
		f.Close()
		if err != nil {
			t.Fatal(err)
		}
	}
}
