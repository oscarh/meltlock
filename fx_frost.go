package main

import "math"

// frost: ice crystals grow over the desktop. Each starts with six arms
// that branch at sixty degrees, in the colours of what they cover, while
// the picture underneath and the older crystals slowly fade.
type frost struct {
	*canvas
	tips, born         []tip
	owedFade, owedSeed float64
}

type tip struct {
	x, y   float64
	rot    float64 // the crystal's own turn, radians
	dir    int     // multiples of sixty degrees
	left   float64 // pixels still to grow
	branch float64 // pixels until the next pair of branches
	gen    int
	c      ink
}

const (
	frostSeeds  = 1.3  // new crystals per second
	frostSpeed  = 42.0 // growth in pixels per second
	frostBranch = 20.0 // typical pixels between branches
	frostFade   = 1.2  // fading passes per second
	frostMax    = 6000 // most tips growing at once
)

func newFrost(s *scene) effect {
	k := &frost{canvas: newCanvas(s)}
	k.picture(s, 0.6)
	k.owedSeed = 3 // a few crystals straight away
	return k
}

func (k *frost) step(s *scene, dt float64) {
	for k.owedFade += dt * frostFade; k.owedFade >= 1; k.owedFade-- {
		k.fade(5)
	}
	for k.owedSeed += dt * frostSeeds; k.owedSeed >= 1; k.owedSeed-- {
		x, y := s.bright()
		i := s.cellAt(x, y)
		seed := tip{x: x, y: y, rot: s.rng.Float64() * math.Pi / 3, c: inkOf(mix(s.col[i], text, 0.25))}
		seed.left = (70 + 230*s.rng.Float64()) * (0.5 + 0.5*s.lum[i])
		for seed.dir = 0; seed.dir < 6; seed.dir++ {
			seed.branch = frostBranch * (0.5 + s.rng.Float64())
			k.tips = append(k.tips, seed)
		}
	}
	grown, born := k.tips[:0], k.born[:0]
	for _, t := range k.tips {
		d := math.Min(frostSpeed*dt, t.left)
		a := t.rot + float64(t.dir)*math.Pi/3
		x, y := t.x+math.Cos(a)*d, t.y+math.Sin(a)*d
		k.line(t.x, t.y, x, y, [3]float64{1, 0.85, 0.65}[t.gen]*s.vig[s.cellAt(x, y)], &t.c)
		t.x, t.y, t.left, t.branch = x, y, t.left-d, t.branch-d
		if t.branch <= 0 && t.gen < 2 && len(k.tips)+len(born) < frostMax {
			t.branch = frostBranch * (0.5 + s.rng.Float64())
			side := t
			side.gen, side.left = t.gen+1, t.left*(0.25+0.3*s.rng.Float64())
			side.c = inkOf(mix(s.col[s.cellAt(x, y)], text, 0.1))
			for _, turn := range [2]int{1, 5} {
				side.dir = (t.dir + turn) % 6
				born = append(born, side) // grows from the next step on
			}
		}
		if t.left > 0 && x >= 0 && y >= 0 && x < float64(k.w) && y < float64(k.h) {
			grown = append(grown, t)
		}
	}
	k.tips, k.born = append(grown, born...), born
}
