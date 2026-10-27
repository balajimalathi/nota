// Package md reads Obsidian-flavored Markdown: wikilinks, embeds, tags,
// highlights, tasks, callouts and YAML frontmatter, on top of goldmark.
package md

import (
	"bytes"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/extension"
	extast "github.com/yuin/goldmark/extension/ast"
	"github.com/yuin/goldmark/parser"
	"github.com/yuin/goldmark/text"
	"github.com/yuin/goldmark/util"
)

// Heading is an ATX or setext heading.
type Heading struct {
	Level int
	Text  string
	Line  int // 0-based
	Start int
}

// Link is a wikilink, embed, or a Markdown link to another note.
type Link struct {
	Target                 string
	Alias                  string
	Embed                  bool
	Markdown               bool
	Start, End             int
	TargetStart, TargetEnd int
}

// Tag is an inline #tag or a frontmatter tag.
type Tag struct {
	Name       string
	Start, End int
	Inline     bool
}

// Task is a GFM task list item.
type Task struct {
	Text string
	Done bool
	Line int
}

// Callout is an Obsidian blockquote callout.
type Callout struct {
	Kind        string
	Title       string
	Line        int
	MarkerStart int
	MarkerEnd   int
}

// Highlight is a ==marked== span.
type Highlight struct {
	Text       string
	Start, End int
	InnerStart int
	InnerEnd   int
}

// Frontmatter is the YAML block at the top of a note. Only the fields
// notes care about are decoded; other keys are kept in Raw.
type Frontmatter struct {
	Raw     string
	Title   string
	Aliases []string
	Tags    []string
}

// Document is the outline of one note.
type Document struct {
	Title       string
	Frontmatter Frontmatter
	Headings    []Heading
	Links       []Link
	Tags        []Tag
	Tasks       []Task
	Callouts    []Callout
	Highlights  []Highlight
}

// SplitTarget separates a wikilink target from a #heading or ^block suffix.
func SplitTarget(target string) (path, fragment string) {
	i := strings.IndexAny(target, "#^")
	if i < 0 {
		return strings.TrimSpace(target), ""
	}
	return strings.TrimSpace(target[:i]), target[i:]
}

// Parse reads src into an outline. Offsets refer to src.
func Parse(src string) Document {
	raw := []byte(src)
	masked, fm := maskFrontmatter(raw)
	root := newParser().Parse(text.NewReader(masked))
	d := Document{Frontmatter: fm}
	for _, tag := range fm.Tags {
		d.Tags = append(d.Tags, Tag{Name: tag})
	}
	_ = ast.Walk(root, func(n ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch x := n.(type) {
		case *ast.Heading:
			start := blockStart(x, masked)
			d.Headings = append(d.Headings, Heading{
				Level: x.Level,
				Text:  inlineText(x, masked),
				Line:  lineOf(masked, start),
				Start: start,
			})
		case *wikilink:
			d.Links = append(d.Links, Link{
				Target: x.Target, Alias: x.Alias, Embed: x.Embed,
				Start: x.Start, End: x.End,
				TargetStart: x.TargetStart, TargetEnd: x.TargetEnd,
			})
		case *tagNode:
			d.Tags = append(d.Tags, Tag{Name: x.Name, Start: x.Start, End: x.End, Inline: true})
		case *markNode:
			d.Highlights = append(d.Highlights, Highlight{
				Text: x.Value, Start: x.Start, End: x.End,
				InnerStart: x.InnerStart, InnerEnd: x.InnerEnd,
			})
		case *ast.Link:
			dest := string(x.Destination)
			if noteDest(dest) {
				d.Links = append(d.Links, Link{Target: dest, Markdown: true})
			}
		case *extast.TaskCheckBox:
			parent := x.Parent()
			start := blockStart(parent, masked)
			d.Tasks = append(d.Tasks, Task{
				Text: strings.TrimSpace(inlineText(parent, masked)),
				Done: x.IsChecked,
				Line: lineOf(masked, start),
			})
		case *ast.Blockquote:
			if c, ok := calloutOf(x, masked); ok {
				d.Callouts = append(d.Callouts, c)
			}
		}
		return ast.WalkContinue, nil
	})
	switch {
	case fm.Title != "":
		d.Title = fm.Title
	case len(d.Headings) > 0:
		d.Title = d.Headings[0].Text
	}
	return d
}

func noteDest(dest string) bool {
	if dest == "" || strings.Contains(dest, "://") || strings.HasPrefix(dest, "#") {
		return false
	}
	low := strings.ToLower(dest)
	return strings.HasSuffix(low, ".md") || !strings.Contains(dest, ".")
}

func newParser() parser.Parser {
	return goldmark.New(
		goldmark.WithExtensions(extension.GFM, notaExt{}),
	).Parser()
}

type notaExt struct{}

func (notaExt) Extend(m goldmark.Markdown) {
	m.Parser().AddOptions(parser.WithInlineParsers(
		util.Prioritized(&embedParser{}, 150),
		util.Prioritized(&wikilinkParser{}, 160),
		util.Prioritized(&markParser{}, 450),
		util.Prioritized(&tagParser{}, 460),
	))
}

type wikilink struct {
	ast.BaseInline
	Target, Alias          string
	Embed                  bool
	Start, End             int
	TargetStart, TargetEnd int
}

var kindWikilink = ast.NewNodeKind("Wikilink")

func (n *wikilink) Kind() ast.NodeKind { return kindWikilink }
func (n *wikilink) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Target": n.Target}, nil)
}

type tagNode struct {
	ast.BaseInline
	Name       string
	Start, End int
}

var kindTag = ast.NewNodeKind("Tag")

func (n *tagNode) Kind() ast.NodeKind { return kindTag }
func (n *tagNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, map[string]string{"Name": n.Name}, nil)
}

type markNode struct {
	ast.BaseInline
	Value                string
	Start, End           int
	InnerStart, InnerEnd int
}

var kindMark = ast.NewNodeKind("Mark")

func (n *markNode) Kind() ast.NodeKind { return kindMark }
func (n *markNode) Dump(src []byte, level int) {
	ast.DumpHelper(n, src, level, nil, nil)
}

type wikilinkParser struct{}

func (wikilinkParser) Trigger() []byte { return []byte{'['} }

func (wikilinkParser) Parse(_ ast.Node, r text.Reader, _ parser.Context) ast.Node {
	line, seg := r.PeekLine()
	if len(line) < 4 || line[0] != '[' || line[1] != '[' {
		return nil
	}
	return takeWikilink(r, line, seg.Start, false)
}

type embedParser struct{}

func (embedParser) Trigger() []byte { return []byte{'!'} }

func (embedParser) Parse(_ ast.Node, r text.Reader, _ parser.Context) ast.Node {
	line, seg := r.PeekLine()
	if len(line) < 5 || line[0] != '!' || line[1] != '[' || line[2] != '[' {
		return nil
	}
	return takeWikilink(r, line, seg.Start, true)
}

func takeWikilink(r text.Reader, line []byte, abs int, embed bool) ast.Node {
	open := 2
	if embed {
		open = 3
	}
	rel := bytes.Index(line[open:], []byte("]]"))
	if rel < 0 {
		return nil
	}
	inner := line[open : open+rel]
	target, alias, tStart, tEnd := splitInner(inner)
	if target == "" {
		return nil
	}
	width := open + rel + 2
	r.Advance(width)
	base := abs + open
	return &wikilink{
		Target: target, Alias: alias, Embed: embed,
		Start: abs, End: abs + width,
		TargetStart: base + tStart, TargetEnd: base + tEnd,
	}
}

func splitInner(inner []byte) (target, alias string, tStart, tEnd int) {
	bar := bytes.IndexByte(inner, '|')
	raw := inner
	if bar >= 0 {
		raw = inner[:bar]
		alias = strings.TrimSpace(string(inner[bar+1:]))
	}
	lead := len(raw) - len(bytes.TrimLeft(raw, " \t"))
	trimmed := bytes.TrimSpace(raw)
	return string(trimmed), alias, lead, lead + len(trimmed)
}

type markParser struct{}

func (markParser) Trigger() []byte { return []byte{'='} }

func (markParser) Parse(_ ast.Node, r text.Reader, _ parser.Context) ast.Node {
	line, seg := r.PeekLine()
	if len(line) < 5 || line[0] != '=' || line[1] != '=' {
		return nil
	}
	rel := bytes.Index(line[2:], []byte("=="))
	if rel <= 0 {
		return nil
	}
	inner := line[2 : 2+rel]
	if bytes.Contains(inner, []byte("\n")) {
		return nil
	}
	width := 2 + rel + 2
	r.Advance(width)
	return &markNode{
		Value: string(inner),
		Start: seg.Start, End: seg.Start + width,
		InnerStart: seg.Start + 2, InnerEnd: seg.Start + 2 + rel,
	}
}

type tagParser struct{}

func (tagParser) Trigger() []byte { return []byte{'#'} }

func (tagParser) Parse(_ ast.Node, r text.Reader, _ parser.Context) ast.Node {
	prev := r.PrecendingCharacter()
	if prev != '\n' && prev != ' ' && prev != '\t' && !unicode.IsPunct(prev) && prev != '(' {
		return nil
	}
	line, seg := r.PeekLine()
	if len(line) < 2 || line[0] != '#' {
		return nil
	}
	i := 1
	r0, w := utf8.DecodeRune(line[i:])
	if !unicode.IsLetter(r0) {
		return nil
	}
	i += w
	for i < len(line) {
		ch, w := utf8.DecodeRune(line[i:])
		if unicode.IsLetter(ch) || unicode.IsDigit(ch) || ch == '_' || ch == '-' || ch == '/' {
			i += w
			continue
		}
		break
	}
	name := string(line[1:i])
	if strings.HasPrefix(name, "/") || strings.HasSuffix(name, "/") || strings.Contains(name, "//") {
		return nil
	}
	r.Advance(i)
	return &tagNode{Name: name, Start: seg.Start, End: seg.Start + i}
}

func maskFrontmatter(src []byte) ([]byte, Frontmatter) {
	if !bytes.HasPrefix(src, []byte("---\n")) && !bytes.HasPrefix(src, []byte("---\r\n")) {
		return src, Frontmatter{}
	}
	nl := bytes.IndexByte(src, '\n')
	if nl < 0 {
		return src, Frontmatter{}
	}
	off := nl + 1
	rest := src[off:]
	for {
		lineEnd := bytes.IndexByte(rest, '\n')
		line := rest
		if lineEnd >= 0 {
			line = rest[:lineEnd]
		}
		if bytes.Equal(bytes.TrimRight(line, "\r"), []byte("---")) {
			end := off + len(line)
			if lineEnd >= 0 {
				end = off + lineEnd + 1
			}
			masked := bytes.Clone(src)
			for i := 0; i < end && i < len(masked); i++ {
				if masked[i] != '\n' && masked[i] != '\r' {
					masked[i] = ' '
				}
			}
			return masked, parseFM(string(src[:end]))
		}
		if lineEnd < 0 {
			break
		}
		rest = rest[lineEnd+1:]
		off += lineEnd + 1
	}
	return src, Frontmatter{}
}

func parseFM(block string) Frontmatter {
	fm := Frontmatter{Raw: block}
	lines := strings.Split(block, "\n")
	var listKey string
	for _, line := range lines {
		trim := strings.TrimRight(line, "\r")
		if strings.TrimSpace(trim) == "---" || strings.TrimSpace(trim) == "" {
			continue
		}
		itemLine := strings.TrimSpace(trim)
		if listKey != "" && strings.HasPrefix(itemLine, "- ") {
			item := strings.Trim(strings.TrimSpace(strings.TrimPrefix(itemLine, "-")), `"'`)
			appendFM(&fm, listKey, item)
			continue
		}
		key, val, ok := strings.Cut(trim, ":")
		if !ok {
			listKey = ""
			continue
		}
		key = strings.ToLower(strings.TrimSpace(key))
		val = strings.TrimSpace(val)
		listKey = ""
		if val == "" || val == "|" || val == ">" {
			if key == "tags" || key == "tag" || key == "aliases" || key == "alias" {
				listKey = key
			}
			continue
		}
		if strings.HasPrefix(val, "[") && strings.HasSuffix(val, "]") {
			for _, part := range strings.Split(val[1:len(val)-1], ",") {
				appendFM(&fm, key, strings.Trim(strings.TrimSpace(part), `"'`))
			}
			continue
		}
		val = strings.Trim(val, `"'`)
		switch key {
		case "title":
			fm.Title = val
		case "tags", "tag", "aliases", "alias":
			if strings.Contains(val, ",") {
				for _, part := range strings.Split(val, ",") {
					appendFM(&fm, key, strings.TrimSpace(part))
				}
			} else {
				appendFM(&fm, key, val)
			}
		}
	}
	return fm
}

func appendFM(fm *Frontmatter, key, val string) {
	if val == "" {
		return
	}
	switch key {
	case "title":
		fm.Title = val
	case "tags", "tag":
		fm.Tags = append(fm.Tags, strings.TrimPrefix(val, "#"))
	case "aliases", "alias":
		fm.Aliases = append(fm.Aliases, val)
	}
}

func calloutOf(bq ast.Node, src []byte) (Callout, bool) {
	raw := inlineText(bq, src)
	line, _, _ := strings.Cut(raw, "\n")
	line = strings.TrimSpace(line)
	if !strings.HasPrefix(line, "[!") {
		return Callout{}, false
	}
	rest := line[2:]
	end := strings.IndexByte(rest, ']')
	if end <= 0 {
		return Callout{}, false
	}
	kind := rest[:end]
	title := strings.TrimSpace(rest[end+1:])
	title = strings.TrimPrefix(title, "+")
	title = strings.TrimPrefix(title, "-")
	title = strings.TrimSpace(title)
	start := blockStart(bq, src)
	marker := "[!" + kind + "]"
	mStart := bytes.Index(src[start:], []byte(marker))
	c := Callout{Kind: kind, Title: title, Line: lineOf(src, start)}
	if mStart >= 0 {
		c.MarkerStart = start + mStart
		c.MarkerEnd = c.MarkerStart + len(marker)
	}
	return c, true
}

func inlineText(n ast.Node, src []byte) string {
	if n == nil {
		return ""
	}
	var b strings.Builder
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch t := c.(type) {
		case *ast.Text:
			b.Write(t.Segment.Value(src))
			if t.SoftLineBreak() || t.HardLineBreak() {
				b.WriteByte('\n')
			}
		case *ast.String:
			b.Write(t.Value)
		}
		return ast.WalkContinue, nil
	})
	return b.String()
}

func blockStart(n ast.Node, src []byte) int {
	if n == nil {
		return 0
	}
	if lines := n.Lines(); lines != nil && lines.Len() > 0 {
		return lines.At(0).Start
	}
	var start int
	found := false
	_ = ast.Walk(n, func(c ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering || found {
			return ast.WalkContinue, nil
		}
		if t, ok := c.(*ast.Text); ok {
			start = t.Segment.Start
			found = true
			return ast.WalkStop, nil
		}
		return ast.WalkContinue, nil
	})
	if !found {
		return 0
	}
	return start
}

func lineOf(src []byte, off int) int {
	if off < 0 {
		return 0
	}
	if off > len(src) {
		off = len(src)
	}
	return bytes.Count(src[:off], []byte("\n"))
}

// ImageExt reports whether name looks like an embeddable image.
func ImageExt(name string) bool {
	switch strings.ToLower(extOf(name)) {
	case ".png", ".jpg", ".jpeg", ".gif", ".webp", ".bmp", ".svg":
		return true
	default:
		return false
	}
}

func extOf(name string) string {
	i := strings.LastIndex(name, ".")
	if i < 0 {
		return ""
	}
	return name[i:]
}
