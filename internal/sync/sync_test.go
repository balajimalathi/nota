package sync

import (
	"context"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestFolderConflict(t *testing.T) {
	local := t.TempDir()
	remote := t.TempDir()
	now := time.Date(2026, 10, 27, 12, 0, 0, 0, time.UTC)
	write(t, local, "a.md", "base")
	write(t, remote, "a.md", "base")
	f := &Folder{Local: local, Remote: remote, Device: "laptop", Now: func() time.Time { return now }}
	if err := f.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	write(t, local, "a.md", "local edit")
	write(t, remote, "a.md", "remote edit")
	if err := f.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(local, "a.md"))
	if err != nil || string(got) != "local edit" {
		t.Fatalf("local lost: %q %v", got, err)
	}
	conflict := ConflictName("a.md", "laptop", now)
	body, err := os.ReadFile(filepath.Join(local, filepath.FromSlash(conflict)))
	if err != nil || string(body) != "remote edit" {
		t.Fatalf("conflict %q %v", body, err)
	}
	st, err := f.Status(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if st.Mode != "folder" {
		t.Fatal(st.Mode)
	}
}

func TestHTTPSync(t *testing.T) {
	serverVault := t.TempDir()
	write(t, serverVault, "note.md", "from server")
	srv := httptest.NewServer(Handler(serverVault, "secret"))
	defer srv.Close()
	local := t.TempDir()
	h := &HTTP{Local: local, BaseURL: srv.URL, Token: "secret", Device: "desk"}
	if err := h.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(local, "note.md"))
	if err != nil || string(b) != "from server" {
		t.Fatalf("%q %v", b, err)
	}
	write(t, local, "note.md", "from client")
	// Base says both still match the pulled hash, so the client edit should push.
	if err := h.Push(context.Background()); err != nil {
		t.Fatal(err)
	}
	got, err := os.ReadFile(filepath.Join(serverVault, "note.md"))
	if err != nil || string(got) != "from client" {
		t.Fatalf("pushed %q %v", got, err)
	}
	bad := &HTTP{Local: local, BaseURL: srv.URL, Token: "nope", Device: "desk"}
	if err := bad.Pull(context.Background()); err == nil {
		t.Fatal("expected auth failure")
	}
}

func TestGitFastForward(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not installed")
	}
	base := t.TempDir()
	a := filepath.Join(base, "a")
	b := filepath.Join(base, "b")
	if err := os.MkdirAll(a, 0o755); err != nil {
		t.Fatal(err)
	}
	git(t, a, "init", "-b", "main")
	write(t, a, "n.md", "v1")
	git(t, a, "add", "n.md")
	git(t, a, "commit", "-m", "v1")
	git(t, base, "clone", a, b)
	write(t, a, "n.md", "v2")
	git(t, a, "add", "n.md")
	git(t, a, "commit", "-m", "v2")
	g := &Git{Root: b, Device: "other"}
	if err := g.Pull(context.Background()); err != nil {
		t.Fatal(err)
	}
	body, err := os.ReadFile(filepath.Join(b, "n.md"))
	if err != nil || string(body) != "v2" {
		t.Fatalf("ff %q %v", body, err)
	}
	st, err := g.Status(context.Background())
	if err != nil || st.Mode != "git" {
		t.Fatalf("%+v %v", st, err)
	}
}

func write(t *testing.T, root, rel, body string) {
	t.Helper()
	p := filepath.Join(root, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(p, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", args...)
	cmd.Dir = dir
	cmd.Env = append(os.Environ(),
		"GIT_AUTHOR_NAME=Nota",
		"GIT_AUTHOR_EMAIL=nota@example.com",
		"GIT_COMMITTER_NAME=Nota",
		"GIT_COMMITTER_EMAIL=nota@example.com",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
