// Command nota-kit previews Nota's new look with made-up notes: the
// workspace, the welcome screen, the command palette, settings and the
// kit's parts. It reads and writes nothing.
//
//	go run ./cmd/nota-kit
package main

import (
	"log"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"

	"nota/internal/kit"
)

const (
	screenWorkspace = iota
	screenWelcome
	screenKit
)

type preview struct {
	screen  int
	ws      *workspace
	gallery galleryState
}

// galleryState holds the kit gallery's sample controls (Task 6 builds them).
type galleryState struct {
	text, search, choice string
	on, check            bool
	num                  float64
	tab, iconTab         int
}

func newPreview() *preview {
	return &preview{ws: newWorkspace(), gallery: galleryState{choice: "llama3.2", num: 14, on: true}}
}

func (p *preview) view(c *ui.Context) {
	pal := kit.Apply(c, p.ws.set.themeName(), float32(p.ws.set.font))
	ui.Column(c).Fill().Background(pal.Background).Children(func() {
		p.bar(c)
		switch p.screen {
		default:
			p.ws.view(c, pal)
		}
	})
}

// bar is the preview's own strip, not part of the design: which screen,
// and which appearance. Its dashed edge sets it apart.
func (p *preview) bar(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Height(40).Shrink(0).Padding(0, 12).Gap(12).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(t.Border).BorderStyle(ui.BorderDashed).Children(func() {
		ui.Text(c, "UI kit preview").FontSize(12).FontWeight(600).TextColor(t.TextMuted)
		kit.Tabs(c, &p.screen, nil, "Workspace", "Welcome", "Kit")
		ui.Spacer(c)
		kit.Tabs(c, &p.ws.set.theme, nil, "System", "Light", "Dark")
	})
}

func main() {
	p := newPreview()
	mygo.App.WhenReady(func() {
		mygo.NewWindow(mygo.WindowOptions{
			Title: "Nota - UI kit preview", Width: 1280, Height: 820,
			MinWidth: 900, MinHeight: 560,
			Content: ui.View(p.view),
		})
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
