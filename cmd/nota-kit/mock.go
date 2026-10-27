package main

import (
	"maps"
	"slices"
)

const mockVault = "Field notes"

var mockRecent = []struct{ Name, Dir string }{
	{"Field notes", `D:\notes`},
	{"Team wiki", `D:\work`},
	{"Journal", `C:\Users\you\Documents`},
}

var mockDirs = map[string]bool{"daily": true, "projects": true, "reading": true}

var mockRoots = []string{"daily", "projects", "reading", "Inbox.md", "Start here.md"}

var mockKids = map[string][]string{
	"daily":    {"daily/2026-10-27.md", "daily/2026-10-28.md"},
	"projects": {"projects/Nota roadmap.md", "projects/Sync design.md"},
	"reading":  {"reading/Local-first software.md"},
}

func mockChildren(p string) []string { return mockKids[p] }

var mockNotes = map[string]string{
	"projects/Nota roadmap.md": `# Nota roadmap

A small offline vault. Notes stay files; AI is optional.

## Now

- [x] Native window with MyGo
- [ ] New look from the UI kit #project
- [ ] Sync over [[Sync design|git and folders]]

## Next

Read ==local-first== ideas in [[Local-first software]], then write them up in the [[Inbox]].

> One accent, hairlines, and room to breathe.
`,
	"projects/Sync design.md":         "# Sync design\n\nGit, a folder, or `nota serve`. Conflicts become a copy: `note (conflict device date).md`.\n\nSee [[Nota roadmap]].\n",
	"reading/Local-first software.md": "# Local-first software\n\nSeven ideals: fast, multi-device, offline, collaboration, longevity, privacy, user control. #reading\n",
	"daily/2026-10-28.md":             "# 2026-10-28\n\n- Reviewed the [[Nota roadmap]]\n- Tried the new palette #idea\n",
	"daily/2026-10-27.md":             "# 2026-10-27\n\n- Sketched the sidebar #idea\n",
	"Inbox.md":                        "# Inbox\n\n- Shortcuts cheat sheet #idea\n",
	"Start here.md":                   "# Start here\n\nThis is a preview vault with made-up notes.\n",
}

// mockPaths returns the notes' paths in order.
func mockPaths() []string { return slices.Sorted(maps.Keys(mockNotes)) }

var mockTags = []struct {
	Tag string
	N   int
}{{"project", 4}, {"idea", 3}, {"reading", 2}, {"daily", 2}, {"sync", 1}}

var mockBacklinks = map[string][]string{
	"projects/Nota roadmap.md":        {"projects/Sync design.md", "daily/2026-10-28.md"},
	"projects/Sync design.md":         {"projects/Nota roadmap.md"},
	"reading/Local-first software.md": {"projects/Nota roadmap.md"},
	"Inbox.md":                        {"projects/Nota roadmap.md"},
}

var mockOutgoing = map[string][]string{
	"projects/Nota roadmap.md": {"projects/Sync design.md", "reading/Local-first software.md", "Inbox.md"},
	"projects/Sync design.md":  {"projects/Nota roadmap.md"},
	"daily/2026-10-28.md":      {"projects/Nota roadmap.md"},
}

// mockGraph places notes in the graph pane, in fractions of its size.
var mockGraph = map[string][2]float32{
	"projects/Nota roadmap.md":        {0.5, 0.45},
	"projects/Sync design.md":         {0.2, 0.2},
	"reading/Local-first software.md": {0.72, 0.22},
	"Inbox.md":                        {0.7, 0.75},
	"daily/2026-10-28.md":             {0.22, 0.72},
}

var mockEdges = [][2]string{
	{"projects/Nota roadmap.md", "projects/Sync design.md"},
	{"projects/Nota roadmap.md", "reading/Local-first software.md"},
	{"projects/Nota roadmap.md", "Inbox.md"},
	{"daily/2026-10-28.md", "projects/Nota roadmap.md"},
}

var mockCommands = []string{"New note", "Open today's note", "Toggle sidebar", "Toggle side panel", "Summarize note", "Index vault for Ask", "Sync now", "Open settings"}

var mockChat = []struct{ Role, Text string }{
	{"user", "Summarize this note"},
	{"assistant", "The roadmap keeps notes as plain files. **Now:** the new look and sync over [[Sync design]]. **Next:** reading on local-first software."},
}

func noEmbed(string) (string, bool) { return "", false }
