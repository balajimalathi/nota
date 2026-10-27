package md

import (
	"net/url"
	"sort"
	"strings"
)

// RewriteLinks replaces the target of each wikilink for which fn reports
// a new target. The heading or block suffix and the alias are kept.
func RewriteLinks(src string, fn func(target string, embed bool) (string, bool)) (string, int) {
	d := Parse(src)
	links := make([]Link, 0, len(d.Links))
	for _, l := range d.Links {
		if l.Markdown || l.TargetEnd <= l.TargetStart {
			continue
		}
		links = append(links, l)
	}
	sort.Slice(links, func(i, j int) bool { return links[i].TargetStart > links[j].TargetStart })
	b := []byte(src)
	n := 0
	for _, l := range links {
		next, ok := fn(l.Target, l.Embed)
		if !ok || next == l.Target {
			continue
		}
		b = append(b[:l.TargetStart], append([]byte(next), b[l.TargetEnd:]...)...)
		n++
	}
	return string(b), n
}

type span struct {
	start, end int
	text       string
}

// ForPreview turns Obsidian syntax into Markdown the native preview can
// show. read, when set, supplies the body of a note embed; depth is
// limited so embeds cannot recurse without end.
func ForPreview(src string, read func(target string) (string, bool)) string {
	return forPreview(src, 0, map[string]bool{}, read)
}

func forPreview(src string, depth int, seen map[string]bool, read func(target string) (string, bool)) string {
	d := Parse(src)
	var spans []span
	for _, l := range d.Links {
		if l.Markdown || l.End <= l.Start {
			continue
		}
		spans = append(spans, span{l.Start, l.End, previewLink(l, depth, seen, read)})
	}
	for _, h := range d.Highlights {
		if h.End <= h.Start {
			continue
		}
		spans = append(spans, span{h.Start, h.End, "<mark>" + h.Text + "</mark>"})
	}
	for _, t := range d.Tags {
		if !t.Inline || t.End <= t.Start {
			continue
		}
		label := "#" + t.Name
		spans = append(spans, span{t.Start, t.End, "[" + label + "](nota://tag/" + url.PathEscape(t.Name) + ")"})
	}
	for _, c := range d.Callouts {
		if c.MarkerEnd <= c.MarkerStart {
			continue
		}
		spans = append(spans, span{c.MarkerStart, c.MarkerEnd, "**" + displayKind(c.Kind) + "**"})
	}
	sort.Slice(spans, func(i, j int) bool { return spans[i].start > spans[j].start })
	b := []byte(src)
	last := len(b)
	for _, s := range spans {
		if s.end > last || s.start < 0 || s.end > len(b) || s.start >= s.end {
			continue
		}
		b = append(b[:s.start], append([]byte(s.text), b[s.end:]...)...)
		last = s.start
	}
	return string(b)
}

func previewLink(l Link, depth int, seen map[string]bool, read func(target string) (string, bool)) string {
	path, frag := SplitTarget(l.Target)
	label := l.Alias
	if label == "" {
		label = l.Target
	}
	if l.Embed && ImageExt(path) {
		alt := label
		if alt == "" {
			alt = path
		}
		return "![" + alt + "](" + path + ")"
	}
	if l.Embed && read != nil && depth < 2 {
		key := strings.ToLower(path)
		if !seen[key] {
			if body, ok := read(path); ok {
				seen[key] = true
				inner := forPreview(body, depth+1, seen, read)
				delete(seen, key)
				quoted := quote(inner)
				return "**" + label + "**\n\n" + quoted
			}
		}
	}
	dest := "nota://open/" + url.PathEscape(path)
	if frag != "" {
		dest += frag
	}
	return "[" + label + "](" + dest + ")"
}

func quote(s string) string {
	if s == "" {
		return ">"
	}
	lines := strings.Split(s, "\n")
	for i, ln := range lines {
		lines[i] = "> " + ln
	}
	return strings.Join(lines, "\n")
}

func displayKind(kind string) string {
	if kind == "" {
		return "Note"
	}
	r := []rune(strings.ToLower(kind))
	r[0] = []rune(strings.ToUpper(string(r[0])))[0]
	return string(r)
}
