package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestIconButtonClicks(t *testing.T) {
	n := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c, "light", 0)
		if IconButton(c, IconSearch, "Search", false).Clicked() {
			n++
		}
	}, 200, 100)
	if err := tt.Click("Search"); err != nil {
		t.Fatal(err)
	}
	if n != 1 {
		t.Errorf("clicked %d times, want 1", n)
	}
}

func TestTabsChoose(t *testing.T) {
	sel := 0
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c, "light", 0)
		Tabs(c, &sel, nil, "Source", "Reading")
	}, 300, 100)
	if err := tt.Click("Reading"); err != nil {
		t.Fatal(err)
	}
	if sel != 1 {
		t.Errorf("selected %d, want 1", sel)
	}
}

func TestNavRowShowsTrail(t *testing.T) {
	clicked := false
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c, "light", 0)
		if NavRow(c, IconPlus, "New note", false, func() { Kbd(c, "Ctrl", "N") }).Clicked() {
			clicked = true
		}
	}, 300, 100)
	if !tt.HasText("New note") || !tt.HasText("Ctrl") {
		t.Fatalf("texts %q", tt.Texts())
	}
	if err := tt.Click("New note"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Error("NavRow did not report the click")
	}
}
