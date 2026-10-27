package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nota/internal/graph"
	"nota/internal/kit"
	"nota/internal/md"
	"nota/internal/note"
)

func (app *App) sidebar(c *ui.Context) {
	kit.Scoped(c, kit.Quiet(app.pal, c.Theme().FontSize), func() {
		t := c.Theme()
		ui.Column(c).Fill().Padding(8).Gap(2).Background(t.Surface).Children(func() {
			app.vaultSwitcher(c)
			if kit.NavRow(c, kit.IconPlus, "New note", false, func() { kit.Kbd(c, app.keyLabel("newNote")...) }).Clicked() {
				app.run("newNote")
			}
			if kit.NavRow(c, kit.IconDaily, "Today", false, nil).Clicked() {
				app.run("dailyNote")
			}
			kit.SectionLabel(c, "Files")
			app.outline.List.Selected = &app.fileRow
			ui.Outline(c, &app.outline, app.roots, app.childPaths, func(path string) {
				icon := kit.IconNote
				if app.dirs[path] {
					icon = kit.IconFolder
				}
				row := ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center)
				row.ContextMenu(func(m *ui.Menu) {
					dir := path
					if !app.dirs[path] {
						dir = parentPath(path)
					}
					if m.Item("New note").Chosen() {
						app.newDir = dir
						app.modalText = ""
						app.creating = true
					}
					if !app.dirs[path] && m.Item("Delete").Chosen() {
						app.openNote(path)
						app.deleting = true
					}
				})
				row.Children(func() {
					ui.Icon(c, icon).FontSize(15).TextColor(t.TextMuted)
					ui.Text(c, note.TitleFromName(path)).SingleLine()
				})
			}).Grow(1)
			if app.fileRow >= 0 && app.fileRow < app.outline.Rows() {
				item := app.outline.Item(app.fileRow)
				if item != app.lastTree && !app.dirs[item] {
					app.lastTree = item
					app.openNote(item)
				}
			}
		})
	})
}

func (app *App) vaultSwitcher(c *ui.Context) {
	t := c.Theme()
	name := filepathBase(app.vault.Root())
	b := ui.ButtonBase(c).AlignSelf(ui.Stretch).Justify(ui.Start).Height(44).Padding(0, 8).Gap(10).Radius(8).
		Tooltip("Open another vault")
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Size(28, 28).Radius(7).Background(app.pal.Primary).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, initial(name)).FontWeight(700).TextColor(app.pal.PrimaryForeground)
		})
		ui.Column(c).Grow(1).MinWidth(0).Gap(1).Children(func() {
			ui.Text(c, name).FontWeight(600).SingleLine()
			ui.Text(c, plural(app.index.Count(), "note")).FontSize(12).TextColor(t.TextMuted)
		})
		ui.Icon(c, kit.IconChevronDown).FontSize(14).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		app.pickVault()
	}
}

// initial is the vault mark's letter. A drive root has an empty base.
func initial(s string) string {
	for _, r := range s {
		return strings.ToUpper(string(r))
	}
	return "N"
}

func (app *App) rightPane(c *ui.Context) {
	ui.Column(c).Fill().Padding(12).Gap(8).Children(func() {
		kit.Tabs(c, &app.rightTab, []*ui.SVG{kit.IconLink, kit.IconOutline, kit.IconTag, kit.IconGraph, kit.IconAI},
			"Links", "Outline", "Tags", "Graph", "Assistant").AlignSelf(ui.Stretch)
		switch app.rightTab {
		case 0:
			app.linksPane(c)
		case 1:
			app.outlinePane(c)
		case 2:
			app.tagsPane(c)
		case 3:
			app.graphPane(c)
		default:
			app.aiPane(c)
		}
	})
}

func hint(c *ui.Context, s string) {
	ui.Row(c).Padding(4, 8).Children(func() {
		ui.Text(c, s).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func (app *App) linksPane(c *ui.Context) {
	t := app.current()
	if t == nil {
		kit.EmptyState(c, kit.IconLink, "No note open", "Links to and from a note show here.", "")
		return
	}
	row := func(group, path, title string) {
		if kit.NavRow(c.Key(group+path), kit.IconNote, title, false, nil).Clicked() {
			app.openNote(path)
		}
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			kit.SectionLabel(c, "Backlinks")
			backs := app.index.Backlinks(t.path)
			if len(backs) == 0 {
				hint(c, "None yet")
			}
			for _, h := range backs {
				row("back:", h.Path, h.Title)
			}
			out, missing := app.index.Outgoing(t.path)
			kit.SectionLabel(c, "Outgoing")
			if len(out) == 0 && len(missing) == 0 {
				hint(c, "None yet")
			}
			for _, h := range out {
				row("out:", h.Path, h.Title)
			}
			for _, name := range missing {
				hint(c, "Unresolved "+name)
			}
			kit.SectionLabel(c, "Related")
			if kit.NavRow(c, kit.IconAI, "Find related", false, nil).Clicked() {
				app.findRelated()
			}
			for _, h := range app.related {
				row("rel:", h.Path, note.TitleFromName(h.Path))
			}
		})
	})
}

func (app *App) outlinePane(c *ui.Context) {
	t := app.current()
	if t == nil {
		kit.EmptyState(c, kit.IconOutline, "No note open", "Headings of the open note show here.", "")
		return
	}
	doc := app.cachedDoc(t)
	if len(doc.Headings) == 0 {
		kit.EmptyState(c, kit.IconOutline, "No headings", "Lines starting with # show here.", "")
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			for i, h := range doc.Headings {
				ui.Row(c.Key(i)).PaddingStart(float32(h.Level-1) * 12).Children(func() {
					if kit.NavRow(c, nil, h.Text, false, nil).Grow(1).Clicked() {
						t.state.GoTo(editor.Pos{Line: h.Line, Col: 0})
					}
				})
			}
		})
	})
}

func (app *App) tagsPane(c *ui.Context) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			if app.tagFilter != "" {
				if kit.NavRow(c, kit.IconClose, "#"+app.tagFilter, true, nil).Tooltip("Clear filter").Clicked() {
					app.tagFilter = ""
				}
				for _, h := range app.index.Tagged(app.tagFilter) {
					if kit.NavRow(c.Key(h.Path), kit.IconNote, h.Title, false, nil).Clicked() {
						app.openNote(h.Path)
					}
				}
				return
			}
			tags := app.index.Tags()
			if len(tags) == 0 {
				kit.EmptyState(c, kit.IconTag, "No tags", "Write #tag in a note to group it.", "")
				return
			}
			kit.SectionLabel(c, "All tags")
			for _, tg := range tags {
				if kit.NavRow(c.Key(tg.Tag), kit.IconTag, tg.Tag, false, func() { ui.Badge(c, itoa(tg.Count)) }).Clicked() {
					app.tagFilter = tg.Tag
				}
			}
		})
	})
}

func (app *App) graphPane(c *ui.Context) {
	path := ""
	if t := app.current(); t != nil {
		path = t.path
	}
	nodes, edges := app.index.Neighborhood(path, 2)
	if len(nodes) == 0 {
		kit.EmptyState(c, kit.IconGraph, "No note open", "Notes around the open note show here.", "")
		return
	}
	lay := graph.Layout(nodes, edges)
	box := ui.Box(c).Grow(1).Fill().Radius(8).Border(1, c.Theme().Border)
	box.Draw(func(p *ui.Painter, r ui.Rect) {
		for _, e := range edges {
			a, aok := lay[e[0]]
			b, bok := lay[e[1]]
			if !aok || !bok {
				continue
			}
			p.Line(
				r.X+float32(a.X)*r.W, r.Y+float32(a.Y)*r.H,
				r.X+float32(b.X)*r.W, r.Y+float32(b.Y)*r.H,
				1, app.graphFG,
			)
		}
		app.graphHits = app.graphHits[:0]
		for _, id := range nodes {
			pt := lay[id]
			x := float32(pt.X) * r.W
			y := float32(pt.Y) * r.H
			col := app.graphFG
			if id == path {
				col = app.graphAC
			}
			p.Fill(ui.Rect{X: r.X + x - 5, Y: r.Y + y - 5, W: 10, H: 10}, col, 5)
			p.Text(r.X+x+8, r.Y+y+4, note.TitleFromName(id), 12, app.graphTX)
			app.graphHits = append(app.graphHits, ghit{path: id, x: x, y: y})
		}
	})
	if box.Clicked() {
		x, y, ok := box.PointerPosition()
		if ok {
			app.pickGraph(x, y)
		}
	}
}

func (app *App) pickGraph(x, y float32) {
	var best string
	bestD := float32(18 * 18)
	for _, h := range app.graphHits {
		dx, dy := h.x-x, h.y-y
		if d := dx*dx + dy*dy; d < bestD {
			bestD = d
			best = h.path
		}
	}
	if best != "" {
		app.openNote(best)
	}
}

func (app *App) cachedDoc(t *noteTab) md.Document {
	ver := t.state.Buffer.Version()
	if app.docPath == t.path && app.docVer == ver {
		return app.docCache
	}
	app.docCache = md.Parse(t.state.Buffer.String())
	app.docPath = t.path
	app.docVer = ver
	return app.docCache
}

func parentPath(p string) string {
	i := strings.LastIndex(p, "/")
	if i < 0 {
		return ""
	}
	return p[:i]
}
