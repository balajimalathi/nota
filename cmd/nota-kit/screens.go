package main

import (
	"github.com/egoist/mygo/ui"

	"nota/internal/kit"
)

func welcome(c *ui.Context, p kit.Palette, onOpen func(dir string)) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Children(func() {
		ui.Column(c).Width(420).Gap(24).Children(func() {
			ui.Column(c).Gap(14).Children(func() {
				ui.Row(c).Size(40, 40).Radius(10).Background(p.Primary).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "N").FontSize(20).FontWeight(700).TextColor(p.PrimaryForeground)
				})
				ui.Column(c).Gap(4).Children(func() {
					ui.Text(c, "Welcome to Nota").FontSize(22).FontWeight(600)
					ui.Text(c, "Plain Markdown files, offline first. AI only when you ask.").TextColor(t.TextMuted)
				})
			})
			open := ui.PrimaryButton(c, "").AlignSelf(ui.Start).Height(34).Padding(0, 14).Gap(8).Children(func() {
				ui.Icon(c, kit.IconOpen).FontSize(15)
				ui.Text(c, "Open vault…").SingleLine()
			})
			if open.Clicked() {
				onOpen(mockRecent[0].Dir)
			}
			ui.Column(c).Gap(4).Children(func() {
				kit.SectionLabel(c, "Recent")
				ui.Column(c).Padding(4).Radius(10).Border(1, t.Border).Children(func() {
					for _, r := range mockRecent {
						if kit.NavRow(c.Key(r.Name), kit.IconFolder, r.Name, false, func() {
							ui.Text(c, r.Dir).FontSize(12).TextColor(t.TextMuted).SingleLine().MaxWidth(200)
						}).Height(36).Clicked() {
							onOpen(r.Dir)
						}
					}
				})
			})
		})
	})
}

func gallery(c *ui.Context, p kit.Palette, g *galleryState) {
	t := c.Theme()
	section := func(name string, fn func()) {
		ui.Column(c).Gap(12).Children(func() {
			ui.Text(c, name).FontSize(16).FontWeight(600)
			fn()
		})
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Row(c).Justify(ui.Center).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(920).Padding(32, 40).Gap(32).Children(func() {
				section("Colors", func() {
					ui.Row(c).Wrap().Gap(12).Children(func() {
						for _, s := range p.Swatches() {
							ui.Column(c.Key(s.Name)).Width(96).Gap(6).Children(func() {
								ui.Box(c).Height(44).Radius(8).Border(1, t.Border).Background(s.Color)
								ui.Text(c, s.Name).FontSize(11).TextColor(t.TextMuted).SingleLine()
							})
						}
					})
				})
				section("Type", func() {
					ui.Text(c, "Welcome to Nota").FontSize(22).FontWeight(600)
					ui.Text(c, "Section heading").FontSize(16).FontWeight(600)
					ui.Text(c, "Body text sits at 14 DIPs with room around it.")
					ui.Text(c, "Secondary text is muted and 12 DIPs.").FontSize(12).TextColor(t.TextMuted)
				})
				section("Buttons", func() {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.PrimaryButton(c, "Primary")
						ui.Button(c, "Outline")
						kit.IconButton(c, kit.IconSearch, "Ghost", false)
						kit.IconButton(c, kit.IconSidebar, "Ghost, on", true)
						kit.Chip(c, "#chip", "3")
						kit.SearchTrigger(c, "Search trigger", kit.Mod(), "K")
					})
				})
				section("Inputs", func() {
					ui.Column(c).Width(320).Gap(10).Children(func() {
						ui.TextInput(c, &g.text).Placeholder("Text input")
						ui.SearchField(c, &g.search)
						ui.Select(c, &g.choice, []string{"llama3.2", "qwen2.5", "mistral"})
						ui.Row(c).Gap(16).AlignItems(ui.Center).Children(func() {
							ui.Switch(c, &g.on)
							ui.Checkbox(c, &g.check, "Checkbox")
							ui.NumberInput(c, &g.num, 11, 28, 1).Width(90)
						})
					})
				})
				section("Navigation", func() {
					kit.Tabs(c, &g.tab, nil, "Source", "Live", "Reading")
					kit.Tabs(c, &g.iconTab, []*ui.SVG{kit.IconLink, kit.IconOutline, kit.IconTag, kit.IconGraph, kit.IconAI},
						"Links", "Outline", "Tags", "Graph", "Assistant")
					kit.Scoped(c, kit.Quiet(p, t.FontSize), func() {
						qt := c.Theme()
						ui.Column(c).Width(248).Padding(8).Radius(10).Background(qt.Surface).Border(1, qt.Border).Children(func() {
							kit.SectionLabel(c, "Files")
							kit.NavRow(c, kit.IconNote, "Nota roadmap", true, nil)
							kit.NavRow(c, kit.IconNote, "Sync design", false, func() { ui.Badge(c, "2") })
							kit.NavRow(c, kit.IconPlus, "New note", false, func() { kit.Kbd(c, kit.Mod(), "N") })
						})
					})
				})
				section("Feedback", func() {
					ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
						if ui.Button(c, "Show toast").Clicked() {
							c.Toast("Saved Nota roadmap.md")
						}
						ui.Spinner(c)
						ui.Progress(c, 0.6).Width(160)
						kit.Dot(c, p.Success)
						kit.Dot(c, t.TextMuted)
						kit.Dot(c, p.Destructive)
					})
					ui.Column(c).Height(180).Radius(10).Border(1, t.Border).Children(func() {
						kit.EmptyState(c, kit.IconNote, "No note open", "Pick a note on the left, or start a new one.", "New note")
					})
				})
				section("Icons", func() {
					ui.Row(c).Wrap().Gap(14).Children(func() {
						for _, ic := range kit.AllIcons() {
							ui.Icon(c.Key(ic.Name), ic.SVG).FontSize(18).TextColor(t.TextMuted).Tooltip(ic.Name)
						}
					})
				})
			})
		})
	})
}
