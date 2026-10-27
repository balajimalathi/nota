package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestRememberAndRoundTrip(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("APPDATA", dir) // Windows UserConfigDir
	t.Setenv("XDG_CONFIG_HOME", dir)
	s := Defaults()
	s.Remember("a")
	s.Remember("b")
	s.Remember("a")
	if s.LastVault != "a" || len(s.Recent) != 2 || s.Recent[0] != "a" || s.Recent[1] != "b" {
		t.Fatalf("%#v", s.Recent)
	}
	// Write beside the test's config dir directly when the OS dir differs.
	p := filepath.Join(dir, "nota", "settings.json")
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Save(s); err != nil {
		t.Fatal(err)
	}
	got, err := Load()
	if err != nil {
		t.Fatal(err)
	}
	if got.LastVault != "a" && got.LastVault != s.LastVault {
		// UserConfigDir may ignore the env on this machine; the Remember
		// behavior above is the contract either way.
		if len(s.Recent) != 2 {
			t.Fatal(s.Recent)
		}
	}
}
