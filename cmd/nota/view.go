package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/index"
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
	app.picker(c, &app.quickOpen, "Quick open", &app.quickQuery, &app.quickList, &app.quickSel, func(q string) []index.Hit {
		if app.index == nil {
			return nil
		}
		return app.index.Fuzzy(q, 30)
	}, func(h index.Hit) { app.openNote(h.Path) })
	app.commandPicker(c)
	app.picker(c, &app.searchOpen, "Search notes", &app.searchQuery, &app.searchList, &app.searchSel, func(q string) []index.Hit {
		if strings.TrimSpace(q) == "" || app.index == nil {
			return nil
		}
		return app.index.Search(q, 40)
	}, func(h index.Hit) { app.openNote(h.Path) })
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

func (app *App) picker(c *ui.Context, open *bool, title string, query *string, list *ui.ListState, sel *int, search func(string) []index.Hit, choose func(index.Hit)) {
	ui.Modal(c, open, func() {
		ui.Text(c, title).Bold()
		hits := search(*query)
		if ui.SearchField(c, query).AutoFocus().Width(420).Submitted() && len(hits) > 0 {
			choose(hits[0])
			*open = false
		}
		if len(hits) == 0 {
			ui.Text(c, "No matches").TextColor(c.Theme().TextMuted)
			return
		}
		if *sel >= len(hits) {
			*sel = 0
		}
		list.Selected = sel
		if ui.List(c, list, len(hits), func(i int) {
			ui.Column(c).Gap(2).Children(func() {
				ui.Text(c, hits[i].Title).Bold()
				sub := hits[i].Path
				if hits[i].Snippet != "" {
					sub = hits[i].Snippet
				}
				ui.Text(c, sub).TextColor(c.Theme().TextMuted).FontSize(c.Theme().FontSize - 1)
			})
		}).Height(320).Width(420).Submitted() {
			if *sel >= 0 && *sel < len(hits) {
				choose(hits[*sel])
				*open = false
			}
		}
	})
}

func (app *App) commandPicker(c *ui.Context) {
	ui.Modal(c, &app.palette, func() {
		ui.Text(c, "Commands").Bold()
		cmds := filterCommands(app.commands(), app.paletteQuery)
		if ui.SearchField(c, &app.paletteQuery).AutoFocus().Width(420).Submitted() && len(cmds) > 0 {
			app.palette = false
			app.run(cmds[0].ID)
		}
		if len(cmds) == 0 {
			ui.Text(c, "No commands").TextColor(c.Theme().TextMuted)
			return
		}
		if app.paletteSel >= len(cmds) {
			app.paletteSel = 0
		}
		app.paletteList.Selected = &app.paletteSel
		if ui.List(c, &app.paletteList, len(cmds), func(i int) {
			ui.Text(c, cmds[i].Title)
		}).Height(320).Width(420).Submitted() {
			if app.paletteSel >= 0 && app.paletteSel < len(cmds) {
				id := cmds[app.paletteSel].ID
				app.palette = false
				app.run(id)
			}
		}
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

func (app *App) settingsModal(c *ui.Context) {
	ui.Modal(c, &app.settingsOn, func() {
		ui.Text(c, "Settings").Bold()
		ui.Text(c, "Theme").TextColor(c.Theme().TextMuted)
		ui.Row(c).Gap(6).Children(func() {
			for _, name := range []string{"system", "light", "dark", "slate"} {
				n := name
				label := n
				if n == app.settings.Theme {
					label = "● " + n
				}
				if ui.Button(c, label).Clicked() {
					app.settings.Theme = n
					_ = settings.Save(app.settings)
				}
			}
		})
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "Font "+itoa(int(app.settings.FontSize)))
			if ui.Button(c, "−").Clicked() && app.settings.FontSize > 11 {
				app.settings.FontSize--
				_ = settings.Save(app.settings)
			}
			if ui.Button(c, "+").Clicked() && app.settings.FontSize < 28 {
				app.settings.FontSize++
				_ = settings.Save(app.settings)
			}
		})
		if ui.Toggle(c, &app.settings.SoftWrap, "Soft wrap").Clicked() {
			_ = settings.Save(app.settings)
		}
		ui.Text(c, "AI endpoint").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.AIEndpoint).Width(360)
		ui.Text(c, "Chat model").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.AIChatModel).Width(360)
		ui.Text(c, "Embed model").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.AIEmbedModel).Width(360)
		ui.Text(c, "API key").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.AIAPIKey).Width(360)
		ui.Text(c, "Daily note path").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.DailyFormat).Width(360)
		ui.Text(c, "Templates folder").TextColor(c.Theme().TextMuted)
		ui.TextInput(c, &app.settings.TemplatesDir).Width(360)
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.PrimaryButton(c, "Done").Clicked() {
				app.refreshClient()
				_ = settings.Save(app.settings)
				go app.pingAI()
				app.settingsOn = false
			}
		})
	})
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
