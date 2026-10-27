package kit

import (
	"runtime"

	"github.com/egoist/mygo/ui"
)

// Mod is the name of the key shortcuts use: ⌘ on macOS, Ctrl elsewhere.
func Mod() string {
	if runtime.GOOS == "darwin" {
		return "⌘"
	}
	return "Ctrl"
}

// IconButton is a ghost button showing icon alone: no face until hovered,
// a soft gray while pressed or while on is true. label is its tooltip and
// its name for assistive technology and tests.
func IconButton(c *ui.Context, icon *ui.SVG, label string, on bool) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Size(28, 28).Radius(6).Label(label).Tooltip(label).TextColor(t.TextMuted)
	switch {
	case on || b.Pressed():
		b.Background(t.SurfacePressed).TextColor(t.Text)
	case b.Hovered():
		b.Background(t.SurfaceHover).TextColor(t.Text)
	}
	b.Children(func() { ui.Icon(c, icon).FontSize(16) })
	return b
}

// NavRow is a row of a sidebar, an inspector or a list: icon (nil for
// none), label, and what trail builds at its end, as a Badge or a Kbd (nil
// for nothing). It is gray while hovered, and a stronger gray with the
// full text color while selected.
func NavRow(c *ui.Context, icon *ui.SVG, label string, selected bool, trail func()) ui.Element {
	t := c.Theme()
	r := ui.ButtonBase(c).AlignSelf(ui.Stretch).Justify(ui.Start).Gap(8).Height(30).Padding(0, 8).Radius(6).TextColor(t.TextMuted)
	switch {
	case selected:
		r.Background(t.SurfacePressed).TextColor(t.Text)
	case r.Hovered():
		r.Background(t.SurfaceHover).TextColor(t.Text)
	}
	r.Children(func() {
		if icon != nil {
			ui.Icon(c, icon).FontSize(15)
		}
		ui.Text(c, label).SingleLine().Grow(1)
		if trail != nil {
			trail()
		}
	})
	return r
}

// SectionLabel is the small muted heading over a group, as shadcn's
// sidebar group label.
func SectionLabel(c *ui.Context, s string) ui.Element {
	t := c.Theme()
	return ui.Row(c).Padding(14, 8, 4, 8).Children(func() {
		ui.Text(c, s).FontSize(12).FontWeight(500).TextColor(t.TextMuted)
	})
}

// Kbd shows the keys of a shortcut as small outlined caps: Kbd(c, Mod(), "K").
func Kbd(c *ui.Context, keys ...string) ui.Element {
	t := c.Theme()
	return ui.Row(c).Gap(3).Shrink(0).Children(func() {
		for _, k := range keys {
			ui.Row(c).Height(18).Padding(0, 5).Radius(4).AlignItems(ui.Center).
				Border(1, t.Border).Background(t.Background).Children(func() {
				ui.Text(c, k).FontSize(11).TextColor(t.TextMuted)
			})
		}
	})
}

// Dot is a 6 DIP status dot: Success for ready, TextMuted for off.
func Dot(c *ui.Context, col ui.Color) ui.Element {
	return ui.Box(c).Size(6, 6).Radius(3).Background(col).Shrink(0)
}

// Tabs is shadcn's tabs list: a muted track whose chosen segment is raised
// on the background. With icons, they show in place of the labels, which
// become tooltips. Segments share the track's width when it is stretched.
func Tabs(c *ui.Context, selected *int, icons []*ui.SVG, labels ...string) ui.Element {
	t := c.Theme()
	seg := ui.SegmentedBase(c, selected, len(labels))
	seg.Track.Shrink(0).Padding(3).Gap(2).Radius(8).Background(t.SurfaceHover).Children(func() {
		for i, label := range labels {
			// Text tabs size to their label. Icon tabs share a stretched
			// track; a zero basis would collapse them when the track is not
			// given a width.
			s := seg.Segment(i).Shrink(0).Height(26).Padding(0, 10).Radius(6).
				Justify(ui.Center).AlignItems(ui.Center).Label(label).TextColor(t.TextMuted)
			if icons != nil {
				s.Grow(1).MinWidth(28)
			}
			if i == *selected {
				s.Background(t.Background).TextColor(t.Text).Shadow(0, 1, 2, 0, ui.RGBA(0, 0, 0, 0.08))
			} else if s.Hovered() {
				s.TextColor(t.Text)
			}
			if icons != nil {
				s.Tooltip(label)
			}
			s.Children(func() {
				if icons != nil {
					ui.Icon(c, icons[i]).FontSize(15)
					return
				}
				ui.Text(c, label).FontSize(t.FontSize - 1).FontWeight(500).SingleLine()
			})
		}
	})
	return seg.Track
}

// SearchTrigger is the quiet, wide field that opens the command palette:
// an icon, placeholder text and the shortcut. It is a button; ask Clicked.
func SearchTrigger(c *ui.Context, placeholder string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Width(340).Height(30).Padding(0, 6, 0, 10).Gap(8).Radius(8).
		Border(1, t.Border).Background(t.Background).TextColor(t.TextMuted).Label(placeholder)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Icon(c, IconSearch).FontSize(14)
		ui.Text(c, placeholder).FontSize(t.FontSize - 1).SingleLine().Grow(1)
		Kbd(c, Mod(), "K")
	})
	return b
}

// Dialog is a modal panel in the kit's look, width DIPs wide: a hairline
// border, 12 DIP corners and a soft shadow over a lightly dimmed window.
// fn builds its content edge to edge; pad inside it.
func Dialog(c *ui.Context, open *bool, width float32, fn func()) ui.Element {
	t := c.Theme()
	return ui.DialogBase(c, open, func(backdrop, panel ui.Element) {
		backdrop.Background(ui.RGBA(0, 0, 0, 0.3))
		panel.Width(width).Radius(12).Border(1, t.Border).Background(t.Background).
			Shadow(0, 16, 40, 0, ui.RGBA(0, 0, 0, 0.16)).Clip()
		fn()
	})
}

// EmptyState centers an icon, a title, a hint and, unless action is "",
// a button. It reports whether the button was clicked.
func EmptyState(c *ui.Context, icon *ui.SVG, title, hint, action string) bool {
	t := c.Theme()
	clicked := false
	ui.Column(c).Grow(1).Center().Gap(6).Padding(24).Children(func() {
		ui.Icon(c, icon).FontSize(22).TextColor(t.TextMuted)
		ui.Text(c, title).FontWeight(600)
		ui.Text(c, hint).FontSize(t.FontSize - 1).TextColor(t.TextMuted)
		if action != "" && ui.Button(c, action).Clicked() {
			clicked = true
		}
	})
	return clicked
}

// Chip is a small outlined pill, as "#project 4", that reports Clicked.
func Chip(c *ui.Context, label, count string) ui.Element {
	t := c.Theme()
	b := ui.ButtonBase(c).Height(24).Padding(0, 8).Gap(6).Radius(999).Border(1, t.Border).TextColor(t.Text)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Text(c, label).FontSize(12)
		if count != "" {
			ui.Text(c, count).FontSize(12).TextColor(t.TextMuted)
		}
	})
	return b
}
