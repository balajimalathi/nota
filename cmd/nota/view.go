package main

import (
	"slices"
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/kit"
	"nota/internal/note"
	"nota/internal/settings"
)

func (app *App) view(c *ui.Context) {
	app.pal = kit.Apply(c, app.settings.Theme, app.settings.FontSize)
	if th := app.mdOpts.Theme; th != nil {
		th.LineHeight = 1.65
		th.Link, th.Muted, th.CodeBackground = app.pal.Link, c.Theme().TextMuted, app.pal.Muted
		if app.settings.FontSize > 0 {
			th.FontSize = app.settings.FontSize
		}
	}
	app.shortcuts(c)
	if app.vault == nil {
		app.welcome(c)
		app.modals(c)
		return
	}
	if t := app.current(); t != nil {
		app.autosave(t)
		app.caretLine.Store(int32(t.state.CursorPos().Line))
		t.state.SoftWrap = app.settings.SoftWrap
		if app.settings.FontSize > 0 {
			t.state.Font.Size = app.settings.FontSize
		}
	}
	app.graphFG = c.Theme().TextMuted
	app.graphAC = app.pal.Link
	app.graphTX = c.Theme().Text
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		app.titleBar(c)
		app.body(c)
		app.statusBar(c)
	})
	app.modals(c)
}

func (app *App) welcome(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Children(func() {
		ui.Column(c).Width(420).Gap(24).Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Row(c).Size(40, 40).Radius(10).Background(app.pal.Primary).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "N").FontSize(20).FontWeight(700).TextColor(app.pal.PrimaryForeground)
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Welcome to Nota").FontSize(22).FontWeight(600)
					ui.Text(c, "A small offline vault for Markdown notes.").TextColor(t.TextMuted)
				})
			})
			open := ui.PrimaryButton(c, "").AlignSelf(ui.Start).Height(34).Padding(0, 14).Gap(8).Children(func() {
				ui.Icon(c, kit.IconOpen).FontSize(15)
				ui.Text(c, "Open vault…").SingleLine()
			})
			if open.Clicked() {
				app.pickVault()
			}
			if len(app.settings.Recent) == 0 {
				return
			}
			ui.Column(c).Gap(4).Children(func() {
				kit.SectionLabel(c, "Recent")
				ui.Column(c).Padding(4).Radius(10).Border(1, t.Border).Children(func() {
					for _, dir := range app.settings.Recent {
						if kit.NavRow(c.Key(dir), kit.IconFolder, filepathBase(dir), false, func() {
							ui.Text(c, dir).FontSize(12).TextColor(t.TextMuted).SingleLine().MaxWidth(200)
						}).Height(36).Clicked() {
							app.openVault(dir)
						}
					}
				})
			})
		})
	})
}

func (app *App) titleBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(44).Shrink(0).Padding(0, 8).Gap(4).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		if kit.IconButton(c, kit.IconSidebar, "Toggle file tree", false).Clicked() {
			app.run("toggleSidebar")
		}
		app.breadcrumb(c)
		ui.Spacer(c)
		if kit.SearchTrigger(c, "Search notes or run a command", app.keyLabel("quickOpen")...).Clicked() {
			app.run("quickOpen")
		}
		ui.Spacer(c)
		kit.Tabs(c, &app.mode, []*ui.SVG{kit.IconSource, kit.IconLive, kit.IconRead}, "Source", "Live", "Reading")
		ui.Box(c).Width(4)
		if kit.IconButton(c, kit.IconAI, "Assistant", app.showRight && app.rightTab == 4).Clicked() {
			app.showRight, app.rightTab = true, 4
		}
		if kit.IconButton(c, kit.IconSync, "Sync", false).Clicked() {
			app.run("sync")
		}
		if kit.IconButton(c, kit.IconSettings, "Settings", false).Clicked() {
			app.run("settings")
		}
		if kit.IconButton(c, kit.IconInspector, "Toggle side pane", false).Clicked() {
			app.run("toggleRight")
		}
	})
}

func (app *App) breadcrumb(c *ui.Context) {
	cur := app.current()
	if cur == nil {
		return
	}
	t := c.Theme()
	parts := strings.Split(cur.path, "/")
	ui.Row(c).Gap(6).PaddingX(4).AlignItems(ui.Center).Children(func() {
		for i, part := range parts {
			if i > 0 {
				ui.Text(c, "/").FontSize(t.FontSize - 1).TextColor(t.TextMuted.Alpha(0.6))
			}
			if i == len(parts)-1 {
				ui.Text(c, note.TitleFromName(part)).FontSize(t.FontSize - 1).FontWeight(500).SingleLine()
				continue
			}
			ui.Text(c, part).FontSize(t.FontSize - 1).TextColor(t.TextMuted).SingleLine()
		}
	})
}

func (app *App) body(c *ui.Context) {
	main := func() {
		if app.showRight {
			ui.SplitPercent(c, &app.editorPct, func() { app.editorColumn(c) }, func() { app.rightPane(c) }).Grow(1)
			return
		}
		app.editorColumn(c)
	}
	if app.showSide {
		ui.Split(c, &app.sidebarW, func() { app.sidebar(c) }, main).Grow(1)
		return
	}
	ui.Column(c).Grow(1).Children(main)
}

func (app *App) statusBar(c *ui.Context) {
	t := c.Theme()
	small := func(s string) { ui.Text(c, s).FontSize(12).TextColor(t.TextMuted).SingleLine() }
	ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(16).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		if cur := app.current(); cur != nil {
			small(cur.path)
			small(plural(len(strings.Fields(cur.state.Buffer.String())), "word"))
		}
		ui.Spacer(c)
		if app.status != "" {
			small(app.status)
		}
		dot := t.TextMuted
		if app.aiReach == "ready" {
			dot = app.pal.Success
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			kit.Dot(c, dot)
			small("AI " + app.aiReach)
		})
		small(plural(app.index.Count(), "note"))
	})
}

func (app *App) modals(c *ui.Context) {
	app.picker(c, &app.quickOpen, "Search notes or type a command…", &app.quickQuery, &app.quickList, &app.quickSel, app.quickItems)
	app.picker(c, &app.searchOpen, "Search inside notes…", &app.searchQuery, &app.searchList, &app.searchSel, app.searchItems)
	app.nameModal(c, &app.creating, "New note", "Name", func() {
		app.createNamed(app.modalText)
		app.creating = false
	})
	app.nameModal(c, &app.renaming, "Rename note", "Name", func() {
		app.doRename(app.modalText)
		app.renaming = false
	})
	ui.Modal(c, &app.deleting, func() {
		name := "this note"
		if t := app.current(); t != nil {
			name = t.path
		}
		ui.Text(c, "Delete "+name+"?").Bold()
		ui.Text(c, "The file is removed from the vault.").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "Cancel").Clicked() {
				app.deleting = false
			}
			if ui.PrimaryButton(c, "Delete").Clicked() {
				app.doDelete()
				app.deleting = false
			}
		})
	})
	app.settingsModal(c)
	app.syncModal(c)
	app.templateModal(c)
}

type paletteItem struct {
	icon       *ui.SVG
	title, sub string
	run        func()
}

func (app *App) quickItems(q string) []paletteItem {
	var out []paletteItem
	if app.index != nil {
		for _, h := range app.index.Fuzzy(q, 30) {
			out = append(out, paletteItem{kit.IconNote, h.Title, h.Path, func() { app.openNote(h.Path) }})
		}
	}
	for _, cmd := range filterCommands(app.commands(), q) {
		out = append(out, paletteItem{kit.IconCommand, cmd.Title, "Command", func() { app.run(cmd.ID) }})
	}
	return out
}

func (app *App) searchItems(q string) []paletteItem {
	if strings.TrimSpace(q) == "" || app.index == nil {
		return nil
	}
	var out []paletteItem
	for _, h := range app.index.Search(q, 40) {
		sub := h.Path
		if h.Snippet != "" {
			sub = h.Snippet
		}
		out = append(out, paletteItem{kit.IconNote, h.Title, sub, func() { app.openNote(h.Path) }})
	}
	return out
}

// picker is the command palette: a field over items, chosen with the
// arrows and Enter. It closes before it runs the choice.
func (app *App) picker(c *ui.Context, open *bool, placeholder string, query *string, list *ui.ListState, sel *int, items func(string) []paletteItem) {
	kit.Dialog(c, open, 560, func() {
		t := c.Theme()
		its := items(*query)
		if *sel >= len(its) {
			*sel = 0
		}
		if c.Shortcut(0, ui.KeyDown) && *sel < len(its)-1 {
			*sel++
		}
		if c.Shortcut(0, ui.KeyUp) && *sel > 0 {
			*sel--
		}
		choose := func() {
			if *sel >= 0 && *sel < len(its) {
				*open = false
				its[*sel].run()
			}
		}
		ui.Row(c).Height(48).Padding(0, 14).Gap(10).AlignItems(ui.Center).
			BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
			ui.Icon(c, kit.IconSearch).FontSize(16).TextColor(t.TextMuted)
			in := ui.TextInput(c, query).Placeholder(placeholder).AutoFocus().Grow(1).
				Border(0, ui.Transparent).Background(ui.Transparent)
			if in.Submitted() {
				choose()
			}
		})
		if len(its) == 0 {
			kit.EmptyState(c, kit.IconSearch, "No matches", "Try another word.", "")
			return
		}
		quiet := *kit.Quiet(app.pal, t.FontSize)
		quiet.Surface = app.pal.Background
		kit.Scoped(c, &quiet, func() {
			list.Selected = sel
			l := ui.List(c, list, len(its), func(i int) {
				it := its[i]
				ui.Row(c).Gap(10).Padding(0, 6).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, it.icon).FontSize(15).TextColor(quiet.TextMuted)
					ui.Text(c, it.title).SingleLine().Grow(1)
					ui.Text(c, it.sub).FontSize(12).TextColor(quiet.TextMuted).SingleLine().MaxWidth(240)
				})
			}).Height(300).Padding(6)
			if l.Submitted() {
				choose()
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

func (app *App) nameModal(c *ui.Context, open *bool, title, label string, done func()) {
	ui.Modal(c, open, func() {
		ui.Text(c, title).Bold()
		ui.Text(c, label).TextColor(c.Theme().TextMuted)
		if ui.TextInput(c, &app.modalText).AutoFocus().Width(320).Submitted() {
			done()
		}
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "Cancel").Clicked() {
				*open = false
			}
			if ui.PrimaryButton(c, "OK").Clicked() {
				done()
			}
		})
	})
}

var (
	settingsSections = []string{"General", "Editor", "Assistant"}
	themeNames       = []string{"system", "light", "dark"}
)

func (app *App) settingsModal(c *ui.Context) {
	kit.Dialog(c, &app.settingsOn, 640, func() {
		ui.Row(c).Height(420).AlignItems(ui.Stretch).Children(func() {
			kit.Scoped(c, kit.Quiet(app.pal, c.Theme().FontSize), func() {
				qt := c.Theme()
				ui.Column(c).Width(170).Shrink(0).Padding(12, 8).Gap(2).Background(qt.Surface).
					BorderWidth(0, 1, 0, 0).BorderColor(qt.Border).Children(func() {
					ui.Row(c).Padding(4, 8, 10, 8).Children(func() { ui.Text(c, "Settings").FontWeight(600) })
					icons := []*ui.SVG{kit.IconSettings, kit.IconLive, kit.IconAI}
					for i, name := range settingsSections {
						if kit.NavRow(c.Key(name), icons[i], name, app.settingsSec == i, nil).Clicked() {
							app.settingsSec = i
						}
					}
				})
			})
			ui.Column(c).Grow(1).Padding(20, 24).Gap(16).Children(func() {
				ui.Text(c, settingsSections[app.settingsSec]).FontSize(16).FontWeight(600)
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Form(c, func() { app.settingsFields(c) })
				})
				ui.Row(c).Justify(ui.End).Children(func() {
					if ui.PrimaryButton(c, "Done").Clicked() {
						app.refreshClient()
						_ = settings.Save(app.settings)
						go app.pingAI()
						app.settingsOn = false
					}
				})
			})
		})
	})
}

func (app *App) settingsFields(c *ui.Context) {
	save := func() { _ = settings.Save(app.settings) }
	in := func(v *string) ui.Element { return ui.TextInput(c, v).Width(280) }
	switch app.settingsSec {
	case 0:
		ui.Field(c, "Appearance", func() {
			if name, ok := pickName(c, app.settings.Theme, themeNames, "System", "Light", "Dark"); ok {
				app.settings.Theme = name
				save()
			}
		}).Description("System follows your OS appearance.")
		ui.Field(c, "Daily note path", func() { in(&app.settings.DailyFormat) }).Description("A Go time layout, as daily/2006-01-02.")
		ui.Field(c, "Templates folder", func() { in(&app.settings.TemplatesDir) })
	case 1:
		ui.Field(c, "Font size", func() {
			f := float64(app.settings.FontSize)
			if ui.NumberInput(c, &f, 11, 28, 1).Width(100).Changed() {
				app.settings.FontSize = float32(f)
				save()
			}
		})
		ui.Field(c, "Soft wrap", func() {
			if ui.Switch(c, &app.settings.SoftWrap).Changed() {
				save()
			}
		})
	default:
		ui.Field(c, "Endpoint", func() { in(&app.settings.AIEndpoint) }).Description("Any OpenAI-compatible server. Ollama by default.")
		ui.Field(c, "Chat model", func() { in(&app.settings.AIChatModel) })
		ui.Field(c, "Embed model", func() { in(&app.settings.AIEmbedModel) })
		ui.Field(c, "API key", func() { in(&app.settings.AIAPIKey).Password() })
	}
}

// pickName shows names as kit.Tabs with labels and reports a new choice.
// A name not in names, as an old "slate", shows as the first.
func pickName(c *ui.Context, cur string, names []string, labels ...string) (string, bool) {
	sel := max(slices.Index(names, cur), 0)
	was := sel
	kit.Tabs(c, &sel, nil, labels...)
	return names[sel], sel != was
}

func (app *App) templateModal(c *ui.Context) {
	ui.Modal(c, &app.templatesOn, func() {
		ui.Text(c, "Insert template").Bold()
		if len(app.templates) == 0 {
			ui.Text(c, "No Markdown files in the templates folder.").TextColor(c.Theme().TextMuted)
		}
		for _, rel := range app.templates {
			r := rel
			if ui.Button(c, note.TitleFromName(r)).Clicked() {
				app.insertTemplate(r)
				app.templatesOn = false
			}
		}
		if ui.Button(c, "Close").Clicked() {
			app.templatesOn = false
		}
	})
}

func filterCommands(cmds []command, q string) []command {
	q = strings.ToLower(strings.TrimSpace(q))
	if q == "" {
		return cmds
	}
	var out []command
	for _, cmd := range cmds {
		if strings.Contains(strings.ToLower(cmd.Title), q) || strings.Contains(strings.ToLower(cmd.ID), q) {
			out = append(out, cmd)
		}
	}
	return out
}

func filepathBase(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return p
	}
	return p[i+1:]
}
