package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/kit"
	"nota/internal/note"
)

type paletteItem struct {
	icon       *ui.SVG
	title, sub string
	path       string // "" for a command
}

func (w *workspace) paletteItems() []paletteItem {
	q := strings.ToLower(strings.TrimSpace(w.query))
	var out []paletteItem
	for _, path := range mockPaths() {
		if t := note.TitleFromName(path); q == "" || strings.Contains(strings.ToLower(t), q) {
			out = append(out, paletteItem{kit.IconNote, t, path, path})
		}
	}
	for _, cmd := range mockCommands {
		if q == "" || strings.Contains(strings.ToLower(cmd), q) {
			out = append(out, paletteItem{kit.IconCommand, cmd, "Command", ""})
		}
	}
	return out
}

func (w *workspace) choose(c *ui.Context, it paletteItem) {
	w.palette = false
	if it.path != "" {
		w.open(it.path)
		return
	}
	c.Toast(it.title)
}

// commandPalette is one dialog for notes and commands, as shadcn's Command.
func (w *workspace) commandPalette(c *ui.Context, p kit.Palette) {
	kit.Dialog(c, &w.palette, 560, func() {
		t := c.Theme()
		items := w.paletteItems()
		if w.paletteSel >= len(items) {
			w.paletteSel = 0
		}
		if c.Shortcut(0, ui.KeyDown) && w.paletteSel < len(items)-1 {
			w.paletteSel++
		}
		if c.Shortcut(0, ui.KeyUp) && w.paletteSel > 0 {
			w.paletteSel--
		}
		ui.Row(c).Height(48).Padding(0, 14).Gap(10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Icon(c, kit.IconSearch).FontSize(16).TextColor(t.TextMuted)
			in := ui.TextInput(c, &w.query).Placeholder("Search notes or type a command…").AutoFocus().Grow(1).
				Border(0, ui.Transparent).Background(ui.Transparent)
			if in.Submitted() && len(items) > 0 {
				w.choose(c, items[w.paletteSel])
			}
		})
		if len(items) == 0 {
			kit.EmptyState(c, kit.IconSearch, "No matches", "Try another word.", "")
			return
		}
		quiet := *kit.Quiet(p, t.FontSize)
		quiet.Surface = p.Background
		kit.Scoped(c, &quiet, func() {
			w.paletteList.Selected = &w.paletteSel
			list := ui.List(c, &w.paletteList, len(items), func(i int) {
				it := items[i]
				ui.Row(c).Gap(10).Padding(0, 6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, it.icon).FontSize(15).TextColor(quiet.TextMuted)
					ui.Text(c, it.title).SingleLine().Grow(1)
					ui.Text(c, it.sub).FontSize(12).TextColor(quiet.TextMuted).SingleLine()
				})
			}).Height(300).Padding(6)
			if list.Submitted() && w.paletteSel < len(items) {
				w.choose(c, items[w.paletteSel])
			}
		})
		ui.Row(c).Height(36).Padding(0, 14).Gap(14).AlignItems(ui.Center).
			BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
			keyHint := func(label string, keys ...string) {
				ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
					kit.Kbd(c, keys...)
					ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
				})
			}
			keyHint("navigate", "↑", "↓")
			keyHint("open", "Enter")
			keyHint("close", "Esc")
		})
	})
}

var settingsSections = []string{"General", "Editor", "Assistant", "Sync"}

func (w *workspace) settingsDialog(c *ui.Context, p kit.Palette) {
	kit.Dialog(c, &w.settings, 680, func() {
		ui.Row(c).Height(440).AlignItems(ui.Stretch).Children(func() {
			kit.Scoped(c, kit.Quiet(p, c.Theme().FontSize), func() {
				qt := c.Theme()
				ui.Column(c).Width(180).Shrink(0).Padding(12, 8).Gap(2).Background(qt.Surface).
					BorderWidth(0, 1, 0, 0).BorderColor(qt.Border).Children(func() {
					ui.Row(c).Padding(4, 8, 10, 8).Children(func() { ui.Text(c, "Settings").FontWeight(600) })
					icons := []*ui.SVG{kit.IconSettings, kit.IconLive, kit.IconAI, kit.IconSync}
					for i, name := range settingsSections {
						if kit.NavRow(c.Key(name), icons[i], name, w.settingsSec == i, nil).Clicked() {
							w.settingsSec = i
						}
					}
				})
			})
			ui.Column(c).Grow(1).Padding(20, 24).Gap(16).Children(func() {
				ui.Text(c, settingsSections[w.settingsSec]).FontSize(16).FontWeight(600)
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Form(c, func() { w.settingsFields(c) })
				})
				ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
					if ui.Button(c, "Cancel").Clicked() {
						w.settings = false
					}
					if ui.PrimaryButton(c, "Done").Clicked() {
						w.settings = false
						c.Toast("Settings saved")
					}
				})
			})
		})
	})
}

func (w *workspace) settingsFields(c *ui.Context) {
	in := func(v *string) { ui.TextInput(c, v).Width(280) }
	switch w.settingsSec {
	case 0:
		ui.Field(c, "Appearance", func() { kit.Tabs(c, &w.set.theme, nil, "System", "Light", "Dark") }).
			Description("System follows your OS appearance.")
		ui.Field(c, "Daily note path", func() { in(&w.set.daily) }).Description("A Go time layout, as daily/2006-01-02.")
		ui.Field(c, "Templates folder", func() { in(&w.set.templates) })
	case 1:
		ui.Field(c, "Font size", func() { ui.NumberInput(c, &w.set.font, 11, 28, 1).Width(100) })
		ui.Field(c, "Soft wrap", func() { ui.Switch(c, &w.set.wrap) })
	case 2:
		ui.Field(c, "Endpoint", func() { in(&w.set.endpoint) }).Description("Any OpenAI-compatible server. Ollama by default.")
		ui.Field(c, "Chat model", func() { in(&w.set.chatModel) })
		ui.Field(c, "Embed model", func() { in(&w.set.embedModel) })
		ui.Field(c, "API key", func() { ui.TextInput(c, &w.set.apiKey).Width(280).Password() })
	default:
		ui.Field(c, "Method", func() { kit.Tabs(c, &w.set.syncMode, nil, "Off", "Git", "Folder", "HTTP") })
		ui.Field(c, "Remote", func() { in(&w.set.remote) }).Description("A folder, or http://host:8765 for nota serve.")
		ui.Field(c, "Token", func() { ui.TextInput(c, &w.set.token).Width(280).Password() })
	}
}
