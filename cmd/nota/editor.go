package main

import (
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/egoist/mygo/ui"
	"github.com/egoist/mygo/ui/editor"
	"github.com/egoist/mygo/ui/markdown"

	"nota/internal/kit"
	"nota/internal/md"
	"nota/internal/note"
)

func (app *App) editorColumn(c *ui.Context) {
	ui.Column(c).Grow(1).Fill().MinWidth(0).Children(func() {
		app.tabStrip(c)
		t := app.current()
		switch {
		case t == nil:
			if kit.EmptyState(c, kit.IconNote, "No note open", "Pick a note on the left, or start a new one.", "New note") {
				app.run("newNote")
			}
		case app.mode == modeRead:
			app.reading(c)
		default:
			th := editor.DefaultTheme(c.Theme())
			th.ActiveLine = ui.Transparent
			th.Tokens[editor.TokenHeading].Color = c.Theme().Text
			th.Tokens[editor.TokenLink].Color = app.pal.Link
			if app.mode == modeLive {
				th.Tokens[editor.TokenMeta] = editor.TokenStyle{Color: th.Background}
			}
			t.state.Theme = th
			// A column, so Grow gives the editor a height. The same key as a tab
			// would make the two one element, and the note would not show.
			ui.Column(c.Key("edit:"+t.path)).Grow(1).AlignItems(ui.Center).Padding(36, 24, 48, 24).Children(func() {
				editor.View(c, t.state).Grow(1).FillWidth().MaxWidth(720)
			})
		}
	})
}

func (app *App) tabStrip(c *ui.Context) {
	t := c.Theme()
	closeAt := -1
	ui.Row(c).Height(40).Shrink(0).Padding(0, 8).Gap(2).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).Children(func() {
		for i, tab := range app.tabs {
			on := i == app.active
			name := note.TitleFromName(tab.path)
			row := ui.Row(c.Key("tab:"+tab.path)).Height(28).Padding(0, 4, 0, 10).Gap(4).Radius(6).AlignItems(ui.Center)
			hover := row.Hovered()
			if on || hover {
				row.Background(t.SurfaceHover)
			}
			row.Children(func() {
				label := ui.ButtonBase(c).Children(func() {
					txt := ui.Text(c, name).FontSize(t.FontSize - 1).SingleLine().MaxWidth(180)
					if !on {
						txt.TextColor(t.TextMuted)
					}
				})
				if label.Clicked() {
					app.active = i
				}
				if on || hover {
					if kit.IconButton(c, kit.IconClose, "Close "+name, false).Size(20, 20).Clicked() {
						closeAt = i
					}
					return
				}
				ui.Row(c).Size(20, 20).Justify(ui.Center).AlignItems(ui.Center).Children(func() {
					if tab.state.Buffer.Modified() {
						kit.Dot(c, t.TextMuted)
					}
				})
			})
		}
	})
	if closeAt >= 0 {
		app.closeTab(closeAt)
	}
}

func (app *App) reading(c *ui.Context) {
	t := app.current()
	ver := t.state.Buffer.Version()
	if app.previewFor != t.path || app.previewVer != ver {
		src := t.state.Buffer.String()
		app.preview = md.ForPreview(src, func(target string) (string, bool) {
			p, ok := app.index.Resolve(target)
			if !ok || p == t.path {
				return "", false
			}
			b, err := app.vault.Read(p)
			if err != nil {
				return "", false
			}
			return string(b), true
		})
		app.previewFor = t.path
		app.previewVer = ver
	}
	ui.Scroll(c.Key("read:" + t.path)).Grow(1).Children(func() {
		ui.Row(c).Justify(ui.Center).Padding(40, 24).Children(func() {
			ui.Column(c).Grow(1).MaxWidth(720).Children(func() {
				markdown.View(c, app.preview, &app.mdOpts)
			})
		})
	})
}

func (app *App) tokenize(line string, state uint64, tokens []editor.Token) ([]editor.Token, uint64) {
	lineNo := int(state >> 8)
	mode := state & 0xff
	next := func(m uint64) ([]editor.Token, uint64) {
		return tokens, uint64(lineNo+1)<<8 | m
	}
	trim := strings.TrimSpace(line)
	if mode&1 != 0 {
		tokens = append(tokens, editor.Token{End: len(line), Kind: editor.TokenCode})
		if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
			return next(0)
		}
		return next(1)
	}
	if lineNo == 0 && trim == "---" {
		tokens = append(tokens, editor.Token{End: len(line), Kind: editor.TokenMeta})
		return next(2)
	}
	if mode&2 != 0 {
		tokens = append(tokens, editor.Token{End: len(line), Kind: editor.TokenMeta})
		if trim == "---" {
			return next(0)
		}
		return next(2)
	}
	if strings.HasPrefix(trim, "```") || strings.HasPrefix(trim, "~~~") {
		tokens = append(tokens, editor.Token{End: len(line), Kind: editor.TokenCode})
		return next(1)
	}
	live := app.mode == modeLive && int(app.caretLine.Load()) != lineNo
	tokens = colorLine(line, tokens, live)
	return next(0)
}

func colorLine(line string, toks []editor.Token, live bool) []editor.Token {
	def := editor.TokenText
	i := 0
	if n, lead, ok := headingMarks(line); ok {
		kind := editor.TokenHeading
		if live {
			kind = editor.TokenMeta
		}
		toks = addTok(toks, lead+n, kind)
		if lead+n < len(line) && line[lead+n] == ' ' {
			if live {
				toks = addTok(toks, lead+n+1, editor.TokenMeta)
			}
		}
		i = lead + n
		if i < len(line) && line[i] == ' ' {
			i++
		}
		def = editor.TokenHeading
	}
	for i < len(line) {
		rest := line[i:]
		switch {
		case strings.HasPrefix(rest, "![[") || strings.HasPrefix(rest, "[["):
			toks, i = colorWiki(line, toks, i, live, def)
		case strings.HasPrefix(rest, "**"), strings.HasPrefix(rest, "__"), strings.HasPrefix(rest, "~~"):
			toks = addTok(toks, i+2, editor.TokenMeta)
			i += 2
		case strings.HasPrefix(rest, "=="):
			if end := strings.Index(rest[2:], "=="); end > 0 {
				toks = addTok(toks, i+2, editor.TokenMeta)
				toks = addTok(toks, i+2+end, editor.TokenStrong)
				toks = addTok(toks, i+4+end, editor.TokenMeta)
				i += 4 + end
				continue
			}
			toks = addTok(toks, i+1, def)
			i++
		case rest[0] == '`':
			end := strings.IndexByte(rest[1:], '`')
			if end >= 0 {
				toks = addTok(toks, i+1, editor.TokenMeta)
				toks = addTok(toks, i+1+end, editor.TokenCode)
				toks = addTok(toks, i+2+end, editor.TokenMeta)
				i += 2 + end
				continue
			}
			toks = addTok(toks, i+1, editor.TokenCode)
			i++
		case rest[0] == '*' || rest[0] == '_':
			toks = addTok(toks, i+1, editor.TokenMeta)
			i++
		case rest[0] == '#':
			if n := tagLen(rest); n > 1 && (i == 0 || boundary(line, i)) {
				toks = addTok(toks, i+n, editor.TokenLink)
				i += n
				continue
			}
			_, w := utf8.DecodeRuneInString(rest)
			toks = addTok(toks, i+w, def)
			i += w
		default:
			_, w := utf8.DecodeRuneInString(rest)
			toks = addTok(toks, i+w, def)
			i += w
		}
	}
	return toks
}

func colorWiki(line string, toks []editor.Token, i int, live bool, def editor.TokenKind) ([]editor.Token, int) {
	rest := line[i:]
	open := 2
	if strings.HasPrefix(rest, "![[") {
		open = 3
	}
	rel := strings.Index(rest[open:], "]]")
	if rel < 0 {
		toks = addTok(toks, i+1, def)
		return toks, i + 1
	}
	inner := rest[open : open+rel]
	aliasAt := strings.IndexByte(inner, '|')
	total := open + rel + 2
	if !live {
		toks = addTok(toks, i+total, editor.TokenLink)
		return toks, i + total
	}
	toks = addTok(toks, i+open, editor.TokenMeta)
	if aliasAt >= 0 {
		toks = addTok(toks, i+open+aliasAt+1, editor.TokenMeta)
		toks = addTok(toks, i+open+rel, editor.TokenLink)
	} else {
		toks = addTok(toks, i+open+rel, editor.TokenLink)
	}
	toks = addTok(toks, i+total, editor.TokenMeta)
	return toks, i + total
}

func headingMarks(line string) (n, lead int, ok bool) {
	lead = 0
	for lead < len(line) && line[lead] == ' ' {
		lead++
	}
	rest := line[lead:]
	for n < len(rest) && rest[n] == '#' && n < 6 {
		n++
	}
	if n == 0 || n == len(rest) || rest[n] != ' ' {
		return 0, 0, false
	}
	return n, lead, true
}

func tagLen(rest string) int {
	if len(rest) < 2 || rest[0] != '#' {
		return 0
	}
	r, w := utf8.DecodeRuneInString(rest[1:])
	if !unicode.IsLetter(r) {
		return 0
	}
	i := 1 + w
	for i < len(rest) {
		r, w = utf8.DecodeRuneInString(rest[i:])
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' || r == '-' || r == '/' {
			i += w
			continue
		}
		break
	}
	return i
}

func boundary(line string, i int) bool {
	if i == 0 {
		return true
	}
	r, _ := utf8.DecodeLastRuneInString(line[:i])
	return !unicode.IsLetter(r) && !unicode.IsDigit(r)
}

func addTok(toks []editor.Token, end int, kind editor.TokenKind) []editor.Token {
	if end < 0 {
		return toks
	}
	if len(toks) > 0 && toks[len(toks)-1].Kind == kind {
		toks[len(toks)-1].End = end
		return toks
	}
	return append(toks, editor.Token{End: end, Kind: kind})
}

func (app *App) complete(r editor.CompletionRequest) {
	t := app.current()
	if t == nil {
		return
	}
	src := t.state.Buffer.String()
	if r.Offset < 0 || r.Offset > len(src) {
		return
	}
	before := src[:r.Offset]
	i := strings.LastIndex(before, "[[")
	if i < 0 {
		return
	}
	frag := before[i+2:]
	if strings.ContainsAny(frag, "\n]") {
		return
	}
	start := i + 2
	prefix := frag
	if bar := strings.LastIndex(frag, "|"); bar >= 0 {
		start = i + 2 + bar + 1
		prefix = frag[bar+1:]
	}
	items := app.index.Complete(strings.TrimSpace(prefix), 40)
	comps := make([]editor.Completion, 0, len(items))
	for _, it := range items {
		comps = append(comps, editor.Completion{
			Label: it.Label, Detail: it.Detail, Insert: it.Label, Kind: editor.CompletionText,
		})
	}
	t.state.ShowCompletions(start, comps)
}

func (app *App) gotoDef(offset int) {
	t := app.current()
	if t == nil {
		return
	}
	doc := md.Parse(t.state.Buffer.String())
	for _, l := range doc.Links {
		if l.End > l.Start && offset >= l.Start && offset < l.End {
			if p, ok := app.index.Resolve(l.Target); ok {
				app.openNote(p)
			} else if !l.Embed && !md.ImageExt(l.Target) {
				app.createNamed(l.Target)
			}
			return
		}
	}
}

func (app *App) hover(r editor.HoverRequest) {
	t := app.current()
	if t == nil {
		return
	}
	doc := md.Parse(t.state.Buffer.String())
	for _, l := range doc.Links {
		if l.End > l.Start && r.Offset >= l.Start && r.Offset < l.End {
			msg := l.Target
			if p, ok := app.index.Resolve(l.Target); ok {
				msg = p
			} else {
				msg = "Unresolved: " + l.Target
			}
			t.state.ShowHover(l.Start, l.End, msg, nil)
			return
		}
	}
}
