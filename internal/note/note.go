// Package note formats daily notes and template placeholders.
package note

import (
	"path/filepath"
	"strings"
	"time"
)

// DailyRel is the vault-relative path of the daily note for now.
// format is a time layout used as a path, defaulting to daily/2006-01-02.
func DailyRel(format string, now time.Time) string {
	if strings.TrimSpace(format) == "" {
		format = "daily/2006-01-02"
	}
	p := filepath.ToSlash(now.Format(format))
	if !strings.HasSuffix(strings.ToLower(p), ".md") {
		p += ".md"
	}
	return strings.TrimPrefix(p, "/")
}

// ApplyTemplate replaces {{title}}, {{date}} and {{time}} in src.
func ApplyTemplate(src, title string, now time.Time) string {
	return strings.NewReplacer(
		"{{title}}", title,
		"{{date}}", now.Format("2006-01-02"),
		"{{time}}", now.Format("15:04"),
	).Replace(src)
}

// TitleFromName is the note title implied by a file name.
func TitleFromName(rel string) string {
	base := filepath.Base(filepath.FromSlash(rel))
	return strings.TrimSuffix(base, filepath.Ext(base))
}
