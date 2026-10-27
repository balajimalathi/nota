// Package manifest records a content hash for every file in a vault.
// Sync uses it later without changing how notes are stored.
package manifest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
)

// Entry is one file.
type Entry struct {
	Path  string `json:"path"`
	Hash  string `json:"hash"`
	Mtime int64  `json:"mtime"`
	Size  int64  `json:"size"`
}

// Manifest is a snapshot of a vault.
type Manifest struct {
	Device  string  `json:"device"`
	Updated int64   `json:"updated"`
	Files   []Entry `json:"files"`
}

// Scan hashes regular files, skipping .git and .app.
func Scan(root, device string) (*Manifest, error) {
	m := &Manifest{Device: device, Updated: time.Now().Unix()}
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		rel, err := filepath.Rel(root, path)
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
		if d.IsDir() {
			return nil
		}
		info, err := d.Info()
		if err != nil || !info.Mode().IsRegular() {
			return nil
		}
		hash, err := hashFile(path)
		if err != nil {
			return nil
		}
		m.Files = append(m.Files, Entry{
			Path: rel, Hash: hash, Mtime: info.ModTime().UnixNano(), Size: info.Size(),
		})
		return nil
	})
	sort.Slice(m.Files, func(i, j int) bool { return m.Files[i].Path < m.Files[j].Path })
	return m, err
}

// Map indexes files by path.
func (m *Manifest) Map() map[string]Entry {
	if m == nil {
		return map[string]Entry{}
	}
	out := make(map[string]Entry, len(m.Files))
	for _, e := range m.Files {
		out[e.Path] = e
	}
	return out
}

// Change is one path that differs between two snapshots.
type Change struct {
	Path string
	Kind string // "add", "update", "delete"
}

// Diff lists paths whose hash changed from base to next.
func Diff(base, next *Manifest) []Change {
	b, n := base.Map(), next.Map()
	var out []Change
	for path, e := range n {
		old, ok := b[path]
		if !ok {
			out = append(out, Change{Path: path, Kind: "add"})
			continue
		}
		if old.Hash != e.Hash {
			out = append(out, Change{Path: path, Kind: "update"})
		}
	}
	for path := range b {
		if _, ok := n[path]; !ok {
			out = append(out, Change{Path: path, Kind: "delete"})
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

// Load reads a manifest file.
func Load(path string) (*Manifest, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var m Manifest
	if err := json.Unmarshal(b, &m); err != nil {
		return nil, err
	}
	return &m, nil
}

// Save writes the manifest.
func (m *Manifest) Save(path string) error {
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	b, err := json.MarshalIndent(m, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(b, '\n'), 0o644)
}

func hashFile(path string) (string, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:]), nil
}

func skip(rel string) bool {
	for _, p := range strings.Split(rel, "/") {
		if p == ".git" || p == ".app" {
			return true
		}
	}
	return false
}
