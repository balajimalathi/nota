package manifest

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAndDiff(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "a.md"), []byte("one"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, ".app"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, ".app", "cache"), []byte("nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	m, err := Scan(root, "dev")
	if err != nil {
		t.Fatal(err)
	}
	if len(m.Files) != 1 || m.Files[0].Path != "a.md" {
		t.Fatalf("%#v", m.Files)
	}
	next := &Manifest{Files: []Entry{{Path: "a.md", Hash: "changed"}, {Path: "b.md", Hash: "new"}}}
	diff := Diff(m, next)
	kinds := map[string]string{}
	for _, c := range diff {
		kinds[c.Path] = c.Kind
	}
	if kinds["a.md"] != "update" || kinds["b.md"] != "add" {
		t.Fatalf("%#v", diff)
	}
	p := filepath.Join(root, ".app", "manifest.json")
	if err := m.Save(p); err != nil {
		t.Fatal(err)
	}
	loaded, err := Load(p)
	if err != nil || len(loaded.Files) != 1 {
		t.Fatalf("%v %#v", err, loaded)
	}
}
