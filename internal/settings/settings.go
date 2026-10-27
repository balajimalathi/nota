// Package settings stores preferences in the user config directory.
package settings

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
)

// Settings is the app's preferences.
type Settings struct {
	Theme        string   `json:"theme"`
	FontSize     float32  `json:"fontSize"`
	SoftWrap     bool     `json:"softWrap"`
	AIEndpoint   string   `json:"aiEndpoint"`
	AIChatModel  string   `json:"aiChatModel"`
	AIEmbedModel string   `json:"aiEmbedModel"`
	AIAPIKey     string   `json:"aiAPIKey,omitempty"`
	Recent       []string `json:"recent"`
	LastVault    string   `json:"lastVault"`
	DailyFormat  string   `json:"dailyFormat"`
	TemplatesDir string   `json:"templatesDir"`
	SyncMode     string   `json:"syncMode"`
	SyncRemote   string   `json:"syncRemote"`
	SyncToken    string   `json:"syncToken,omitempty"`
	Device       string   `json:"device"`
}

// Defaults is a new install.
func Defaults() Settings {
	host, _ := os.Hostname()
	if host == "" {
		host = "this-device"
	}
	return Settings{
		Theme:        "system",
		FontSize:     15,
		SoftWrap:     true,
		AIEndpoint:   "http://localhost:11434/v1",
		AIChatModel:  "llama3.2",
		AIEmbedModel: "nomic-embed-text",
		DailyFormat:  "daily/2006-01-02",
		TemplatesDir: "templates",
		SyncMode:     "off",
		Device:       host,
	}
}

// Dir is the config directory, created if needed.
func Dir() (string, error) {
	base, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	dir := filepath.Join(base, "nota")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	return dir, nil
}

// Path is settings.json.
func Path() (string, error) {
	dir, err := Dir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "settings.json"), nil
}

// Load reads settings, or Defaults when the file is missing.
func Load() (Settings, error) {
	s := Defaults()
	p, err := Path()
	if err != nil {
		return s, err
	}
	b, err := os.ReadFile(p)
	if err != nil {
		if os.IsNotExist(err) {
			return s, nil
		}
		return s, err
	}
	if err := json.Unmarshal(b, &s); err != nil {
		return Defaults(), err
	}
	if s.AIEndpoint == "" {
		s.AIEndpoint = Defaults().AIEndpoint
	}
	if s.Device == "" {
		s.Device = Defaults().Device
	}
	if s.DailyFormat == "" {
		s.DailyFormat = Defaults().DailyFormat
	}
	if s.TemplatesDir == "" {
		s.TemplatesDir = "templates"
	}
	if s.FontSize <= 0 {
		s.FontSize = 15
	}
	return s, nil
}

// Save writes settings.
func Save(s Settings) error {
	p, err := Path()
	if err != nil {
		return err
	}
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(p, append(b, '\n'), 0o644)
}

// Remember moves vault to the front of the recent list.
func (s *Settings) Remember(vault string) {
	vault = strings.TrimSpace(vault)
	if vault == "" {
		return
	}
	s.LastVault = vault
	next := []string{vault}
	for _, r := range s.Recent {
		if r != vault && len(next) < 10 {
			next = append(next, r)
		}
	}
	s.Recent = next
}
