package md

import (
	"strings"
	"testing"
)

const sample = `---
title: Hello
aliases:
  - Hi
tags: [a, b]
---

# Hello

See [[Other Note|the note]] and [[Pictures/Cat.png]] and ![[shot.png]].

A ==marked== word and a #topic tag.

- [x] done
- [ ] open

> [!warning] Careful
> Do not break it.

` + "```" + `
[[not a link]]
` + "```" + `
`

func TestParse(t *testing.T) {
	d := Parse(sample)
	if d.Title != "Hello" {
		t.Fatalf("title %q", d.Title)
	}
	if len(d.Frontmatter.Aliases) != 1 || d.Frontmatter.Aliases[0] != "Hi" {
		t.Fatalf("aliases %#v", d.Frontmatter.Aliases)
	}
	if len(d.Frontmatter.Tags) != 2 {
		t.Fatalf("tags %#v", d.Frontmatter.Tags)
	}
	if len(d.Headings) != 1 || d.Headings[0].Text != "Hello" || d.Headings[0].Level != 1 {
		t.Fatalf("headings %#v", d.Headings)
	}
	var wiki, embed int
	for _, l := range d.Links {
		if l.Embed {
			embed++
			if l.Target != "shot.png" {
				t.Fatalf("embed %q", l.Target)
			}
		} else if !l.Markdown && l.Alias == "the note" && l.Target != "Other Note" {
			t.Fatalf("alias link %#v", l)
		} else if !l.Markdown && !l.Embed {
			wiki++
		}
	}
	if wiki < 2 || embed != 1 {
		t.Fatalf("links wiki=%d embed=%d %#v", wiki, embed, d.Links)
	}
	foundTag := false
	for _, tag := range d.Tags {
		if tag.Name == "topic" && tag.Inline {
			foundTag = true
		}
	}
	if !foundTag {
		t.Fatalf("tags %#v", d.Tags)
	}
	if len(d.Tasks) != 2 || !d.Tasks[0].Done || d.Tasks[1].Done {
		t.Fatalf("tasks %#v", d.Tasks)
	}
	if len(d.Callouts) != 1 || d.Callouts[0].Kind != "warning" || d.Callouts[0].Title != "Careful" {
		t.Fatalf("callouts %#v", d.Callouts)
	}
	if len(d.Highlights) != 1 || d.Highlights[0].Text != "marked" {
		t.Fatalf("marks %#v", d.Highlights)
	}
	for _, l := range d.Links {
		if l.Target == "not a link" {
			t.Fatal("parsed a wikilink inside a fence")
		}
	}
}

func TestRewriteAndPreview(t *testing.T) {
	src := "Go to [[Foo#h|there]] and [[Foo]]."
	out, n := RewriteLinks(src, func(target string, _ bool) (string, bool) {
		path, frag := SplitTarget(target)
		if path != "Foo" {
			return "", false
		}
		return "notes/Foo" + frag, true
	})
	if n != 2 {
		t.Fatalf("rewrites %d", n)
	}
	if out != "Go to [[notes/Foo#h|there]] and [[notes/Foo]]." {
		t.Fatalf("rewrite %q", out)
	}
	prev := ForPreview("![[a.png]] and [[Foo|bar]] ==x== #tag", nil)
	if !strings.Contains(prev, "![a.png](a.png)") {
		t.Fatalf("image %q", prev)
	}
	if !strings.Contains(prev, "nota://open/Foo") || !strings.Contains(prev, "<mark>x</mark>") || !strings.Contains(prev, "nota://tag/tag") {
		t.Fatalf("preview %q", prev)
	}
	body := ForPreview("![[Other]]", func(target string) (string, bool) {
		if target == "Other" {
			return "inside", true
		}
		return "", false
	})
	if !strings.Contains(body, "> inside") {
		t.Fatalf("embed %q", body)
	}
}

func TestSplitTarget(t *testing.T) {
	p, f := SplitTarget("Foo#Bar")
	if p != "Foo" || f != "#Bar" {
		t.Fatalf("%q %q", p, f)
	}
}
