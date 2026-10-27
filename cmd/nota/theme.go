package main

import "github.com/egoist/mygo/ui"

func slateTheme() *ui.Theme {
	t := ui.DarkTheme()
	t.Background = ui.Hex("#0f172a")
	t.Surface = ui.Hex("#1e293b")
	t.SurfaceHover = ui.Hex("#334155")
	t.SurfacePressed = ui.Hex("#475569")
	t.Border = ui.Hex("#334155")
	t.Text = ui.Hex("#e2e8f0")
	t.TextMuted = ui.Hex("#94a3b8")
	t.Accent = ui.Hex("#2dd4bf")
	t.AccentHover = ui.Hex("#5eead4")
	t.AccentPressed = ui.Hex("#14b8a6")
	t.AccentText = ui.Hex("#042f2e")
	t.Selection = ui.RGBA(45, 212, 191, 0.35)
	t.Focus = ui.RGBA(45, 212, 191, 0.7)
	return t
}
