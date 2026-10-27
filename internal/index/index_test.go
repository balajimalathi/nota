package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestLinksSearchAndRewrite(t *testing.T) {
	ix := New()
	now := time.Now()
	ix.Upsert("alpha.md", "# Alpha\n\nSee [[Beta Note]] and #topic.\n\nalpha words here", now, 10)
	ix.Upsert("beta.md", "# Beta Note\n\nNothing else about zebras.", now, 10)
	ix.Upsert("nested/beta.md", "another beta", now, 4)

	if p, ok := ix.Resolve("beta"); !ok || p != "beta.md" {
		t.Fatalf("exact beta.md: %q %v", p, ok)
	}
	p, ok := ix.Resolve("Beta Note")
	if !ok || p != "beta.md" {
		t.Fatalf("resolve beta note %q %v", p, ok)
	}
	backs := ix.Backlinks("beta.md")
	if len(backs) != 1 || backs[0].Path != "alpha.md" {
		t.Fatalf("backlinks %#v", backs)
	}
	tags := ix.Tags()
	if len(tags) != 1 || tags[0].Tag != "topic" || tags[0].Count != 1 {
		t.Fatalf("tags %#v", tags)
	}
	hits := ix.Search("zebras", 5)
	if len(hits) != 1 || hits[0].Path != "beta.md" {
		t.Fatalf("search %#v", hits)
	}
	fuzzy := ix.Fuzzy("alp", 5)
	if len(fuzzy) == 0 || fuzzy[0].Path != "alpha.md" {
		t.Fatalf("fuzzy %#v", fuzzy)
	}
	ix.Upsert("a/note.md", "see [[b/note]]", now, 8)
	ix.Upsert("b/note.md", "body", now, 4)
	if _, ok := ix.Resolve("note"); ok {
		t.Fatal("ambiguous note resolved")
	}
	out, n := ix.Rewrite("see [[b/note]]", "b/note.md", "c/note.md")
	if n != 1 || out != "see [[c/note]]" {
		t.Fatalf("rewrite %d %q", n, out)
	}
	ix.Remove("alpha.md")
	if ix.Count() != 4 {
		t.Fatalf("count %d", ix.Count())
	}
	if len(ix.Backlinks("beta.md")) != 0 {
		t.Fatal("backlink remained")
	}

	dir := t.TempDir()
	cache := filepath.Join(dir, "index.gob")
	ix2 := New()
	ix2.Upsert("a.md", "hello unique term", now, 3)
	if err := ix2.Save(cache); err != nil {
		t.Fatal(err)
	}
	got, err := Load(cache)
	if err != nil {
		t.Fatal(err)
	}
	if got.Count() != 1 || len(got.Search("unique", 3)) != 1 {
		t.Fatalf("loaded count %d", got.Count())
	}
	st, err := os.Stat(filepath.Join(dir, "a.md"))
	_ = st
	if !got.Fresh("a.md", now, 3) {
		t.Fatal("not fresh")
	}
}

func TestBulkRelink(t *testing.T) {
	ix := New()
	now := time.Now()
	ix.Bulk(func() {
		ix.Upsert("a.md", "[[b]]", now, 1)
		ix.Upsert("b.md", "target", now, 1)
	})
	if backs := ix.Backlinks("b.md"); len(backs) != 1 {
		t.Fatalf("bulk backlinks %#v", backs)
	}
	nodes, edges := ix.Neighborhood("a.md", 1)
	if len(nodes) != 2 || len(edges) != 1 {
		t.Fatalf("graph nodes %v edges %v", nodes, edges)
	}
}
