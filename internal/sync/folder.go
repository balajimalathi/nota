package sync

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"nota/internal/manifest"
)

// Folder syncs a vault with another directory using manifest hashes.
type Folder struct {
	Local  string
	Remote string
	Device string
	Now    func() time.Time
}

func (f *Folder) now() time.Time {
	if f.Now != nil {
		return f.Now()
	}
	return time.Now()
}

func (f *Folder) Status(ctx context.Context) (Status, error) {
	local, remote, base, err := f.snaps()
	if err != nil {
		return Status{}, err
	}
	st := Status{Mode: "folder"}
	st.Dirty, st.Behind, st.Conflicts = summarize(local, remote, base)
	st.Detail = f.Remote
	return st, nil
}

func (f *Folder) Pull(ctx context.Context) error {
	return f.merge(f.Remote, f.Local, true)
}

func (f *Folder) Push(ctx context.Context) error {
	return f.merge(f.Local, f.Remote, false)
}

func (f *Folder) snaps() (local, remote, base *manifest.Manifest, err error) {
	local, err = manifest.Scan(f.Local, f.Device)
	if err != nil {
		return nil, nil, nil, err
	}
	remote, err = manifest.Scan(f.Remote, "")
	if err != nil {
		return nil, nil, nil, err
	}
	base, err = manifest.Load(f.basePath())
	if err != nil {
		if os.IsNotExist(err) {
			base = &manifest.Manifest{}
		} else {
			return nil, nil, nil, err
		}
	}
	return local, remote, base, nil
}

func (f *Folder) basePath() string {
	return filepath.Join(f.Local, ".app", "sync-base.json")
}

func (f *Folder) merge(srcRoot, dstRoot string, pull bool) error {
	src, err := manifest.Scan(srcRoot, "")
	if err != nil {
		return err
	}
	dst, err := manifest.Scan(dstRoot, f.Device)
	if err != nil {
		return err
	}
	base, err := manifest.Load(f.basePath())
	if err != nil {
		if os.IsNotExist(err) {
			base = &manifest.Manifest{}
		} else {
			return err
		}
	}
	if err := apply(srcRoot, dstRoot, src, dst, base, f.Device, f.now()); err != nil {
		return err
	}
	var next *manifest.Manifest
	if pull {
		next, err = manifest.Scan(f.Local, f.Device)
	} else {
		next, err = manifest.Scan(f.Local, f.Device)
	}
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(f.basePath()), 0o755); err != nil {
		return err
	}
	return next.Save(f.basePath())
}

func apply(srcRoot, dstRoot string, src, dst, base *manifest.Manifest, device string, now time.Time) error {
	sm, dm, bm := src.Map(), dst.Map(), base.Map()
	for path, s := range sm {
		d, dok := dm[path]
		b, bok := bm[path]
		switch {
		case dok && d.Hash == s.Hash:
			continue
		case dok && bok && d.Hash == b.Hash:
			if err := copyFile(srcRoot, dstRoot, path); err != nil {
				return err
			}
		case !dok && (!bok || b.Hash == ""):
			if err := copyFile(srcRoot, dstRoot, path); err != nil {
				return err
			}
		case !dok && bok && s.Hash != b.Hash:
			if err := copyConflict(srcRoot, dstRoot, path, device, now); err != nil {
				return err
			}
		case dok && s.Hash != d.Hash:
			if bok && s.Hash == b.Hash {
				continue
			}
			if err := copyConflict(srcRoot, dstRoot, path, device, now); err != nil {
				return err
			}
		}
	}
	for path, d := range dm {
		if _, ok := sm[path]; ok {
			continue
		}
		b, bok := bm[path]
		if bok && d.Hash == b.Hash {
			abs := filepath.Join(dstRoot, filepath.FromSlash(path))
			if err := os.Remove(abs); err != nil && !os.IsNotExist(err) {
				return err
			}
		}
	}
	return nil
}

func copyFile(srcRoot, dstRoot, rel string) error {
	b, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	dst := filepath.Join(dstRoot, filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func copyConflict(srcRoot, dstRoot, rel, device string, now time.Time) error {
	b, err := os.ReadFile(filepath.Join(srcRoot, filepath.FromSlash(rel)))
	if err != nil {
		return err
	}
	name := ConflictName(rel, device, now)
	dst := filepath.Join(dstRoot, filepath.FromSlash(name))
	if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
		return err
	}
	return os.WriteFile(dst, b, 0o644)
}

func summarize(local, remote, base *manifest.Manifest) (dirty []string, behind int, conflicts []string) {
	lm, rm, bm := local.Map(), remote.Map(), base.Map()
	for path, l := range lm {
		r, rok := rm[path]
		b, bok := bm[path]
		switch {
		case rok && r.Hash == l.Hash:
			continue
		case bok && l.Hash != b.Hash && rok && r.Hash != b.Hash && r.Hash != l.Hash:
			conflicts = append(conflicts, path)
		case !bok && rok && r.Hash != l.Hash:
			conflicts = append(conflicts, path)
		case bok && l.Hash != b.Hash:
			dirty = append(dirty, path)
		case rok && r.Hash != l.Hash:
			behind++
		case !rok:
			dirty = append(dirty, path)
		}
	}
	for path := range rm {
		if _, ok := lm[path]; !ok {
			behind++
		}
	}
	return dirty, behind, conflicts
}
