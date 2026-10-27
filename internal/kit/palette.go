// Package kit is Nota's look: shadcn/ui's zinc palette mapped onto MyGo's
// theme, Lucide icons, and the few parts every screen is built from.
package kit

import "github.com/egoist/mygo/ui"

// Palette holds the colors a screen paints with, named as shadcn/ui names
// them. Link is the one hue, kept for wikilinks, tags and the graph.
type Palette struct {
	Dark              bool
	Background        ui.Color
	Foreground        ui.Color
	Muted             ui.Color
	MutedForeground   ui.Color
	Border            ui.Color
	Primary           ui.Color
	PrimaryHover      ui.Color
	PrimaryPressed    ui.Color
	PrimaryForeground ui.Color
	Hover             ui.Color
	Pressed           ui.Color
	Ring              ui.Color
	Destructive       ui.Color
	Success           ui.Color
	Warning           ui.Color
	Link              ui.Color
	Selection         ui.Color
	Sidebar           ui.Color
	SidebarSelected   ui.Color
	Scrollbar         ui.Color
}

// Light is shadcn's zinc theme for a light appearance.
func Light() Palette {
	return Palette{
		Background:        ui.Hex("#ffffff"),
		Foreground:        ui.Hex("#09090b"),
		Muted:             ui.Hex("#f4f4f5"),
		MutedForeground:   ui.Hex("#71717a"),
		Border:            ui.Hex("#e4e4e7"),
		Primary:           ui.Hex("#18181b"),
		PrimaryHover:      ui.Hex("#27272a"),
		PrimaryPressed:    ui.Hex("#3f3f46"),
		PrimaryForeground: ui.Hex("#fafafa"),
		Hover:             ui.Hex("#f4f4f5"),
		Pressed:           ui.Hex("#e4e4e7"),
		Ring:              ui.Hex("#a1a1aa"),
		Destructive:       ui.Hex("#ef4444"),
		Success:           ui.Hex("#16a34a"),
		Warning:           ui.Hex("#d97706"),
		Link:              ui.Hex("#4f46e5"),
		Selection:         ui.RGBA(99, 102, 241, 0.22),
		Sidebar:           ui.Hex("#fafafa"),
		SidebarSelected:   ui.Hex("#e4e4e7"),
		Scrollbar:         ui.RGBA(0, 0, 0, 0.18),
	}
}

// Dark is shadcn's zinc theme for a dark appearance.
func Dark() Palette {
	return Palette{
		Dark:              true,
		Background:        ui.Hex("#09090b"),
		Foreground:        ui.Hex("#fafafa"),
		Muted:             ui.Hex("#27272a"),
		MutedForeground:   ui.Hex("#a1a1aa"),
		Border:            ui.Hex("#27272a"),
		Primary:           ui.Hex("#fafafa"),
		PrimaryHover:      ui.Hex("#e4e4e7"),
		PrimaryPressed:    ui.Hex("#d4d4d8"),
		PrimaryForeground: ui.Hex("#18181b"),
		Hover:             ui.Hex("#27272a"),
		Pressed:           ui.Hex("#3f3f46"),
		Ring:              ui.Hex("#52525b"),
		Destructive:       ui.Hex("#f87171"),
		Success:           ui.Hex("#22c55e"),
		Warning:           ui.Hex("#f59e0b"),
		Link:              ui.Hex("#818cf8"),
		Selection:         ui.RGBA(129, 140, 248, 0.30),
		Sidebar:           ui.Hex("#18181b"),
		SidebarSelected:   ui.Hex("#3f3f46"),
		Scrollbar:         ui.RGBA(255, 255, 255, 0.16),
	}
}

// Pick returns the palette for mode, "system", "light" or "dark",
// following the system's appearance for "system" and anything else.
func Pick(mode string, systemDark bool) Palette {
	switch mode {
	case "light":
		return Light()
	case "dark":
		return Dark()
	}
	if systemDark {
		return Dark()
	}
	return Light()
}

// Swatch is a palette color with its token name, for the kit's gallery.
type Swatch struct {
	Name  string
	Color ui.Color
}

// Swatches lists p's colors in the order the gallery shows them.
func (p Palette) Swatches() []Swatch {
	return []Swatch{
		{"background", p.Background}, {"foreground", p.Foreground},
		{"muted", p.Muted}, {"muted-foreground", p.MutedForeground},
		{"border", p.Border}, {"primary", p.Primary},
		{"primary-foreground", p.PrimaryForeground}, {"hover", p.Hover},
		{"pressed", p.Pressed}, {"ring", p.Ring}, {"link", p.Link},
		{"destructive", p.Destructive}, {"success", p.Success},
		{"warning", p.Warning}, {"sidebar", p.Sidebar},
		{"sidebar-selected", p.SidebarSelected},
	}
}
