package sync

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"nota/internal/manifest"
)

// HTTP syncs a vault with a server from Handler.
type HTTP struct {
	Local   string
	BaseURL string
	Token   string
	Device  string
	Client  *http.Client
	Now     func() time.Time
}

func (h *HTTP) client() *http.Client {
	if h.Client != nil {
		return h.Client
	}
	return http.DefaultClient
}

func (h *HTTP) now() time.Time {
	if h.Now != nil {
		return h.Now()
	}
	return time.Now()
}

func (h *HTTP) Status(ctx context.Context) (Status, error) {
	local, err := manifest.Scan(h.Local, h.Device)
	if err != nil {
		return Status{}, err
	}
	remote, err := h.remoteManifest(ctx)
	if err != nil {
		return Status{}, err
	}
	base, _ := manifest.Load(h.basePath())
	if base == nil {
		base = &manifest.Manifest{}
	}
	dirty, behind, conflicts := summarize(local, remote, base)
	return Status{Mode: "http", Dirty: dirty, Behind: behind, Conflicts: conflicts, Detail: h.BaseURL}, nil
}

func (h *HTTP) Pull(ctx context.Context) error {
	remote, err := h.remoteManifest(ctx)
	if err != nil {
		return err
	}
	return h.merge(ctx, remote, true)
}

func (h *HTTP) Push(ctx context.Context) error {
	remote, err := h.remoteManifest(ctx)
	if err != nil {
		return err
	}
	return h.merge(ctx, remote, false)
}

func (h *HTTP) merge(ctx context.Context, remote *manifest.Manifest, pull bool) error {
	local, err := manifest.Scan(h.Local, h.Device)
	if err != nil {
		return err
	}
	base, err := manifest.Load(h.basePath())
	if err != nil {
		if os.IsNotExist(err) {
			base = &manifest.Manifest{}
		} else {
			return err
		}
	}
	lm, rm, bm := local.Map(), remote.Map(), base.Map()
	if pull {
		if err := h.pullFiles(ctx, lm, rm, bm); err != nil {
			return err
		}
	} else if err := h.pushFiles(ctx, lm, rm, bm); err != nil {
		return err
	}
	next, err := manifest.Scan(h.Local, h.Device)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(h.basePath()), 0o755); err != nil {
		return err
	}
	return next.Save(h.basePath())
}

func (h *HTTP) pullFiles(ctx context.Context, local, remote, base map[string]manifest.Entry) error {
	for path, s := range remote {
		d, dok := local[path]
		b, bok := base[path]
		take := false
		conflict := false
		switch {
		case dok && d.Hash == s.Hash:
		case dok && bok && d.Hash == b.Hash:
			take = true
		case !dok && (!bok || b.Hash == ""):
			take = true
		case !dok && bok && s.Hash != b.Hash:
			conflict = true
		case dok && s.Hash != d.Hash && !(bok && s.Hash == b.Hash):
			conflict = true
		}
		if !take && !conflict {
			continue
		}
		body, err := h.getFile(ctx, path)
		if err != nil {
			return err
		}
		name := path
		if conflict {
			name = ConflictName(path, h.Device, h.now())
		}
		dst := filepath.Join(h.Local, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(dst, body, 0o644); err != nil {
			return err
		}
	}
	for path, d := range local {
		if _, ok := remote[path]; ok {
			continue
		}
		if b, ok := base[path]; ok && d.Hash == b.Hash {
			_ = os.Remove(filepath.Join(h.Local, filepath.FromSlash(path)))
		}
	}
	return nil
}

func (h *HTTP) pushFiles(ctx context.Context, local, remote, base map[string]manifest.Entry) error {
	for path, l := range local {
		r, rok := remote[path]
		b, bok := base[path]
		switch {
		case rok && r.Hash == l.Hash:
			continue
		case rok && bok && r.Hash == b.Hash:
			if err := h.putFile(ctx, path); err != nil {
				return err
			}
		case !rok && (!bok || b.Hash == ""):
			if err := h.putFile(ctx, path); err != nil {
				return err
			}
		case rok && l.Hash != r.Hash && !(bok && l.Hash == b.Hash):
			body, err := os.ReadFile(filepath.Join(h.Local, filepath.FromSlash(path)))
			if err != nil {
				return err
			}
			if err := h.putBytes(ctx, ConflictName(path, h.Device, h.now()), body); err != nil {
				return err
			}
		}
	}
	for path, r := range remote {
		if _, ok := local[path]; ok {
			continue
		}
		if b, ok := base[path]; ok && r.Hash == b.Hash {
			if err := h.deleteFile(ctx, path); err != nil {
				return err
			}
		}
	}
	return nil
}

func (h *HTTP) basePath() string {
	return filepath.Join(h.Local, ".app", "sync-base.json")
}

func (h *HTTP) remoteManifest(ctx context.Context) (*manifest.Manifest, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, strings.TrimRight(h.BaseURL, "/")+"/v1/manifest", nil)
	if err != nil {
		return nil, err
	}
	h.auth(req)
	res, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("manifest: %s", res.Status)
	}
	var m manifest.Manifest
	if err := json.NewDecoder(res.Body).Decode(&m); err != nil {
		return nil, err
	}
	return &m, nil
}

func (h *HTTP) getFile(ctx context.Context, path string) ([]byte, error) {
	u := strings.TrimRight(h.BaseURL, "/") + "/v1/file?path=" + url.QueryEscape(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	h.auth(req)
	res, err := h.client().Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return nil, fmt.Errorf("get %s: %s", path, res.Status)
	}
	return io.ReadAll(res.Body)
}

func (h *HTTP) putFile(ctx context.Context, path string) error {
	b, err := os.ReadFile(filepath.Join(h.Local, filepath.FromSlash(path)))
	if err != nil {
		return err
	}
	return h.putBytes(ctx, path, b)
}

func (h *HTTP) putBytes(ctx context.Context, path string, body []byte) error {
	u := strings.TrimRight(h.BaseURL, "/") + "/v1/file?path=" + url.QueryEscape(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, u, bytes.NewReader(body))
	if err != nil {
		return err
	}
	h.auth(req)
	res, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("put %s: %s", path, res.Status)
	}
	return nil
}

func (h *HTTP) deleteFile(ctx context.Context, path string) error {
	u := strings.TrimRight(h.BaseURL, "/") + "/v1/file?path=" + url.QueryEscape(path)
	req, err := http.NewRequestWithContext(ctx, http.MethodDelete, u, nil)
	if err != nil {
		return err
	}
	h.auth(req)
	res, err := h.client().Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	if res.StatusCode >= 300 {
		return fmt.Errorf("delete %s: %s", path, res.Status)
	}
	return nil
}

func (h *HTTP) auth(req *http.Request) {
	if h.Token != "" {
		req.Header.Set("Authorization", "Bearer "+h.Token)
	}
}
