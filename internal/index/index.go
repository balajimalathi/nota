// Package index keeps links, tags and a BM25 full-text index in memory.
// The cache is a gob file that can be rebuilt from the vault.
package index

import (
	"encoding/gob"
	"hash/fnv"
	"math"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"

	"nota/internal/md"
)

const cacheVersion = 1

// Hit is one search or fuzzy result.
type Hit struct {
	Path    string
	Title   string
	Score   float64
	Snippet string
}

// TagCount is how many notes carry a tag.
type TagCount struct {
	Tag   string
	Count int
}

// Completion is a wikilink suggestion.
type Completion struct {
	Label  string
	Path   string
	Detail string
}

type posting struct {
	Doc uint32
	TF  int
}

type doc struct {
	Path    string
	Title   string
	Aliases []string
	Tags    []string
	Links   []string
	Mtime   int64
	Size    int64
	Hash    uint64
	DocLen  int
	Terms   map[string]int
	Preview string
	out     []uint32
	unres   []string
}

// Index is safe for concurrent readers and one writer at a time.
type Index struct {
	mu      sync.RWMutex
	docs    []*doc
	byPath  map[string]uint32
	byName  map[string][]uint32
	back    map[uint32][]uint32
	post    map[string][]posting
	df      map[string]int
	totalDL int
	free    []uint32
	bulking bool
}

// New returns an empty index.
func New() *Index {
	return &Index{
		byPath: map[string]uint32{},
		byName: map[string][]uint32{},
		back:   map[uint32][]uint32{},
		post:   map[string][]posting{},
		df:     map[string]int{},
	}
}

// Bulk runs fn and rebuilds backlinks once at the end.
func (ix *Index) Bulk(fn func()) {
	ix.mu.Lock()
	ix.bulking = true
	ix.mu.Unlock()
	fn()
	ix.mu.Lock()
	ix.bulking = false
	ix.relinkAll()
	ix.mu.Unlock()
}

// Count is the number of notes.
func (ix *Index) Count() int {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	n := 0
	for _, d := range ix.docs {
		if d != nil {
			n++
		}
	}
	return n
}

// Paths lists note paths.
func (ix *Index) Paths() []string {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	out := make([]string, 0, len(ix.docs))
	for _, d := range ix.docs {
		if d != nil {
			out = append(out, d.Path)
		}
	}
	sort.Strings(out)
	return out
}

// Fresh reports whether path is indexed at this size and modification time.
func (ix *Index) Fresh(path string, mtime time.Time, size int64) bool {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	id, ok := ix.byPath[slash(path)]
	if !ok || ix.docs[id] == nil {
		return false
	}
	d := ix.docs[id]
	return d.Size == size && d.Mtime == mtime.UnixNano()
}

// Upsert indexes the note at path.
func (ix *Index) Upsert(path, src string, mtime time.Time, size int64) {
	path = slash(path)
	sum := hashText(src)
	parsed := md.Parse(src)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	if id, ok := ix.byPath[path]; ok && ix.docs[id] != nil && ix.docs[id].Hash == sum {
		ix.docs[id].Mtime = mtime.UnixNano()
		ix.docs[id].Size = size
		return
	}
	d := ix.build(path, src, parsed, mtime, size, sum)
	var id uint32
	if old, ok := ix.byPath[path]; ok {
		id = old
		ix.unindex(id)
		ix.docs[id] = d
	} else if n := len(ix.free); n > 0 {
		id = ix.free[n-1]
		ix.free = ix.free[:n-1]
		ix.docs[id] = d
	} else {
		id = uint32(len(ix.docs))
		ix.docs = append(ix.docs, d)
	}
	ix.byPath[path] = id
	ix.indexDoc(id, d)
	if !ix.bulking {
		ix.relinkOne(id)
		ix.relinkWaiting(d)
	}
}

// Remove drops a note.
func (ix *Index) Remove(path string) {
	path = slash(path)
	ix.mu.Lock()
	defer ix.mu.Unlock()
	id, ok := ix.byPath[path]
	if !ok || ix.docs[id] == nil {
		return
	}
	incoming := append([]uint32(nil), ix.back[id]...)
	ix.unindex(id)
	delete(ix.byPath, path)
	delete(ix.back, id)
	ix.docs[id] = nil
	ix.free = append(ix.free, id)
	if !ix.bulking {
		for _, src := range incoming {
			if ix.docs[src] != nil {
				ix.relinkOne(src)
			}
		}
	}
}

// Resolve finds the note a wikilink target points at.
func (ix *Index) Resolve(target string) (string, bool) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	id, ok := ix.lookup(target)
	if !ok {
		return "", false
	}
	return ix.docs[id].Path, true
}

// Title is the note's title, or the file stem.
func (ix *Index) Title(path string) string {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	if id, ok := ix.byPath[slash(path)]; ok && ix.docs[id] != nil && ix.docs[id].Title != "" {
		return ix.docs[id].Title
	}
	return stem(path)
}

// Backlinks lists notes that link to path.
func (ix *Index) Backlinks(path string) []Hit {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	id, ok := ix.byPath[slash(path)]
	if !ok {
		return nil
	}
	return ix.hitsOf(ix.back[id])
}

// Outgoing lists resolved and unresolved links from path.
func (ix *Index) Outgoing(path string) (resolved []Hit, unresolved []string) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	id, ok := ix.byPath[slash(path)]
	if !ok || ix.docs[id] == nil {
		return nil, nil
	}
	d := ix.docs[id]
	return ix.hitsOf(d.out), append([]string(nil), d.unres...)
}

// Tags lists tags by descending count.
func (ix *Index) Tags() []TagCount {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	counts := map[string]int{}
	for _, d := range ix.docs {
		if d == nil {
			continue
		}
		seen := map[string]bool{}
		for _, tag := range d.Tags {
			key := strings.ToLower(tag)
			if seen[key] {
				continue
			}
			seen[key] = true
			counts[tag]++
		}
	}
	out := make([]TagCount, 0, len(counts))
	for tag, n := range counts {
		out = append(out, TagCount{Tag: tag, Count: n})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Count == out[j].Count {
			return out[i].Tag < out[j].Tag
		}
		return out[i].Count > out[j].Count
	})
	return out
}

// Tagged lists notes with tag.
func (ix *Index) Tagged(tag string) []Hit {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var ids []uint32
	want := strings.ToLower(tag)
	for id, d := range ix.docs {
		if d == nil {
			continue
		}
		for _, t := range d.Tags {
			if strings.ToLower(t) == want {
				ids = append(ids, uint32(id))
				break
			}
		}
	}
	return ix.hitsOf(ids)
}

// Complete lists notes whose name, path or alias starts with prefix.
func (ix *Index) Complete(prefix string, limit int) []Completion {
	if limit <= 0 {
		limit = 20
	}
	prefix = strings.ToLower(strings.TrimSpace(prefix))
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var out []Completion
	seen := map[string]bool{}
	for _, d := range ix.docs {
		if d == nil {
			continue
		}
		label := stem(d.Path)
		if d.Title != "" {
			label = d.Title
		}
		cands := []string{label, stem(d.Path), strings.TrimSuffix(d.Path, ".md")}
		cands = append(cands, d.Aliases...)
		ok := prefix == ""
		for _, c := range cands {
			if strings.HasPrefix(strings.ToLower(c), prefix) {
				ok = true
				break
			}
		}
		if !ok || seen[d.Path] {
			continue
		}
		seen[d.Path] = true
		out = append(out, Completion{Label: label, Path: d.Path, Detail: d.Path})
	}
	sort.Slice(out, func(i, j int) bool {
		si, sj := scoreName(out[i].Label, prefix), scoreName(out[j].Label, prefix)
		if si == sj {
			return out[i].Label < out[j].Label
		}
		return si > sj
	})
	if len(out) > limit {
		out = out[:limit]
	}
	return out
}

// Fuzzy ranks notes by a subsequence match on the title and path.
func (ix *Index) Fuzzy(query string, limit int) []Hit {
	if limit <= 0 {
		limit = 20
	}
	q := strings.TrimSpace(query)
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	var hits []Hit
	for _, d := range ix.docs {
		if d == nil {
			continue
		}
		name := d.Title
		if name == "" {
			name = stem(d.Path)
		}
		s := scoreName(name, q)
		s += scoreName(d.Path, q) * 0.5
		for _, a := range d.Aliases {
			if alt := scoreName(a, q); alt > s {
				s = alt
			}
		}
		if q != "" && s <= 0 {
			continue
		}
		if q == "" {
			s = 1
		}
		hits = append(hits, Hit{Path: d.Path, Title: name, Score: s, Snippet: d.Preview})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Path < hits[j].Path
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Search ranks notes with BM25. The last token is a prefix unless the
// query ends in a space.
func (ix *Index) Search(query string, limit int) []Hit {
	if limit <= 0 {
		limit = 30
	}
	prefixLast := !strings.HasSuffix(query, " ")
	terms := tokenize(query)
	if len(terms) == 0 {
		return nil
	}
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	n := 0
	for _, d := range ix.docs {
		if d != nil {
			n++
		}
	}
	if n == 0 {
		return nil
	}
	avg := float64(ix.totalDL) / float64(n)
	if avg < 1 {
		avg = 1
	}
	type acc struct {
		score float64
	}
	scores := map[uint32]*acc{}
	const k1, b = 1.2, 0.75
	for i, term := range terms {
		var lists [][]posting
		if i == len(terms)-1 && prefixLast {
			for key, ps := range ix.post {
				if strings.HasPrefix(key, term) {
					lists = append(lists, ps)
				}
			}
		} else if ps, ok := ix.post[term]; ok {
			lists = append(lists, ps)
		}
		for _, ps := range lists {
			df := len(ps)
			idf := math.Log(1 + (float64(n)-float64(df)+0.5)/(float64(df)+0.5))
			for _, p := range ps {
				d := ix.docs[p.Doc]
				if d == nil {
					continue
				}
				tf := float64(p.TF)
				dl := float64(d.DocLen)
				denom := tf + k1*(1-b+b*dl/avg)
				add := idf * (tf * (k1 + 1) / denom)
				a := scores[p.Doc]
				if a == nil {
					a = &acc{}
					scores[p.Doc] = a
				}
				a.score += add
			}
		}
	}
	hits := make([]Hit, 0, len(scores))
	lowQ := strings.ToLower(strings.TrimSpace(query))
	for id, a := range scores {
		d := ix.docs[id]
		if d == nil {
			continue
		}
		if strings.Contains(strings.ToLower(d.Title), lowQ) {
			a.score += 2
		}
		title := d.Title
		if title == "" {
			title = stem(d.Path)
		}
		hits = append(hits, Hit{Path: d.Path, Title: title, Score: a.score, Snippet: snippet(d.Preview, terms)})
	}
	sort.Slice(hits, func(i, j int) bool {
		if hits[i].Score == hits[j].Score {
			return hits[i].Path < hits[j].Path
		}
		return hits[i].Score > hits[j].Score
	})
	if len(hits) > limit {
		hits = hits[:limit]
	}
	return hits
}

// Neighborhood returns notes within depth links of path, and the edges
// among them. A depth of 0 returns every note when there are at most 80.
func (ix *Index) Neighborhood(path string, depth int) (nodes []string, edges [][2]string) {
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	if path == "" || depth <= 0 {
		if ix.count() > 80 {
			return nil, nil
		}
		for _, d := range ix.docs {
			if d != nil {
				nodes = append(nodes, d.Path)
			}
		}
		sort.Strings(nodes)
		edges = ix.edgesBetween(toSet(nodes))
		return nodes, edges
	}
	start, ok := ix.byPath[slash(path)]
	if !ok {
		return nil, nil
	}
	seen := map[uint32]bool{start: true}
	layer := []uint32{start}
	for step := 0; step < depth; step++ {
		var next []uint32
		for _, id := range layer {
			for _, nb := range ix.neighbors(id) {
				if seen[nb] || ix.docs[nb] == nil {
					continue
				}
				seen[nb] = true
				next = append(next, nb)
			}
		}
		layer = next
	}
	for id := range seen {
		nodes = append(nodes, ix.docs[id].Path)
	}
	sort.Strings(nodes)
	return nodes, ix.edgesBetween(toSet(nodes))
}

// Rewrite updates wikilinks in src that resolve to oldPath so they point
// at newPath, keeping headings and aliases.
func (ix *Index) Rewrite(src, oldPath, newPath string) (string, int) {
	oldPath, newPath = slash(oldPath), slash(newPath)
	ix.mu.RLock()
	defer ix.mu.RUnlock()
	label := ix.display(newPath, oldPath)
	return md.RewriteLinks(src, func(target string, _ bool) (string, bool) {
		base, frag := md.SplitTarget(target)
		id, ok := ix.lookup(base)
		if !ok || ix.docs[id] == nil || ix.docs[id].Path != oldPath {
			return "", false
		}
		return label + frag, true
	})
}

// Save writes the gob cache.
func (ix *Index) Save(path string) error {
	ix.mu.RLock()
	snap := ix.snapshot()
	ix.mu.RUnlock()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return err
	}
	tmp := path + ".tmp"
	f, err := os.Create(tmp)
	if err != nil {
		return err
	}
	err = gob.NewEncoder(f).Encode(snap)
	cerr := f.Close()
	if err != nil {
		os.Remove(tmp)
		return err
	}
	if cerr != nil {
		os.Remove(tmp)
		return cerr
	}
	if err := os.Rename(tmp, path); err != nil {
		_ = os.Remove(path)
		if err2 := os.Rename(tmp, path); err2 != nil {
			os.Remove(tmp)
			return err2
		}
	}
	return nil
}

// Load reads a cache. A missing or foreign file yields an empty index.
func Load(path string) (*Index, error) {
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, err
	}
	defer f.Close()
	var snap diskIndex
	if err := gob.NewDecoder(f).Decode(&snap); err != nil || snap.Version != cacheVersion {
		return New(), nil
	}
	ix := New()
	for _, s := range snap.Docs {
		d := &doc{
			Path: s.Path, Title: s.Title, Aliases: s.Aliases, Tags: s.Tags, Links: s.Links,
			Mtime: s.Mtime, Size: s.Size, Hash: s.Hash, DocLen: s.DocLen, Terms: s.Terms, Preview: s.Preview,
		}
		id := uint32(len(ix.docs))
		ix.docs = append(ix.docs, d)
		ix.byPath[s.Path] = id
		ix.indexDoc(id, d)
	}
	ix.relinkAll()
	return ix, nil
}

type diskDoc struct {
	Path, Title, Preview string
	Aliases, Tags, Links []string
	Mtime, Size          int64
	Hash                 uint64
	DocLen               int
	Terms                map[string]int
}

type diskIndex struct {
	Version int
	Docs    []diskDoc
}

func (ix *Index) snapshot() diskIndex {
	s := diskIndex{Version: cacheVersion}
	for _, d := range ix.docs {
		if d == nil {
			continue
		}
		s.Docs = append(s.Docs, diskDoc{
			Path: d.Path, Title: d.Title, Preview: d.Preview,
			Aliases: append([]string(nil), d.Aliases...),
			Tags:    append([]string(nil), d.Tags...),
			Links:   append([]string(nil), d.Links...),
			Mtime:   d.Mtime, Size: d.Size, Hash: d.Hash, DocLen: d.DocLen,
			Terms: d.Terms,
		})
	}
	return s
}

func (ix *Index) build(path, src string, parsed md.Document, mtime time.Time, size int64, sum uint64) *doc {
	title := parsed.Title
	if title == "" {
		title = stem(path)
	}
	var links []string
	for _, l := range parsed.Links {
		base, _ := md.SplitTarget(l.Target)
		if base != "" {
			links = append(links, base)
		}
	}
	tags := make([]string, 0, len(parsed.Tags))
	seen := map[string]bool{}
	for _, t := range parsed.Tags {
		key := strings.ToLower(t.Name)
		if t.Name == "" || seen[key] {
			continue
		}
		seen[key] = true
		tags = append(tags, t.Name)
	}
	terms := map[string]int{}
	for _, tok := range tokenize(src) {
		terms[tok]++
	}
	dl := 0
	for _, n := range terms {
		dl += n
	}
	preview := src
	if i := strings.Index(preview, "\n"); strings.HasPrefix(preview, "---\n") || strings.HasPrefix(preview, "---\r\n") {
		if j := strings.Index(preview[i+1:], "\n---"); j >= 0 {
			preview = preview[i+1+j+4:]
		}
	}
	preview = strings.TrimSpace(preview)
	if len(preview) > 240 {
		preview = preview[:240]
	}
	return &doc{
		Path: path, Title: title, Aliases: parsed.Frontmatter.Aliases, Tags: tags, Links: links,
		Mtime: mtime.UnixNano(), Size: size, Hash: sum, DocLen: dl, Terms: terms, Preview: preview,
	}
}

func (ix *Index) indexDoc(id uint32, d *doc) {
	ix.addNames(id, d)
	ix.totalDL += d.DocLen
	for term, tf := range d.Terms {
		ix.post[term] = append(ix.post[term], posting{Doc: id, TF: tf})
		ix.df[term]++
	}
}

func (ix *Index) unindex(id uint32) {
	d := ix.docs[id]
	if d == nil {
		return
	}
	ix.removeNames(id, d)
	ix.totalDL -= d.DocLen
	for term := range d.Terms {
		old := ix.post[term]
		ps := make([]posting, 0, len(old))
		for _, p := range old {
			if p.Doc != id {
				ps = append(ps, p)
			}
		}
		if len(ps) == 0 {
			delete(ix.post, term)
			delete(ix.df, term)
		} else {
			ix.post[term] = ps
			ix.df[term] = len(ps)
		}
	}
	for _, t := range d.out {
		ix.back[t] = removeID(ix.back[t], id)
	}
}

func (ix *Index) addNames(id uint32, d *doc) {
	add := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return
		}
		ix.byName[s] = append(ix.byName[s], id)
	}
	add(stem(d.Path))
	add(strings.TrimSuffix(d.Path, ".md"))
	add(d.Title)
	for _, a := range d.Aliases {
		add(a)
	}
}

func (ix *Index) removeNames(id uint32, d *doc) {
	drop := func(s string) {
		s = strings.ToLower(strings.TrimSpace(s))
		if s == "" {
			return
		}
		ix.byName[s] = removeID(ix.byName[s], id)
		if len(ix.byName[s]) == 0 {
			delete(ix.byName, s)
		}
	}
	drop(stem(d.Path))
	drop(strings.TrimSuffix(d.Path, ".md"))
	drop(d.Title)
	for _, a := range d.Aliases {
		drop(a)
	}
}

func (ix *Index) relinkAll() {
	ix.back = map[uint32][]uint32{}
	for id, d := range ix.docs {
		if d == nil {
			continue
		}
		d.out = d.out[:0]
		d.unres = d.unres[:0]
		ix.linkOut(uint32(id), d)
	}
}

func (ix *Index) relinkOne(id uint32) {
	d := ix.docs[id]
	if d == nil {
		return
	}
	for _, t := range d.out {
		ix.back[t] = removeID(ix.back[t], id)
	}
	d.out = d.out[:0]
	d.unres = d.unres[:0]
	ix.linkOut(id, d)
}

func (ix *Index) linkOut(id uint32, d *doc) {
	seen := map[uint32]bool{}
	for _, raw := range d.Links {
		tid, ok := ix.lookup(raw)
		if !ok {
			d.unres = append(d.unres, raw)
			continue
		}
		if seen[tid] {
			continue
		}
		seen[tid] = true
		d.out = append(d.out, tid)
		ix.back[tid] = append(ix.back[tid], id)
	}
}

func (ix *Index) relinkWaiting(d *doc) {
	want := map[string]bool{}
	want[strings.ToLower(stem(d.Path))] = true
	want[strings.ToLower(strings.TrimSuffix(d.Path, ".md"))] = true
	want[strings.ToLower(d.Title)] = true
	for _, a := range d.Aliases {
		want[strings.ToLower(a)] = true
	}
	for id, other := range ix.docs {
		if other == nil {
			continue
		}
		for _, u := range other.unres {
			if want[strings.ToLower(u)] {
				ix.relinkOne(uint32(id))
				break
			}
		}
	}
}

func (ix *Index) lookup(target string) (uint32, bool) {
	path, _ := md.SplitTarget(target)
	path = strings.TrimSpace(path)
	path = strings.TrimSuffix(path, ".md")
	if path == "" {
		return 0, false
	}
	slashPath := slash(path)
	if id, ok := ix.byPath[slashPath+".md"]; ok && ix.docs[id] != nil {
		return id, true
	}
	low := strings.ToLower(slashPath)
	var pathHits []uint32
	for p, id := range ix.byPath {
		if ix.docs[id] == nil {
			continue
		}
		if strings.ToLower(strings.TrimSuffix(p, ".md")) == low {
			pathHits = append(pathHits, id)
		}
	}
	if len(pathHits) == 1 {
		return pathHits[0], true
	}
	if strings.Contains(slashPath, "/") {
		return 0, false
	}
	ids := ix.byName[low]
	var hits []uint32
	seen := map[uint32]bool{}
	for _, id := range ids {
		if ix.docs[id] == nil || seen[id] {
			continue
		}
		seen[id] = true
		hits = append(hits, id)
	}
	if len(hits) == 1 {
		return hits[0], true
	}
	return 0, false
}

func (ix *Index) display(path, ignore string) string {
	base := stem(path)
	n := 0
	for _, d := range ix.docs {
		if d == nil || d.Path == ignore || d.Path == path {
			continue
		}
		if strings.EqualFold(stem(d.Path), base) {
			n++
		}
	}
	if n == 0 {
		return base
	}
	return strings.TrimSuffix(path, ".md")
}

func (ix *Index) hitsOf(ids []uint32) []Hit {
	out := make([]Hit, 0, len(ids))
	seen := map[uint32]bool{}
	for _, id := range ids {
		if seen[id] || ix.docs[id] == nil {
			continue
		}
		seen[id] = true
		d := ix.docs[id]
		title := d.Title
		if title == "" {
			title = stem(d.Path)
		}
		out = append(out, Hit{Path: d.Path, Title: title, Snippet: d.Preview})
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Path < out[j].Path })
	return out
}

func (ix *Index) neighbors(id uint32) []uint32 {
	var out []uint32
	if d := ix.docs[id]; d != nil {
		out = append(out, d.out...)
	}
	out = append(out, ix.back[id]...)
	return out
}

func (ix *Index) edgesBetween(want map[string]bool) [][2]string {
	var edges [][2]string
	for _, d := range ix.docs {
		if d == nil || !want[d.Path] {
			continue
		}
		for _, id := range d.out {
			other := ix.docs[id]
			if other == nil || !want[other.Path] || other.Path == d.Path {
				continue
			}
			edges = append(edges, [2]string{d.Path, other.Path})
		}
	}
	return edges
}

func (ix *Index) count() int {
	n := 0
	for _, d := range ix.docs {
		if d != nil {
			n++
		}
	}
	return n
}

func toSet(paths []string) map[string]bool {
	m := make(map[string]bool, len(paths))
	for _, p := range paths {
		m[p] = true
	}
	return m
}

func removeID(ids []uint32, id uint32) []uint32 {
	out := ids[:0]
	for _, x := range ids {
		if x != id {
			out = append(out, x)
		}
	}
	return out
}

func slash(p string) string {
	return filepath.ToSlash(p)
}

func stem(p string) string {
	base := filepath.Base(filepath.FromSlash(p))
	return strings.TrimSuffix(base, filepath.Ext(base))
}

func hashText(s string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(s))
	return h.Sum64()
}

func tokenize(s string) []string {
	var out []string
	var b strings.Builder
	flush := func() {
		if b.Len() == 0 {
			return
		}
		tok := b.String()
		b.Reset()
		if len(tok) < 2 && !isDigits(tok) {
			return
		}
		if stopword(tok) {
			return
		}
		out = append(out, tok)
	}
	for _, r := range strings.ToLower(s) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			b.WriteRune(r)
			continue
		}
		flush()
	}
	flush()
	return out
}

func isDigits(s string) bool {
	if s == "" {
		return false
	}
	for _, r := range s {
		if !unicode.IsDigit(r) {
			return false
		}
	}
	return true
}

func stopword(s string) bool {
	switch s {
	case "the", "and", "for", "with", "that", "this", "from", "are", "was", "were", "but", "not", "you", "your", "have", "has", "had":
		return true
	default:
		return false
	}
}

func snippet(preview string, terms []string) string {
	if preview == "" || len(terms) == 0 {
		return preview
	}
	low := strings.ToLower(preview)
	at := -1
	for _, term := range terms {
		if i := strings.Index(low, term); i >= 0 {
			at = i
			break
		}
	}
	if at < 0 {
		return oneLine(preview)
	}
	start := at - 40
	if start < 0 {
		start = 0
	}
	end := at + 120
	if end > len(preview) {
		end = len(preview)
	}
	s := preview[start:end]
	if start > 0 {
		s = "…" + s
	}
	return oneLine(s)
}

func oneLine(s string) string {
	s = strings.ReplaceAll(s, "\n", " ")
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > 180 {
		s = s[:180] + "…"
	}
	return s
}

func scoreName(name, query string) float64 {
	if query == "" {
		return 1
	}
	n := strings.ToLower(name)
	q := strings.ToLower(query)
	if n == q {
		return 100
	}
	if strings.HasPrefix(n, q) {
		return 80 - float64(len(n)-len(q))*0.1
	}
	if strings.Contains(n, q) {
		return 60
	}
	ni, score, run := 0, 0.0, 0.0
	boundary := true
	for _, r := range q {
		found := false
		for ni < len(n) {
			nr, w := utf8.DecodeRuneInString(n[ni:])
			if nr == r {
				found = true
				run += 2
				score += 1 + run
				if boundary {
					score += 2
				}
				boundary = false
				ni += w
				break
			}
			boundary = !unicode.IsLetter(nr) && !unicode.IsDigit(nr)
			run = 0
			ni += w
		}
		if !found {
			return 0
		}
	}
	return score
}
