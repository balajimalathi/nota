package main

import (
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"

	"nota/internal/graph"
	"nota/internal/md"
	"nota/internal/note"
)

func (app *App) sidebar(c *ui.Context) {
	ui.Column(c).Fill().Background(c.Theme().Surface).Children(func() {
		ui.Row(c).Padding(8, 10).Children(func() {
			ui.Text(c, "Files").Bold()
		})
		app.outline.List.Selected = &app.fileRow
		ui.Outline(c, &app.outline, app.roots, app.childPaths, func(path string) {
			label := note.TitleFromName(path)
			if app.dirs[path] {
				label = note.TitleFromName(path)
			}
			row := ui.Row(c).Grow(1).Gap(6)
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
				ui.Text(c, label).SingleLine()
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
}

func (app *App) rightPane(c *ui.Context) {
	ui.Column(c).Fill().Padding(8).Gap(8).Children(func() {
		ui.Tabs(c, &app.rightTab, "Links", "Outline", "Tags", "Graph", "AI")
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

func (app *App) linksPane(c *ui.Context) {
	t := app.current()
	if t == nil {
		ui.Text(c, "Open a note to see its links.").TextColor(c.Theme().TextMuted)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(8).Children(func() {
			ui.Text(c, "Backlinks").Bold()
			backs := app.index.Backlinks(t.path)
			if len(backs) == 0 {
				ui.Text(c, "None").TextColor(c.Theme().TextMuted)
			}
			for _, h := range backs {
				hit := h
				if ui.Button(c, hit.Title).Clicked() {
					app.openNote(hit.Path)
				}
			}
			out, missing := app.index.Outgoing(t.path)
			ui.Text(c, "Outgoing").Bold()
			if len(out) == 0 && len(missing) == 0 {
				ui.Text(c, "None").TextColor(c.Theme().TextMuted)
			}
			for _, h := range out {
				hit := h
				if ui.Button(c, hit.Title).Clicked() {
					app.openNote(hit.Path)
				}
			}
			for _, name := range missing {
				n := name
				ui.Text(c, "Unresolved "+n).TextColor(c.Theme().TextMuted)
			}
			ui.Text(c, "Related").Bold()
			if ui.Button(c, "Find related").Clicked() {
				app.findRelated()
			}
			for _, h := range app.related {
				hit := h
				if ui.Button(c, note.TitleFromName(hit.Path)).Clicked() {
					app.openNote(hit.Path)
				}
			}
		})
	})
}

func (app *App) outlinePane(c *ui.Context) {
	t := app.current()
	if t == nil {
		ui.Text(c, "No note").TextColor(c.Theme().TextMuted)
		return
	}
	doc := app.cachedDoc(t)
	if len(doc.Headings) == 0 {
		ui.Text(c, "No headings").TextColor(c.Theme().TextMuted)
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(4).Children(func() {
			for _, h := range doc.Headings {
				head := h
				label := strings.Repeat("  ", head.Level-1) + head.Text
				if ui.Button(c, label).Clicked() {
					t.state.GoTo(editor.Pos{Line: head.Line, Col: 0})
				}
			}
		})
	})
}

func (app *App) tagsPane(c *ui.Context) {
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(6).Children(func() {
			if app.tagFilter != "" {
				ui.Text(c, "#"+app.tagFilter).Bold()
				if ui.Button(c, "Clear").Clicked() {
					app.tagFilter = ""
				}
				for _, h := range app.index.Tagged(app.tagFilter) {
					hit := h
					if ui.Button(c, hit.Title).Clicked() {
						app.openNote(hit.Path)
					}
				}
				return
			}
			tags := app.index.Tags()
			if len(tags) == 0 {
				ui.Text(c, "No tags").TextColor(c.Theme().TextMuted)
				return
			}
			for _, tag := range tags {
				tg := tag
				if ui.Button(c, "#"+tg.Tag+"  "+itoa(tg.Count)).Clicked() {
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
		ui.Text(c, "Open a note to see the notes around it.").TextColor(c.Theme().TextMuted)
		return
	}
	lay := graph.Layout(nodes, edges)
	box := ui.Box(c).Grow(1).Fill()
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
