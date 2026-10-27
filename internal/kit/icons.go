package kit

import (
	"embed"

	"github.com/egoist/mygo/ui"
)

//go:embed icons/*.svg
var iconFiles embed.FS

// NamedIcon is an icon with its Lucide name.
type NamedIcon struct {
	Name string
	SVG  *ui.SVG
}

var named []NamedIcon

func loadIcon(name string) *ui.SVG {
	b, err := iconFiles.ReadFile("icons/" + name + ".svg")
	if err != nil {
		panic("kit: no icon " + name)
	}
	s := ui.MustParseSVG(b)
	named = append(named, NamedIcon{name, s})
	return s
}

// Icons from Lucide (lucide-static 0.468.0, ISC license, see icons/LICENSE).
var (
	IconNote        = loadIcon("file-text")
	IconFolder      = loadIcon("folder")
	IconOpen        = loadIcon("folder-open")
	IconSearch      = loadIcon("search")
	IconPlus        = loadIcon("plus")
	IconCommand     = loadIcon("command")
	IconSettings    = loadIcon("settings")
	IconSync        = loadIcon("refresh-cw")
	IconAI          = loadIcon("sparkles")
	IconLink        = loadIcon("link")
	IconOutline     = loadIcon("list-tree")
	IconTag         = loadIcon("hash")
	IconGraph       = loadIcon("waypoints")
	IconSidebar     = loadIcon("panel-left")
	IconInspector   = loadIcon("panel-right")
	IconClose       = loadIcon("x")
	IconDaily       = loadIcon("calendar-days")
	IconRead        = loadIcon("book-open")
	IconSource      = loadIcon("code")
	IconLive        = loadIcon("pen-line")
	IconSend        = loadIcon("arrow-up")
	IconChevronDown = loadIcon("chevron-down")
)

// AllIcons returns every icon of the kit with its Lucide name.
func AllIcons() []NamedIcon { return named }
