// Package keymap reads VS Code keybindings, comments included.
package keymap

import (
	"encoding/json"
	"strings"
)

// Binding maps a chord to a command id.
type Binding struct {
	Key     string `json:"key"`
	Command string `json:"command"`
	When    string `json:"when,omitempty"`
}

// Chord is a key press. Key is a lower-case name such as "o" or "comma".
type Chord struct {
	Ctrl  bool
	Shift bool
	Alt   bool
	Super bool
	Key   string
}

// Default is the built-in map.
func Default() []Binding {
	return []Binding{
		{Key: "ctrl+o", Command: "quickOpen"},
		{Key: "ctrl+p", Command: "commandPalette"},
		{Key: "ctrl+shift+f", Command: "search"},
		{Key: "ctrl+n", Command: "newNote"},
		{Key: "ctrl+s", Command: "save"},
		{Key: "ctrl+w", Command: "closeTab"},
		{Key: "ctrl+b", Command: "toggleSidebar"},
		{Key: "ctrl+shift+b", Command: "toggleRight"},
		{Key: "ctrl+e", Command: "cycleMode"},
		{Key: "ctrl+shift+d", Command: "dailyNote"},
		{Key: "ctrl+,", Command: "settings"},
		{Key: "ctrl+shift+g", Command: "graph"},
	}
}

// Parse reads a keybindings JSON array. Line comments and trailing commas
// are allowed, as in VS Code.
func Parse(data []byte) ([]Binding, error) {
	clean := stripJSONC(string(data))
	if strings.TrimSpace(clean) == "" {
		return nil, nil
	}
	var out []Binding
	if err := json.Unmarshal([]byte(clean), &out); err != nil {
		return nil, err
	}
	return out, nil
}

// ParseChord reads a chord such as "ctrl+shift+f" or "cmd+p".
func ParseChord(s string) (Chord, bool) {
	var c Chord
	parts := strings.Split(strings.ToLower(strings.TrimSpace(s)), "+")
	if len(parts) == 0 || parts[0] == "" {
		return c, false
	}
	for _, p := range parts {
		switch strings.TrimSpace(p) {
		case "ctrl", "control":
			c.Ctrl = true
		case "shift":
			c.Shift = true
		case "alt", "opt", "option":
			c.Alt = true
		case "cmd", "command", "meta", "super", "win":
			c.Super = true
		case "":
			return c, false
		default:
			if c.Key != "" {
				return Chord{}, false
			}
			c.Key = strings.TrimSpace(p)
		}
	}
	if c.Key == "" {
		return c, false
	}
	return c, true
}

// Lookup returns the command for a press. cmdIsCtrl treats Super as Ctrl,
// which is how the app maps the platform command key.
func Lookup(bindings []Binding, press Chord, cmdIsCtrl bool) (string, bool) {
	for _, b := range bindings {
		ch, ok := ParseChord(b.Key)
		if !ok || b.Command == "" {
			continue
		}
		if cmdIsCtrl && ch.Super && !ch.Ctrl {
			ch.Ctrl = true
			ch.Super = false
		}
		if ch == press {
			return b.Command, true
		}
	}
	return "", false
}

func stripJSONC(s string) string {
	var b strings.Builder
	b.Grow(len(s))
	inStr, esc, line, block := false, false, false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if line {
			if c == '\n' {
				line = false
				b.WriteByte(c)
			}
			continue
		}
		if block {
			if c == '*' && i+1 < len(s) && s[i+1] == '/' {
				block = false
				i++
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '/' {
			line = true
			i++
			continue
		}
		if c == '/' && i+1 < len(s) && s[i+1] == '*' {
			block = true
			i++
			continue
		}
		b.WriteByte(c)
	}
	return stripTrailingCommas(b.String())
}

func stripTrailingCommas(s string) string {
	var b strings.Builder
	inStr, esc := false, false
	for i := 0; i < len(s); i++ {
		c := s[i]
		if inStr {
			b.WriteByte(c)
			if esc {
				esc = false
				continue
			}
			if c == '\\' {
				esc = true
				continue
			}
			if c == '"' {
				inStr = false
			}
			continue
		}
		if c == '"' {
			inStr = true
			b.WriteByte(c)
			continue
		}
		if c == ',' {
			j := i + 1
			for j < len(s) && (s[j] == ' ' || s[j] == '\n' || s[j] == '\r' || s[j] == '\t') {
				j++
			}
			if j < len(s) && (s[j] == ']' || s[j] == '}') {
				continue
			}
		}
		b.WriteByte(c)
	}
	return b.String()
}
