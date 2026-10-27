// Package graph lays out note links for the graph view.
package graph

import "math"

// Point is a node position in the unit square, inset from the edges.
type Point struct{ X, Y float64 }

// Layout places nodes deterministically. Edges pull connected notes together.
// Coordinates are in (0, 1).
func Layout(ids []string, edges [][2]string) map[string]Point {
	n := len(ids)
	out := make(map[string]Point, n)
	if n == 0 {
		return out
	}
	pos := make([]Point, n)
	at := make(map[string]int, n)
	for i, id := range ids {
		at[id] = i
		ang := 2 * math.Pi * float64(i) / float64(n)
		pos[i] = Point{X: 0.5 + 0.38*math.Cos(ang), Y: 0.5 + 0.38*math.Sin(ang)}
	}
	type pair struct{ a, b int }
	var links []pair
	for _, e := range edges {
		a, aok := at[e[0]]
		b, bok := at[e[1]]
		if aok && bok && a != b {
			links = append(links, pair{a, b})
		}
	}
	for iter := 0; iter < 40; iter++ {
		dx := make([]float64, n)
		dy := make([]float64, n)
		for i := 0; i < n; i++ {
			for j := i + 1; j < n; j++ {
				rx := pos[i].X - pos[j].X
				ry := pos[i].Y - pos[j].Y
				dist := math.Hypot(rx, ry)
				if dist < 0.02 {
					dist = 0.02
				}
				f := 0.004 / (dist * dist)
				dx[i] += f * rx / dist
				dy[i] += f * ry / dist
				dx[j] -= f * rx / dist
				dy[j] -= f * ry / dist
			}
			dx[i] += (0.5 - pos[i].X) * 0.01
			dy[i] += (0.5 - pos[i].Y) * 0.01
		}
		for _, e := range links {
			rx := pos[e.b].X - pos[e.a].X
			ry := pos[e.b].Y - pos[e.a].Y
			dx[e.a] += rx * 0.04
			dy[e.a] += ry * 0.04
			dx[e.b] -= rx * 0.04
			dy[e.b] -= ry * 0.04
		}
		for i := range pos {
			pos[i].X = clamp(pos[i].X+dx[i], 0.06, 0.94)
			pos[i].Y = clamp(pos[i].Y+dy[i], 0.08, 0.92)
		}
	}
	for i, id := range ids {
		out[id] = pos[i]
	}
	return out
}

func clamp(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}
