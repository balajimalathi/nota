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
