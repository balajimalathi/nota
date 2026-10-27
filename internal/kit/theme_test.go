package kit

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

func TestPick(t *testing.T) {
	cases := []struct {
		mode      string
		sys, dark bool
	}{
		{"light", true, false},
		{"dark", false, true},
		{"system", true, true},
		{"system", false, false},
		{"", true, true},
	}
	for _, tc := range cases {
		if got := Pick(tc.mode, tc.sys).Dark; got != tc.dark {
			t.Errorf("Pick(%q, %v).Dark = %v, want %v", tc.mode, tc.sys, got, tc.dark)
		}
	}
}

func TestThemeUsesPalette(t *testing.T) {
	for _, p := range []Palette{Light(), Dark()} {
		th := Theme(p, 15)
		if th.Dark != p.Dark || th.Background != p.Background || th.Surface != p.Background ||
			th.Accent != p.Primary || th.AccentText != p.PrimaryForeground || th.Border != p.Border ||
			th.TextMuted != p.MutedForeground || th.Radius != 8 || th.FontSize != 15 {
			t.Errorf("Theme(dark=%v) = %+v", p.Dark, *th)
		}
	}
}

func TestQuietChoosesInGray(t *testing.T) {
	p := Light()
	q := Quiet(p, 0)
	if q.Accent != p.SidebarSelected || q.AccentText != p.Foreground || q.Surface != p.Sidebar {
		t.Errorf("Quiet accent %v, text %v, surface %v", q.Accent, q.AccentText, q.Surface)
	}
}

func TestApplyFollowsSystem(t *testing.T) {
	var got Palette
	tt := ui.NewTester(func(c *ui.Context) {
		got = Apply(c, "system", 0)
		ui.Text(c, "x")
	}, 200, 100)
	tt.Frame()
	if got.Dark {
		t.Fatal("a light system gave the dark palette")
	}
	tt.SetDark(true)
	tt.Frame()
	if !got.Dark {
		t.Error("a dark system gave the light palette")
	}
}

func TestScopedRestores(t *testing.T) {
	var inside, after ui.Color
	q := Quiet(Light(), 0)
	tt := ui.NewTester(func(c *ui.Context) {
		Apply(c, "light", 0)
		Scoped(c, q, func() { inside = c.Theme().Accent })
		after = c.Theme().Accent
	}, 200, 100)
	tt.Frame()
	if inside != Light().SidebarSelected || after != Light().Primary {
		t.Errorf("inside %v, after %v", inside, after)
	}
}
