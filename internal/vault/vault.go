// Package vault is a folder of notes. Files on disk are the source of truth.
package vault

import (
	"context"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/fsnotify/fsnotify"
)

// File is one path in the vault, relative to the root with forward slashes.
type File struct {
	Path    string
	Dir     bool
	Size    int64
	ModTime time.Time
}

// Event is a change reported by Watch.
type Event struct {
	Path string
	Op   string // "write", "remove", or "rescan"
}

// Vault is an open folder.
type Vault struct {
	root string
}

// Open checks that root is a directory.
func Open(root string) (*Vault, error) {
	abs, err := filepath.Abs(root)
	if err != nil {
		return nil, err
	}
	st, err := os.Stat(abs)
	if err != nil {
		return nil, err
	}
	if !st.IsDir() {
		return nil, &os.PathError{Op: "open", Path: abs, Err: os.ErrInvalid}
	}
	return &Vault{root: abs}, nil
}

// Root is the absolute vault path.
func (v *Vault) Root() string { return v.root }

// Files lists the vault, skipping .git, .app and node_modules.
func (v *Vault) Files() ([]File, error) {
	var out []File
	err := filepath.WalkDir(v.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(v.root, path)
		if err != nil || rel == "." {
			return nil
		}
		rel = filepath.ToSlash(rel)
		if skip(rel) {
			if d.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		out = append(out, File{Path: rel, Dir: d.IsDir(), Size: info.Size(), ModTime: info.ModTime()})
		return nil
	})
	return out, err
}

// Read returns the bytes of a relative path.
func (v *Vault) Read(rel string) ([]byte, error) {
	abs, err := v.abs(rel)
	if err != nil {
		return nil, err
	}
	return os.ReadFile(abs)
}

// Write stores data atomically.
func (v *Vault) Write(rel string, data []byte) error {
	abs, err := v.abs(rel)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(abs), 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(abs), ".nota-*.tmp")
	if err != nil {
		return err
	}
	name := tmp.Name()
	_, werr := tmp.Write(data)
	cerr := tmp.Close()
	if werr != nil {
		os.Remove(name)
		return werr
	}
	if cerr != nil {
		os.Remove(name)
		return cerr
	}
	if err := replace(name, abs); err != nil {
		os.Remove(name)
		return err
	}
	return nil
}

// Rename moves a file or directory inside the vault.
func (v *Vault) Rename(from, to string) error {
	a, err := v.abs(from)
	if err != nil {
		return err
	}
	b, err := v.abs(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(b), 0o755); err != nil {
		return err
	}
	return os.Rename(a, b)
}

// Remove deletes a file or an empty-able directory tree.
func (v *Vault) Remove(rel string) error {
	abs, err := v.abs(rel)
	if err != nil {
		return err
	}
	return os.RemoveAll(abs)
}

// Mkdir creates a directory.
func (v *Vault) Mkdir(rel string) error {
	abs, err := v.abs(rel)
	if err != nil {
		return err
	}
	return os.MkdirAll(abs, 0o755)
}

// Stat returns one relative path.
func (v *Vault) Stat(rel string) (File, error) {
	abs, err := v.abs(rel)
	if err != nil {
		return File{}, err
	}
	info, err := os.Stat(abs)
	if err != nil {
		return File{}, err
	}
	return File{Path: slash(rel), Dir: info.IsDir(), Size: info.Size(), ModTime: info.ModTime()}, nil
}

// EnsureApp makes the .app directory, ignored when the vault is a git repo.
func (v *Vault) EnsureApp() (string, error) {
	dir := filepath.Join(v.root, ".app")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	gi := filepath.Join(dir, ".gitignore")
	if _, err := os.Stat(gi); os.IsNotExist(err) {
		_ = os.WriteFile(gi, []byte("*\n"), 0o644)
	}
	return dir, nil
}

// Watch reports changes until ctx ends. Subdirectories are watched too.
func (v *Vault) Watch(ctx context.Context) (<-chan Event, error) {
	w, err := fsnotify.NewWatcher()
	if err != nil {
		return nil, err
	}
	if err := watchTree(w, v.root); err != nil {
		w.Close()
		return nil, err
	}
	ch := make(chan Event, 128)
	go func() {
		defer close(ch)
		defer w.Close()
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-w.Errors:
				if err == nil {
					return
				}
			case ev, ok := <-w.Events:
				if !ok {
					return
				}
				rel, err := filepath.Rel(v.root, ev.Name)
				if err != nil {
					continue
				}
				rel = filepath.ToSlash(rel)
				if rel == "." || skip(rel) {
					continue
				}
				if ev.Has(fsnotify.Create) {
					if st, err := os.Stat(ev.Name); err == nil && st.IsDir() {
						_ = watchTree(w, ev.Name)
					}
				}
				op := "write"
				if ev.Has(fsnotify.Remove) || ev.Has(fsnotify.Rename) {
					op = "remove"
				}
				select {
				case ch <- Event{Path: rel, Op: op}:
				default:
					select {
					case ch <- Event{Op: "rescan"}:
					default:
					}
				}
			}
		}
	}()
	return ch, nil
}

func (v *Vault) abs(rel string) (string, error) {
	rel = filepath.Clean(filepath.FromSlash(rel))
	if rel == "." || rel == "" {
		return "", os.ErrInvalid
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", os.ErrInvalid
	}
	return filepath.Join(v.root, rel), nil
}

func slash(rel string) string {
	return filepath.ToSlash(filepath.Clean(rel))
}

func skip(rel string) bool {
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for _, p := range parts {
		switch p {
		case ".git", ".app", "node_modules":
			return true
		}
	}
	return false
}

func watchTree(w *fsnotify.Watcher, root string) error {
	return filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil || !d.IsDir() {
			return nil
		}
		rel, relErr := filepath.Rel(root, path)
		if relErr == nil && rel != "." && skip(filepath.ToSlash(rel)) {
			return filepath.SkipDir
		}
		return w.Add(path)
	})
}

func replace(tmp, dst string) error {
	if err := os.Rename(tmp, dst); err == nil {
		return nil
	}
	_ = os.Remove(dst)
	return os.Rename(tmp, dst)
}
