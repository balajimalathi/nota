// Package sync copies a vault through git, a folder, or a small HTTP server.
// Conflicts keep the local file and write the other side beside it.
package sync

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"time"
	"unicode"
)

// Status is a point-in-time view of sync.
type Status struct {
	Mode      string
	Ahead     int
	Behind    int
	Dirty     []string
	Conflicts []string
	Detail    string
}

// Backend moves notes in and out of a vault.
type Backend interface {
	Status(ctx context.Context) (Status, error)
	Pull(ctx context.Context) error
	Push(ctx context.Context) error
}

// ConflictName is the path used when both sides edited a file.
func ConflictName(path, device string, now time.Time) string {
	ext := filepath.Ext(path)
	base := strings.TrimSuffix(path, ext)
	safe := sanitize(device)
	if safe == "" {
		safe = "other"
	}
	return fmt.Sprintf("%s (conflict %s %s)%s", base, safe, now.UTC().Format("2006-01-02 1504"), ext)
}

func sanitize(s string) string {
	var b strings.Builder
	for _, r := range s {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '-' || r == '_' {
			b.WriteRune(r)
		} else if r == ' ' {
			b.WriteByte('-')
		}
	}
	return b.String()
}
