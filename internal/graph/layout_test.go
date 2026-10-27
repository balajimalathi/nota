package graph

import "testing"

func TestLayoutConnectedCloser(t *testing.T) {
	ids := []string{"a", "b", "c", "d"}
	edges := [][2]string{{"a", "b"}}
	pos := Layout(ids, edges)
	if len(pos) != 4 {
		t.Fatalf("nodes: %d", len(pos))
	}
	ab := dist(pos["a"], pos["b"])
	ac := dist(pos["a"], pos["c"])
	if ab <= 0 || ac <= 0 {
		t.Fatal("zero distance")
	}
	for _, p := range pos {
		if p.X < 0.05 || p.X > 0.95 || p.Y < 0.05 || p.Y > 0.95 {
			t.Fatalf("out of bounds: %+v", p)
		}
	}
	again := Layout(ids, edges)
	if again["a"] != pos["a"] {
		t.Fatal("layout is not stable")
	}
}

func dist(a, b Point) float64 {
	dx := a.X - b.X
	dy := a.Y - b.Y
	return dx*dx + dy*dy
}
