package vault

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestWriteScanRename(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	if err := v.Write("notes/a.md", []byte("hello")); err != nil {
		t.Fatal(err)
	}
	got, err := v.Read("notes/a.md")
	if err != nil || string(got) != "hello" {
		t.Fatalf("read %q %v", got, err)
	}
	if _, err := v.abs("../outside"); err == nil {
		t.Fatal("escaped")
	}
	files, err := v.Files()
	if err != nil {
		t.Fatal(err)
	}
	var saw bool
	for _, f := range files {
		if f.Path == "notes/a.md" && !f.Dir && f.Size == 5 {
			saw = true
		}
	}
	if !saw {
		t.Fatalf("files %#v", files)
	}
	if err := v.Rename("notes/a.md", "b.md"); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(root, "b.md")); err != nil {
		t.Fatal(err)
	}
	app, err := v.EnsureApp()
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(app, ".gitignore")); err != nil {
		t.Fatal(err)
	}
	files, _ = v.Files()
	for _, f := range files {
		if f.Path == ".app" || strings.HasPrefix(f.Path, ".app/") {
			t.Fatalf("listed .app: %s", f.Path)
		}
	}
}

func TestWatch(t *testing.T) {
	root := t.TempDir()
	v, err := Open(root)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	ch, err := v.Watch(ctx)
	if err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond)
	if err := os.WriteFile(filepath.Join(root, "n.md"), []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	deadline := time.After(2 * time.Second)
	for {
		select {
		case ev := <-ch:
			if ev.Path == "n.md" || ev.Op == "rescan" {
				return
			}
		case <-deadline:
			t.Fatal("timed out waiting for watch")
		}
	}
}
