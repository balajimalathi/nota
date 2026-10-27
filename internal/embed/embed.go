// Package embed stores note vectors in one flat file and ranks them by cosine.
package embed

import (
	"encoding/binary"
	"hash/fnv"
	"math"
	"os"
	"strings"
	"sync"

	"nota/internal/md"
)

const (
	magic      = "NOTAEMB1"
	maxChunk   = 1200
	fileHeader = 16
)

// Chunk is one piece of a note, split on headings.
type Chunk struct {
	Path    string
	Heading string
	Text    string
	Hash    uint64
}

// Hit is a nearest chunk.
type Hit struct {
	Path    string
	Heading string
	Text    string
	Score   float32
}

// Store is a brute-force vector index. It is safe for concurrent use.
type Store struct {
	mu     sync.Mutex
	dim    int
	chunks []Chunk
	vecs   []float32
	at     map[uint64]int
}

// New returns an empty store.
func New() *Store {
	return &Store{at: map[uint64]int{}}
}

// Dim is the vector width, or 0 before the first insert.
func (s *Store) Dim() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.dim
}

// Len is the number of chunks.
func (s *Store) Len() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.chunks)
}

// Has reports whether hash is stored.
func (s *Store) Has(hash uint64) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.at[hash]
	return ok
}

// Missing returns chunks whose vectors are not stored.
func (s *Store) Missing(chunks []Chunk) []Chunk {
	s.mu.Lock()
	defer s.mu.Unlock()
	var out []Chunk
	for _, c := range chunks {
		if _, ok := s.at[c.Hash]; !ok {
			out = append(out, c)
		}
	}
	return out
}

// Upsert stores vectors aligned with chunks. The first call sets the dimension.
func (s *Store) Upsert(chunks []Chunk, vecs [][]float32) error {
	if len(chunks) != len(vecs) {
		return os.ErrInvalid
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, c := range chunks {
		v := vecs[i]
		if len(v) == 0 {
			continue
		}
		if s.dim == 0 {
			s.dim = len(v)
		}
		if len(v) != s.dim {
			return os.ErrInvalid
		}
		if idx, ok := s.at[c.Hash]; ok {
			copy(s.vecs[idx*s.dim:], v)
			s.chunks[idx] = c
			continue
		}
		s.at[c.Hash] = len(s.chunks)
		s.chunks = append(s.chunks, c)
		s.vecs = append(s.vecs, v...)
	}
	return nil
}

// DropPath removes every chunk from a note.
func (s *Store) DropPath(path string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dim == 0 {
		return
	}
	var chunks []Chunk
	vecs := make([]float32, 0, len(s.vecs))
	at := map[uint64]int{}
	for i, c := range s.chunks {
		if c.Path == path {
			continue
		}
		at[c.Hash] = len(chunks)
		chunks = append(chunks, c)
		vecs = append(vecs, s.vecs[i*s.dim:(i+1)*s.dim]...)
	}
	s.chunks, s.vecs, s.at = chunks, vecs, at
}

// Search returns the k nearest chunks, optionally skipping one note.
func (s *Store) Search(q []float32, k int, skipPath string) []Hit {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.dim == 0 || len(q) != s.dim || k <= 0 {
		return nil
	}
	type pair struct {
		i     int
		score float32
	}
	best := make([]pair, 0, k)
	for i := range s.chunks {
		if skipPath != "" && s.chunks[i].Path == skipPath {
			continue
		}
		score := cosine(q, s.vecs[i*s.dim:(i+1)*s.dim])
		if len(best) < k {
			best = append(best, pair{i, score})
			continue
		}
		worst := 0
		for j := 1; j < len(best); j++ {
			if best[j].score < best[worst].score {
				worst = j
			}
		}
		if score > best[worst].score {
			best[worst] = pair{i, score}
		}
	}
	out := make([]Hit, len(best))
	for n, p := range best {
		c := s.chunks[p.i]
		out[n] = Hit{Path: c.Path, Heading: c.Heading, Text: c.Text, Score: p.score}
	}
	for i := 1; i < len(out); i++ {
		j := i
		for j > 0 && out[j].Score > out[j-1].Score {
			out[j], out[j-1] = out[j-1], out[j]
			j--
		}
	}
	return out
}

// Save writes the flat file.
func (s *Store) Save(path string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	f, err := os.Create(path + ".tmp")
	if err != nil {
		return err
	}
	defer f.Close()
	if _, err := f.WriteString(magic); err != nil {
		return err
	}
	hdr := make([]byte, 8)
	binary.LittleEndian.PutUint32(hdr[0:4], uint32(s.dim))
	binary.LittleEndian.PutUint32(hdr[4:8], uint32(len(s.chunks)))
	if _, err := f.Write(hdr); err != nil {
		return err
	}
	for i, c := range s.chunks {
		if err := writeStr(f, c.Path); err != nil {
			return err
		}
		if err := writeStr(f, c.Heading); err != nil {
			return err
		}
		if err := writeStr(f, c.Text); err != nil {
			return err
		}
		var hash [8]byte
		binary.LittleEndian.PutUint64(hash[:], c.Hash)
		if _, err := f.Write(hash[:]); err != nil {
			return err
		}
		vec := s.vecs[i*s.dim : (i+1)*s.dim]
		buf := make([]byte, 4*len(vec))
		for k, v := range vec {
			binary.LittleEndian.PutUint32(buf[k*4:], math.Float32bits(v))
		}
		if _, err := f.Write(buf); err != nil {
			return err
		}
	}
	if err := f.Close(); err != nil {
		return err
	}
	if err := os.Rename(path+".tmp", path); err != nil {
		_ = os.Remove(path)
		return os.Rename(path+".tmp", path)
	}
	return nil
}

// Load reads a flat file. A missing file is an empty store.
func Load(path string) (*Store, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return New(), nil
		}
		return nil, err
	}
	if len(b) < fileHeader || string(b[:8]) != magic {
		return New(), nil
	}
	dim := int(binary.LittleEndian.Uint32(b[8:12]))
	n := int(binary.LittleEndian.Uint32(b[12:16]))
	s := New()
	s.dim = dim
	off := fileHeader
	for i := 0; i < n; i++ {
		var c Chunk
		var ok bool
		c.Path, off, ok = readStr(b, off)
		if !ok {
			return New(), nil
		}
		c.Heading, off, ok = readStr(b, off)
		if !ok {
			return New(), nil
		}
		c.Text, off, ok = readStr(b, off)
		if !ok {
			return New(), nil
		}
		if off+8 > len(b) {
			return New(), nil
		}
		c.Hash = binary.LittleEndian.Uint64(b[off : off+8])
		off += 8
		if dim == 0 || off+dim*4 > len(b) {
			return New(), nil
		}
		vec := make([]float32, dim)
		for k := 0; k < dim; k++ {
			vec[k] = math.Float32frombits(binary.LittleEndian.Uint32(b[off : off+4]))
			off += 4
		}
		s.at[c.Hash] = len(s.chunks)
		s.chunks = append(s.chunks, c)
		s.vecs = append(s.vecs, vec...)
	}
	return s, nil
}

// Chunks splits a note on headings, then into pieces of at most 1200 runes.
func Chunks(path, src string) []Chunk {
	doc := md.Parse(src)
	lines := strings.Split(src, "\n")
	type sec struct {
		heading string
		text    string
	}
	var secs []sec
	if len(doc.Headings) == 0 {
		secs = []sec{{text: src}}
	} else {
		for i, h := range doc.Headings {
			end := len(lines)
			if i+1 < len(doc.Headings) {
				end = doc.Headings[i+1].Line
			}
			start := h.Line
			if start < 0 {
				start = 0
			}
			if end > len(lines) {
				end = len(lines)
			}
			if start > end {
				start = end
			}
			secs = append(secs, sec{heading: h.Text, text: strings.Join(lines[start:end], "\n")})
		}
	}
	var out []Chunk
	for _, sec := range secs {
		for _, piece := range splitRunes(sec.text, maxChunk) {
			piece = strings.TrimSpace(piece)
			if piece == "" {
				continue
			}
			out = append(out, Chunk{
				Path: path, Heading: sec.heading, Text: piece, Hash: hashChunk(path, sec.heading, piece),
			})
		}
	}
	return out
}

func splitRunes(s string, n int) []string {
	r := []rune(s)
	if len(r) <= n {
		return []string{s}
	}
	var out []string
	for len(r) > 0 {
		end := n
		if end > len(r) {
			end = len(r)
		} else {
			for i := end; i > end/2; i-- {
				if r[i-1] == '\n' {
					end = i
					break
				}
			}
		}
		out = append(out, string(r[:end]))
		r = r[end:]
	}
	return out
}

func hashChunk(path, heading, text string) uint64 {
	h := fnv.New64a()
	_, _ = h.Write([]byte(path))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(heading))
	_, _ = h.Write([]byte{0})
	_, _ = h.Write([]byte(text))
	return h.Sum64()
}

func cosine(a, b []float32) float32 {
	var dot, na, nb float32
	for i := range a {
		dot += a[i] * b[i]
		na += a[i] * a[i]
		nb += b[i] * b[i]
	}
	if na == 0 || nb == 0 {
		return 0
	}
	return dot / (float32(math.Sqrt(float64(na))) * float32(math.Sqrt(float64(nb))))
}

func writeStr(f *os.File, s string) error {
	var n [4]byte
	binary.LittleEndian.PutUint32(n[:], uint32(len(s)))
	if _, err := f.Write(n[:]); err != nil {
		return err
	}
	_, err := f.WriteString(s)
	return err
}

func readStr(b []byte, off int) (string, int, bool) {
	if off+4 > len(b) {
		return "", off, false
	}
	n := int(binary.LittleEndian.Uint32(b[off : off+4]))
	off += 4
	if n < 0 || off+n > len(b) {
		return "", off, false
	}
	return string(b[off : off+n]), off + n, true
}
