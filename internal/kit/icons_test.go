package kit

import "testing"

func TestIconsParse(t *testing.T) {
	all := AllIcons()
	if len(all) != 22 {
		t.Fatalf("%d icons, want 22", len(all))
	}
	for _, ic := range all {
		if w, h := ic.SVG.Size(); w != 24 || h != 24 {
			t.Errorf("%s is %vx%v, want 24x24", ic.Name, w, h)
		}
	}
}
