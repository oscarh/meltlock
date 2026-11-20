package main

import "math"

// contour: the desktop as a map. Contour lines follow the brightness of
// the picture, spread outwards from its bright parts like slow rings, and
// bend as the ground beneath them shifts.
type contour struct {
	*canvas
	soft  []float64
	field []float64 // height of every cell, in contour lines
}

const (
	contourLines = 11.0 // lines between black and white
	contourDrift = 0.07 // lines per second that the rings travel
	contourHeave = 0.04 // how fast the ground shifts
)

func newContour(s *scene) effect {
	return &contour{canvas: newCanvas(s), soft: s.blur(s.soft()), field: make([]float64, s.cols*s.rows)}
}

func (k *contour) step(s *scene, dt float64) {
	k.clear()
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			ground := 0.6 * noise3(float64(c)/22, float64(r)/22, s.t*contourHeave)
			k.field[r*s.cols+c] = contourLines*(k.soft[r*s.cols+c]+ground) + s.t*contourDrift
		}
	}
	// Each square between four cell centres gets the pieces of the lines
	// that cross it.
	for r := 0; r < s.rows-1; r++ {
		for c := 0; c < s.cols-1; c++ {
			i := r*s.cols + c
			v := [4]float64{k.field[i], k.field[i+1], k.field[i+s.cols+1], k.field[i+s.cols]} // clockwise
			lo, hi := math.Min(math.Min(v[0], v[1]), math.Min(v[2], v[3])), math.Max(math.Max(v[0], v[1]), math.Max(v[2], v[3]))
			first := int(math.Floor(lo)) + 1
			for level := first; level <= int(math.Floor(hi)) && level < first+3; level++ {
				k.cross(s, i, c, r, &v, level)
			}
		}
	}
}

// cross draws one contour line's way through a square.
func (k *contour) cross(s *scene, i, c, r int, v *[4]float64, level int) {
	corner := [4][2]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}}
	var pts [4][2]float64
	n := 0
	l := float64(level)
	for e := 0; e < 4; e++ {
		a, b := v[e], v[(e+1)%4]
		if (a < l) == (b < l) {
			continue
		}
		t := (l - a) / (b - a)
		p, q := corner[e], corner[(e+1)%4]
		pts[n] = [2]float64{(float64(c) + 0.5 + p[0] + (q[0]-p[0])*t) * cell, (float64(r) + 0.5 + p[1] + (q[1]-p[1])*t) * cell}
		n++
	}
	// Every fourth line is drawn stronger, as on a map.
	col, alpha := inkOf(s.col[i]), (0.4+0.6*s.lum[i])*s.vig[i]
	if level%4 == 0 {
		col, alpha = inkOf(mix(s.col[i], text, 0.45)), 1.0
	}
	for p := 0; p+1 < n; p += 2 {
		k.line(pts[p][0], pts[p][1], pts[p+1][0], pts[p+1][1], alpha, &col)
	}
}
