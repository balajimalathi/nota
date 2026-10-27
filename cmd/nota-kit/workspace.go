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

const (
	modeSource = iota
	modeLive
	modeRead
)

const (
	paneLinks = iota
	paneOutline
	paneTags
	paneGraph
	paneAI
)

// prefs stands in for settings.Settings.
type prefs struct {
	theme                           int // 0 system, 1 light, 2 dark
	font                            float64
	wrap                            bool
	daily, templates                string
	endpoint, chatModel, embedModel string
	apiKey                          string
	syncMode                        int
	remote, token                   string
}

func (p prefs) themeName() string { return [...]string{"system", "light", "dark"}[p.theme] }

// workspace is the main screen: the state the real App keeps, filled with
// mock notes.
type workspace struct {
	set prefs

	sideOpen, inspOpen bool
	tree               ui.OutlineState[string]
	fileRow            int
	lastRow            string

	tabs   []string
	active int
	states map[string]*editor.State
	mode   int
	pane   int

	palette     bool
	query       string
	paletteList ui.ListState
	paletteSel  int

	settings    bool
	settingsSec int

	aiInput string
	mdOpts  markdown.Options
	mdTheme markdown.Theme
}

func newWorkspace() *workspace {
	return &workspace{
		set: prefs{font: 14, wrap: true, daily: "daily/2006-01-02", templates: "templates",
			endpoint: "http://localhost:11434/v1", chatModel: "llama3.2", embedModel: "nomic-embed-text"},
		sideOpen: true, inspOpen: true, fileRow: -1,
		tabs:   []string{"projects/Nota roadmap.md", "projects/Sync design.md", "daily/2026-10-28.md"},
		states: map[string]*editor.State{},
	}
}

func (w *workspace) current() string {
	if w.active < 0 || w.active >= len(w.tabs) {
		return ""
	}
	return w.tabs[w.active]
}

func (w *workspace) open(path string) {
	if mockDirs[path] {
		return
	}
	for i, t := range w.tabs {
		if t == path {
			w.active = i
			return
		}
	}
	w.tabs = append(w.tabs, path)
	w.active = len(w.tabs) - 1
}

func (w *workspace) close(i int) {
	w.tabs = append(w.tabs[:i], w.tabs[i+1:]...)
	if w.active >= len(w.tabs) {
		w.active = len(w.tabs) - 1
	}
	if w.active < 0 {
		w.active = 0
	}
}

func (w *workspace) openPalette() { w.palette, w.query, w.paletteSel = true, "", 0 }

func (w *workspace) view(c *ui.Context, p kit.Palette) {
	if c.Shortcut(ui.Cmd, ui.KeyK) || c.Shortcut(ui.Cmd, ui.KeyP) || c.Shortcut(ui.Cmd, ui.KeyO) {
		w.openPalette()
	}
	if c.Shortcut(ui.Cmd, ui.KeyBackslash) {
		w.sideOpen = !w.sideOpen
	}
	if c.Shortcut(ui.Cmd, ui.KeyE) {
		w.mode = (w.mode + 1) % 3
	}
	w.styleMarkdown(c, p)
	ui.Column(c).Grow(1).Children(func() {
		w.titleBar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			if w.sideOpen {
				w.sidebar(c, p)
			}
			w.center(c, p)
			if w.inspOpen {
				w.inspector(c, p)
			}
		})
		w.statusBar(c, p)
	})
}

func (w *workspace) styleMarkdown(c *ui.Context, p kit.Palette) {
	w.mdTheme = markdown.Theme{
		FontSize: float32(w.set.font) + 1, LineHeight: 1.65,
		Link: p.Link, Muted: c.Theme().TextMuted, CodeBackground: p.Muted,
		Mark: ui.RGBA(250, 204, 21, 0.4),
	}
	w.mdOpts.WrapCode = true
	w.mdOpts.OnLink = func(string) {}
	w.mdOpts.Theme = &w.mdTheme
}

func (w *workspace) titleBar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(44).Shrink(0).Padding(0, 8).Gap(4).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		if kit.IconButton(c, kit.IconSidebar, "Toggle sidebar", false).Clicked() {
			w.sideOpen = !w.sideOpen
		}
		w.breadcrumb(c)
		ui.Spacer(c)
		if kit.SearchTrigger(c, "Search notes or run a command").Clicked() {
			w.openPalette()
		}
		ui.Spacer(c)
		kit.Tabs(c, &w.mode, []*ui.SVG{kit.IconSource, kit.IconLive, kit.IconRead}, "Source", "Live", "Reading")
		ui.Box(c).Width(4)
		if kit.IconButton(c, kit.IconAI, "Assistant", w.inspOpen && w.pane == paneAI).Clicked() {
			w.inspOpen, w.pane = true, paneAI
		}
		if kit.IconButton(c, kit.IconSync, "Sync", false).Clicked() {
			c.Toast("Synced with git · nothing waiting")
		}
		if kit.IconButton(c, kit.IconSettings, "Settings", false).Clicked() {
			w.settings = true
		}
		if kit.IconButton(c, kit.IconInspector, "Toggle side panel", false).Clicked() {
			w.inspOpen = !w.inspOpen
		}
	})
}

func (w *workspace) breadcrumb(c *ui.Context) {
	t := c.Theme()
	path := w.current()
	if path == "" {
		return
	}
	parts := strings.Split(path, "/")
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

func (w *workspace) sidebar(c *ui.Context, p kit.Palette) {
	kit.Scoped(c, kit.Quiet(p, c.Theme().FontSize), func() {
		t := c.Theme()
		ui.Column(c).Width(248).Shrink(0).Padding(8).Gap(2).Background(t.Surface).
			BorderWidth(0, 1, 0, 0).BorderColor(t.Border).Children(func() {
			w.vaultSwitcher(c, p)
			if kit.NavRow(c, kit.IconPlus, "New note", false, func() { kit.Kbd(c, kit.Mod(), "N") }).Clicked() {
				c.Toast("New note")
			}
			if kit.NavRow(c, kit.IconDaily, "Today", false, nil).Clicked() {
				w.open("daily/2026-10-28.md")
			}
			kit.SectionLabel(c, "Files")
			w.tree.List.Selected = &w.fileRow
			ui.Outline(c, &w.tree, mockRoots, mockChildren, func(path string) {
				icon := kit.IconNote
				if mockDirs[path] {
					icon = kit.IconFolder
				}
				ui.Row(c).Grow(1).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Icon(c, icon).FontSize(15).TextColor(t.TextMuted)
					ui.Text(c, note.TitleFromName(path)).SingleLine()
				})
			}).Grow(1)
			if w.fileRow >= 0 && w.fileRow < w.tree.Rows() {
				if item := w.tree.Item(w.fileRow); item != w.lastRow {
					w.lastRow = item
					w.open(item)
				}
			}
			kit.SectionLabel(c, "Tags")
			ui.Row(c).Wrap().Gap(6).Padding(0, 8, 8, 8).Children(func() {
				for _, tg := range mockTags {
					if kit.Chip(c.Key(tg.Tag), "#"+tg.Tag, strconv.Itoa(tg.N)).Clicked() {
						w.inspOpen, w.pane = true, paneTags
					}
				}
			})
		})
	})
}

func (w *workspace) vaultSwitcher(c *ui.Context, p kit.Palette) {
	t := c.Theme()
	b := ui.ButtonBase(c).AlignSelf(ui.Stretch).Justify(ui.Start).Height(44).Padding(0, 8).Gap(10).Radius(8)
	if b.Hovered() {
		b.Background(t.SurfaceHover)
	}
	b.Children(func() {
		ui.Row(c).Size(28, 28).Radius(7).Background(p.Primary).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "F").FontWeight(700).TextColor(p.PrimaryForeground)
		})
		ui.Column(c).Grow(1).Gap(1).Children(func() {
			ui.Text(c, mockVault).FontWeight(600).SingleLine()
			ui.Text(c, strconv.Itoa(len(mockNotes))+" notes").FontSize(12).TextColor(t.TextMuted)
		})
		ui.Icon(c, kit.IconChevronDown).FontSize(14).TextColor(t.TextMuted)
	})
	if b.Clicked() {
		c.Toast("Open another vault…")
	}
}

func (w *workspace) center(c *ui.Context, p kit.Palette) {
	ui.Column(c).Grow(1).MinWidth(0).Children(func() {
		w.tabStrip(c)
		path := w.current()
		switch {
		case path == "":
			if kit.EmptyState(c, kit.IconNote, "No note open", "Pick a note on the left, or start a new one.", "New note") {
				c.Toast("New note")
			}
		case w.mode == modeRead:
			w.reading(c, path)
		default:
			w.source(c, p, path)
		}
	})
}

func (w *workspace) tabStrip(c *ui.Context) {
	t := c.Theme()
	closeAt := -1
	ui.Row(c).Height(40).Shrink(0).Padding(0, 8).Gap(2).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		for i, path := range w.tabs {
			on := i == w.active
			name := note.TitleFromName(path)
			tab := ui.Row(c.Key(path)).Height(28).Padding(0, 4, 0, 10).Gap(4).Radius(6).AlignItems(ui.Center)
			hover := tab.Hovered()
			if on || hover {
				tab.Background(t.SurfaceHover)
			}
			tab.Children(func() {
				label := ui.ButtonBase(c).Children(func() {
					txt := ui.Text(c, name).FontSize(t.FontSize - 1).SingleLine().MaxWidth(180)
					if !on {
						txt.TextColor(t.TextMuted)
					}
				})
				if label.Clicked() {
					w.active = i
				}
				if on || hover {
					if kit.IconButton(c, kit.IconClose, "Close "+name, false).Size(20, 20).Clicked() {
						closeAt = i
					}
				} else {
					ui.Box(c).Size(20, 20)
				}
			})
		}
	})
	if closeAt >= 0 {
		w.close(closeAt)
	}
}

// source is the editor, in a centered column with no gutter.
func (w *workspace) source(c *ui.Context, p kit.Palette, path string) {
	st := w.states[path]
	if st == nil {
		buf := editor.NewBuffer(mockNotes[path])
		buf.SetLanguage(editor.LanguageFor(path))
		st = &editor.State{Buffer: buf, LineNumbers: editor.LineNumbersOff, NoFolding: true, Placeholder: "Start writing…"}
		w.states[path] = st
	}
	st.SoftWrap = w.set.wrap
	st.LineHeight = 1.6
	st.Font.Size = float32(w.set.font)
	th := editor.DefaultTheme(c.Theme())
	th.ActiveLine = ui.Transparent
	th.Tokens[editor.TokenLink].Color = p.Link
	if w.mode == modeLive {
		th.Tokens[editor.TokenMeta].Color = c.Theme().TextMuted.Alpha(0.45)
	}
	st.Theme = th
	ui.Row(c.Key(path)).Grow(1).Justify(ui.Center).Padding(36, 24, 0, 24).Children(func() {
		editor.View(c, st).Grow(1).MaxWidth(720)
	})
}

func (w *workspace) reading(c *ui.Context, path string) {
	src := md.ForPreview(mockNotes[path], noEmbed)
	ui.Scroll(c.Key(path)).Grow(1).Children(func() {
		ui.Row(c).Justify(ui.Center).Padding(40, 24).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(720).Children(func() {
				markdown.View(c, src, &w.mdOpts)
			})
		})
	})
}

func (w *workspace) statusBar(c *ui.Context, p kit.Palette) {
	t := c.Theme()
	small := func(s string) { ui.Text(c, s).FontSize(12).TextColor(t.TextMuted).SingleLine() }
	ui.Row(c).Height(28).Shrink(0).Padding(0, 12).Gap(16).AlignItems(ui.Center).
		BorderWidth(1, 0, 0, 0).BorderColor(t.Border).Children(func() {
		if path := w.current(); path != "" {
			small(path)
			small(strconv.Itoa(len(strings.Fields(mockNotes[path]))) + " words")
		}
		ui.Spacer(c)
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() { kit.Dot(c, p.Success); small("Synced") })
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() { kit.Dot(c, p.Success); small("Ollama ready") })
		small(strconv.Itoa(len(mockNotes)) + " notes")
	})
}
