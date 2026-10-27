package embed

import (
	"path/filepath"
	"testing"
)

func TestSearchAndFile(t *testing.T) {
	s := New()
	chunks := []Chunk{
		{Path: "a.md", Heading: "A", Text: "cats", Hash: 1},
		{Path: "b.md", Heading: "B", Text: "dogs", Hash: 2},
	}
	vecs := [][]float32{{1, 0, 0}, {0, 1, 0}}
	if err := s.Upsert(chunks, vecs); err != nil {
		t.Fatal(err)
	}
	hits := s.Search([]float32{0.9, 0.1, 0}, 2, "")
	if len(hits) != 2 || hits[0].Path != "a.md" {
		t.Fatalf("%#v", hits)
	}
	hits = s.Search([]float32{1, 0, 0}, 2, "a.md")
	if len(hits) != 1 || hits[0].Path != "b.md" {
		t.Fatalf("skip %#v", hits)
	}
	dir := t.TempDir()
	path := filepath.Join(dir, "e.bin")
	if err := s.Save(path); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(path)
	if err != nil || loaded.Len() != 2 || loaded.Dim() != 3 {
		t.Fatalf("load %v %d", err, loaded.Len())
	}
	parts := Chunks("n.md", "# One\n\nalpha\n\n# Two\n\nbeta")
	if len(parts) != 2 || parts[0].Heading != "One" || parts[1].Heading != "Two" {
		t.Fatalf("chunks %#v", parts)
	}
	if len(s.Missing(parts)) != 2 {
		t.Fatal("missing")
	}
}
