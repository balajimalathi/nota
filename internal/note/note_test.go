package note

import (
	"testing"
	"time"
)

func TestDailyAndTemplate(t *testing.T) {
	now := time.Date(2026, 10, 27, 15, 4, 0, 0, time.UTC)
	if got := DailyRel("", now); got != "daily/2026-10-27.md" {
		t.Fatalf("default daily: %s", got)
	}
	if got := DailyRel("journal/2006/01/02", now); got != "journal/2026/10/27.md" {
		t.Fatalf("custom daily: %s", got)
	}
	got := ApplyTemplate("# {{title}}\n{{date}} {{time}}", "Hello", now)
	if got != "# Hello\n2026-10-27 15:04" {
		t.Fatalf("template: %q", got)
	}
	if TitleFromName("a/b.md") != "b" {
		t.Fatal("title")
	}
}
