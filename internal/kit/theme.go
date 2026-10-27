package kit

import "github.com/egoist/mygo/ui"

// Theme returns MyGo's theme painted with p: controls are outlined on the
// background, as shadcn's outline buttons and inputs, and primary buttons,
// checks, switches, tooltips and toasts take the primary color. A
// fontSize of 0 keeps the system's size.
func Theme(p Palette, fontSize float32) *ui.Theme {
	t := ui.LightTheme()
	if p.Dark {
		t = ui.DarkTheme()
	}
	t.Background = p.Background
	t.Surface = p.Background
	t.SurfaceHover = p.Hover
	t.SurfacePressed = p.Pressed
	t.Border = p.Border
	t.Text = p.Foreground
	t.TextMuted = p.MutedForeground
	t.Accent = p.Primary
	t.AccentHover = p.PrimaryHover
	t.AccentPressed = p.PrimaryPressed
	t.AccentText = p.PrimaryForeground
	t.Danger = p.Destructive
	t.Warning = p.Warning
	t.Success = p.Success
	t.Selection = p.Selection
	t.Focus = p.Ring
	t.Inverse = p.Primary
	t.InverseText = p.PrimaryForeground
	t.Scrollbar = p.Scrollbar
	t.Radius = 8
	t.Spacing = 4
	if fontSize > 0 {
		t.FontSize = fontSize
	}
	return t
}

// Quiet returns the theme sidebars and lists are built in: their face is
// the sidebar's color, and the row chosen is a soft gray with the
// foreground on it instead of the primary color.
func Quiet(p Palette, fontSize float32) *ui.Theme {
	t := Theme(p, fontSize)
	t.Surface = p.Sidebar
	t.SurfaceHover = p.Hover
	t.SurfacePressed = p.SidebarSelected
	t.Accent = p.SidebarSelected
	t.AccentHover = p.SidebarSelected
	t.AccentPressed = p.SidebarSelected
	t.AccentText = p.Foreground
	return t
}

// Apply sets the frame's theme from mode and returns its palette. Call it
// first in a view: every frame starts from the system's theme, which tells
// whether the system is dark.
func Apply(c *ui.Context, mode string, fontSize float32) Palette {
	p := Pick(mode, c.Theme().Dark)
	c.SetTheme(Theme(p, fontSize))
	return p
}

// Scoped builds fn with t as the theme, then puts back the one before, so
// that only the widgets fn creates take t.
func Scoped(c *ui.Context, t *ui.Theme, fn func()) {
	prev := c.Theme()
	c.SetTheme(t)
	fn()
	c.SetTheme(prev)
}
