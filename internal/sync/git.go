package sync

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Git syncs a vault with the git remote of the folder.
type Git struct {
	Root   string
	Device string
	Now    func() time.Time
}

func (g *Git) now() time.Time {
	if g.Now != nil {
		return g.Now()
	}
	return time.Now()
}

func (g *Git) Status(ctx context.Context) (Status, error) {
	st := Status{Mode: "git"}
	if _, err := exec.LookPath("git"); err != nil {
		return st, errors.New("git is not installed")
	}
	if _, err := g.git(ctx, "rev-parse", "--is-inside-work-tree"); err != nil {
		st.Detail = "not a git repository"
		return st, nil
	}
	out, err := g.git(ctx, "status", "--porcelain")
	if err != nil {
		return st, err
	}
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if len(line) > 3 {
			st.Dirty = append(st.Dirty, strings.TrimSpace(line[2:]))
		}
	}
	counts, err := g.git(ctx, "rev-list", "--left-right", "--count", "HEAD...@{u}")
	if err == nil {
		fields := strings.Fields(counts)
		if len(fields) == 2 {
			st.Ahead, _ = strconv.Atoi(fields[0])
			st.Behind, _ = strconv.Atoi(fields[1])
		}
	}
	return st, nil
}

// Init creates a repository in the vault.
func (g *Git) Init(ctx context.Context) error {
	_, err := g.git(ctx, "init")
	return err
}

func (g *Git) Pull(ctx context.Context) error {
	if _, err := g.git(ctx, "fetch"); err != nil {
		return err
	}
	if _, err := g.git(ctx, "merge", "--ff-only", "@{u}"); err == nil {
		return nil
	}
	names, err := g.git(ctx, "diff", "--name-only", "HEAD", "@{u}")
	if err != nil {
		return fmt.Errorf("fast-forward failed: %w", err)
	}
	for _, p := range splitLines(names) {
		if err := g.importRemote(ctx, p); err != nil {
			return err
		}
	}
	return nil
}

func (g *Git) importRemote(ctx context.Context, rel string) error {
	remote, err := g.git(ctx, "show", "@{u}:"+rel)
	if err != nil {
		return nil
	}
	abs := filepath.Join(g.Root, filepath.FromSlash(rel))
	local, lerr := os.ReadFile(abs)
	head, herr := g.git(ctx, "show", "HEAD:"+rel)
	if lerr != nil {
		return os.WriteFile(abs, []byte(remote), 0o644)
	}
	if herr == nil && string(local) == head {
		return os.WriteFile(abs, []byte(remote), 0o644)
	}
	if string(local) == remote {
		return nil
	}
	name := ConflictName(rel, g.Device, g.now())
	return os.WriteFile(filepath.Join(g.Root, filepath.FromSlash(name)), []byte(remote), 0o644)
}

func (g *Git) Push(ctx context.Context) error {
	if _, err := g.git(ctx, "add", "-A", "--", ".", ":!.app"); err != nil {
		return err
	}
	if out, err := g.git(ctx, "status", "--porcelain"); err == nil && strings.TrimSpace(out) != "" {
		if _, err := g.git(ctx, "commit", "-m", "sync notes"); err != nil {
			return err
		}
	}
	_, err := g.git(ctx, "push")
	return err
}

func (g *Git) git(ctx context.Context, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = g.Root
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = strings.TrimSpace(stdout.String())
		}
		if msg == "" {
			return "", err
		}
		return stdout.String(), fmt.Errorf("%s", msg)
	}
	return stdout.String(), nil
}

func splitLines(s string) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(strings.TrimRight(line, "\r"))
		if line != "" {
			out = append(out, filepath.ToSlash(line))
		}
	}
	return out
}
