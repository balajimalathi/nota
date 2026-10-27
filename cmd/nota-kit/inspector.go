package main

import (
	"strconv"
	"strings"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"

	"nota/internal/kit"
	"nota/internal/md"
	"nota/internal/note"
)

func (w *workspace) inspector(c *ui.Context, p kit.Palette) {
	t := c.Theme()
	ui.Column(c).Width(300).Shrink(0).Padding(12).Gap(8).BorderWidth(0, 0, 0, 1).BorderColor(t.Border).Children(func() {
		kit.Tabs(c, &w.pane, []*ui.SVG{kit.IconLink, kit.IconOutline, kit.IconTag, kit.IconGraph, kit.IconAI},
			"Links", "Outline", "Tags", "Graph", "Assistant").AlignSelf(ui.Stretch)
		path := w.current()
		switch w.pane {
		case paneLinks:
			w.linksPane(c, path)
		case paneOutline:
			w.outlinePane(c, path)
		case paneTags:
			w.tagsPane(c)
		case paneGraph:
			w.graphPane(c, p, path)
		default:
			w.aiPane(c, p)
		}
	})
}

func hint(c *ui.Context, s string) {
	ui.Row(c).Padding(4, 8).Children(func() {
		ui.Text(c, s).FontSize(12).TextColor(c.Theme().TextMuted)
	})
}

func (w *workspace) linksPane(c *ui.Context, path string) {
	if path == "" {
		kit.EmptyState(c, kit.IconLink, "No note open", "Links to and from a note show here.", "")
		return
	}
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(2).Children(func() {
			group := func(name string, paths []string) {
				kit.SectionLabel(c, name)
				if len(paths) == 0 {
					hint(c, "None yet")
				}
				for _, q := range paths {
					if kit.NavRow(c.Key(name+q), kit.IconNote, note.TitleFromName(q), false, nil).Clicked() {
						w.open(q)
					}
				}
			}
			group("Backlinks", mockBacklinks[path])
			group("Outgoing", mockOutgoing[path])
		})
	})
}

func (w *workspace) outlinePane(c *ui.Context, path string) {
	doc := md.Parse(mockNotes[path])
	if len(doc.Headings) == 0 {
		kit.EmptyState(c, kit.IconOutline, "No headings", "Lines starting with # show here.", "")
		return
	}
	ui.Column(c).Gap(2).Children(func() {
		for i, h := range doc.Headings {
			ui.Row(c.Key(i)).PaddingStart(float32(h.Level-1) * 12).Children(func() {
				if kit.NavRow(c, nil, h.Text, false, nil).Grow(1).Clicked() {
					if st := w.states[path]; st != nil {
						st.GoTo(editor.Pos{Line: h.Line})
					}
				}
			})
		}
	})
}

func (w *workspace) tagsPane(c *ui.Context) {
	ui.Column(c).Gap(2).Children(func() {
		kit.SectionLabel(c, "All tags")
		for _, tg := range mockTags {
			kit.NavRow(c.Key(tg.Tag), kit.IconTag, tg.Tag, false, func() {
				ui.Badge(c, strconv.Itoa(tg.N))
			})
		}
	})
}

func (w *workspace) graphPane(c *ui.Context, p kit.Palette, path string) {
	t := c.Theme()
	edge, dot, label := t.Border, t.TextMuted, t.Text
	box := ui.Box(c).Grow(1).Fill().Radius(8).Border(1, t.Border)
	box.Draw(func(pt *ui.Painter, r ui.Rect) {
		at := func(id string) (float32, float32) {
			v := mockGraph[id]
			return r.X + v[0]*r.W, r.Y + v[1]*r.H
		}
		for _, e := range mockEdges {
			x1, y1 := at(e[0])
			x2, y2 := at(e[1])
			pt.Line(x1, y1, x2, y2, 1, edge)
		}
		for id := range mockGraph {
			x, y := at(id)
			col := dot
			if id == path {
				col = p.Link
			}
			pt.Fill(ui.Rect{X: x - 4, Y: y - 4, W: 8, H: 8}, col, 4)
			pt.Text(x+8, y+4, note.TitleFromName(id), 11, label)
		}
	})
}

func (w *workspace) aiPane(c *ui.Context, p kit.Palette) {
	t := c.Theme()
	ui.Row(c).Wrap().Gap(6).Children(func() {
		for _, a := range []string{"Summarize", "Rewrite", "Continue", "Title & tags"} {
			if kit.Chip(c.Key(a), a, "").Clicked() {
				c.Toast(a + "…")
			}
		}
	})
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Gap(12).PaddingY(8).Children(func() {
			for i, m := range mockChat {
				if m.Role == "user" {
					ui.Row(c.Key(i)).Justify(ui.End).Children(func() {
						ui.Row(c).Padding(6, 10).Radius(10).Background(t.SurfaceHover).MaxWidth(220).Children(func() {
							ui.Text(c, m.Text)
						})
					})
					continue
				}
				ui.Row(c.Key(i)).Gap(8).Children(func() {
					ui.Icon(c, kit.IconAI).FontSize(14).TextColor(p.Link)
					ui.Column(c).Grow(1).MinWidth(0).Children(func() {
						markdown.View(c, md.ForPreview(m.Text, noEmbed), &w.mdOpts)
					})
				})
			}
		})
	})
	ui.Row(c).Gap(6).AlignItems(ui.Center).Padding(4).Radius(10).Border(1, t.Border).Children(func() {
		in := ui.TextInput(c, &w.aiInput).Placeholder("Ask about this note…").Grow(1).
			Border(0, ui.Transparent).Background(ui.Transparent)
		send := ui.PrimaryButton(c, "").Size(28, 28).Padding(0).Radius(8).Label("Send").Children(func() {
			ui.Icon(c, kit.IconSend).FontSize(15)
		})
		if (in.Submitted() || send.Clicked()) && strings.TrimSpace(w.aiInput) != "" {
			c.Toast("Asked: " + w.aiInput)
			w.aiInput = ""
		}
	})
	ui.Text(c, "Ollama · llama3.2 · answers stay on this device").FontSize(11).TextColor(t.TextMuted)
}
