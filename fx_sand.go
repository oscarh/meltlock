package main

import "math"

// Grain states.
const (
	empty = iota
	stuck // still part of the picture
	loose // falling or lying in a heap
)

// sand: the bright parts of the picture are grains. They come loose a few
// at a time, fall, and pile up at the bottom in heaps.
type sand struct {
	state   []uint8
	lum     []float64
	col     []rgb
	back    []float64 // faint picture behind the grains
	owed    float64   // simulation ticks owed
	release float64   // grains to release per tick
}

const sandTick = 1.0 / 24

func newSand(s *scene) effect {
	n := s.cols * s.rows
	f := &sand{state: make([]uint8, n), lum: make([]float64, n), col: make([]rgb, n), back: make([]float64, n)}
	threshold := math.Max(s.brightest(0.35), 0.08)
	grains := 0
	for i, l := range s.lum {
		if l >= threshold {
			f.state[i], f.lum[i], f.col[i] = stuck, math.Max(l, 0.45), s.col[i]
			grains++
		} else {
			f.back[i] = l * 0.4
		}
	}
	// Let the whole picture come loose over roughly eight minutes.
	f.release = float64(grains) / (8 * 60 / sandTick)
	return f
}

func (f *sand) step(s *scene, dt float64) {
	for f.owed += dt / sandTick; f.owed >= 1; f.owed-- {
		f.tick(s)
	}
}

func (f *sand) tick(s *scene) {
	// Loosen grains at random; picking empty cells simply does nothing, so
	// the last grains take longer, like the tail of an hourglass.
	tries := f.release * 3
	for ; tries > s.rng.Float64(); tries-- {
		if i := s.rng.Intn(len(f.state)); f.state[i] == stuck {
			f.state[i] = loose
		}
	}
	// Bottom row first, so a grain moves at most one row per tick.
	for r := s.rows - 2; r >= 0; r-- {
		for c := 0; c < s.cols; c++ {
			i := r*s.cols + c
			if f.state[i] != loose {
				continue
			}
			to := -1
			if below := i + s.cols; f.state[below] == empty {
				to = below
			} else {
				// Slide diagonally when both the side and the diagonal are free.
				side := 1 - 2*s.rng.Intn(2)
				for range 2 {
					if cc := c + side; cc >= 0 && cc < s.cols &&
						f.state[i+side] == empty && f.state[below+side] == empty {
						to = below + side
						break
					}
					side = -side
				}
			}
			if to >= 0 {
				f.state[to], f.lum[to], f.col[to] = loose, f.lum[i], f.col[i]
				f.state[i] = empty
			}
		}
	}
}

func (f *sand) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	if f.state[i] == empty {
		return f.back[i] * s.vig[i], s.col[i]
	}
	return f.lum[i] * s.vig[i], f.col[i]
}
