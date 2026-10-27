package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/index"
	"nota/internal/note"
	"nota/internal/settings"
)

func (app *App) view(c *ui.Context) {
	app.applyTheme(c)
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
	app.graphAC = c.Theme().Accent
	app.graphTX = c.Theme().Text
	ui.Column(c).Fill().Background(c.Theme().Background).Children(func() {
		app.toolbar(c)
		app.body(c)
		app.footer(c)
	})
	app.modals(c)
}

func (app *App) applyTheme(c *ui.Context) {
	var th *ui.Theme
	switch app.settings.Theme {
	case "light":
		th = ui.LightTheme()
	case "dark":
		th = ui.DarkTheme()
	case "slate":
		th = slateTheme()
	}
	if th != nil {
		if app.settings.FontSize > 0 {
			th.FontSize = app.settings.FontSize
		}
		c.SetTheme(th)
	}
	if app.mdOpts.Theme != nil && app.settings.FontSize > 0 {
		app.mdOpts.Theme.FontSize = app.settings.FontSize
	}
}

func (app *App) welcome(c *ui.Context) {
	ui.Column(c).Fill().Center().Gap(12).Children(func() {
		ui.Text(c, "Nota").FontSize(36).Bold()
		ui.Text(c, "A small offline vault for Markdown notes.").TextColor(c.Theme().TextMuted)
		if ui.PrimaryButton(c, "Open vault").Clicked() {
			app.pickVault()
		}
		if len(app.settings.Recent) > 0 {
			ui.Text(c, "Recent").TextColor(c.Theme().TextMuted)
			for _, path := range app.settings.Recent {
				p := path
				if ui.Button(c, p).Clicked() {
					app.openVault(p)
				}
			}
		}
	})
}

func (app *App) toolbar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Gap(6).Padding(6, 8).AlignItems(ui.Center).Background(t.Surface).Children(func() {
		ui.Text(c, filepathBase(app.vault.Root())).Bold()
		if ui.Button(c, "New").Clicked() {
			app.run("newNote")
		}
		if ui.Button(c, "Open").Clicked() {
			app.pickVault()
		}
		if ui.Button(c, "Search").Clicked() {
			app.run("search")
		}
		if ui.Button(c, modeLabel(app.mode)).Clicked() {
			app.run("cycleMode")
		}
		if ui.Button(c, "AI").Clicked() {
			app.showRight = true
			app.rightTab = 4
		}
		ui.Box(c).Grow(1)
		ui.Text(c, app.aiReach).TextColor(t.TextMuted)
		if ui.Button(c, "Sync").Clicked() {
			app.run("sync")
		}
		if ui.Button(c, "Settings").Clicked() {
			app.settingsOn = true
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

func (app *App) footer(c *ui.Context) {
	t := c.Theme()
	var bits []string
	if cur := app.current(); cur != nil {
		bits = append(bits, cur.path)
		bits = append(bits, plural(len(strings.Fields(cur.state.Buffer.String())), "word"))
	}
	bits = append(bits, plural(app.index.Count(), "note"))
	if app.status != "" {
		bits = append(bits, app.status)
	}
	ui.Row(c).Padding(4, 8).Background(t.Surface).Children(func() {
		ui.Text(c, strings.Join(bits, "  ·  ")).TextColor(t.TextMuted).FontSize(t.FontSize - 1)
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

func modeLabel(mode int) string {
	switch mode {
	case modeLive:
		return "Live"
	case modeRead:
		return "Reading"
	default:
		return "Source"
	}
}

func filepathBase(p string) string {
	i := strings.LastIndexAny(p, `/\`)
	if i < 0 {
		return p
	}
	return p[i+1:]
}
