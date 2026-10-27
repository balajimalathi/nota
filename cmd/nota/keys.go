package main

import (
	"strings"

	"github.com/egoist/mygo/ui"

	"nota/internal/keymap"
	"nota/internal/kit"
)

func chordMods(ch keymap.Chord) ui.Modifiers {
	var m ui.Modifiers
	if ch.Ctrl || ch.Super {
		m |= ui.Cmd
	}
	if ch.Shift {
		m |= ui.Shift
	}
	if ch.Alt {
		m |= ui.Alt
	}
	return m
}

func namedKey(name string) (ui.Key, bool) {
	letters := [...]ui.Key{
		ui.KeyA, ui.KeyB, ui.KeyC, ui.KeyD, ui.KeyE, ui.KeyF, ui.KeyG,
		ui.KeyH, ui.KeyI, ui.KeyJ, ui.KeyK, ui.KeyL, ui.KeyM, ui.KeyN,
		ui.KeyO, ui.KeyP, ui.KeyQ, ui.KeyR, ui.KeyS, ui.KeyT, ui.KeyU,
		ui.KeyV, ui.KeyW, ui.KeyX, ui.KeyY, ui.KeyZ,
	}
	if len(name) == 1 && name[0] >= 'a' && name[0] <= 'z' {
		return letters[name[0]-'a'], true
	}
	switch name {
	case ",", "comma":
		return ui.KeyComma, true
	case "\\", "backslash":
		return ui.KeyBackslash, true
	default:
		return 0, false
	}
}

// keyLabel is the first chord bound to command, as kit.Kbd keys, or nil.
func (app *App) keyLabel(command string) []string {
	for _, b := range app.bindings {
		ch, ok := keymap.ParseChord(b.Key)
		if b.Command != command || !ok {
			continue
		}
		var keys []string
		if ch.Ctrl || ch.Super { // one key to chordMods
			keys = append(keys, kit.Mod())
		}
		if ch.Shift {
			keys = append(keys, "Shift")
		}
		if ch.Alt {
			keys = append(keys, "Alt")
		}
		return append(keys, strings.ToUpper(ch.Key))
	}
	return nil
}
