package main

import "math"

// life: the bright parts of the picture come alive as Conway's Game of
// Life and grow over the faint remains of the desktop. Cells inherit a
// parent's colour and leave a fading trail when they die.
type life struct {
	alive, next []bool
	col, colNew []rgb
	trail       []float64
	owed        float64
	wake        int // ticks until the next patch wakes up
}

const lifeTick = 1.0 / 7

func newLife(s *scene) effect {
	n := s.cols * s.rows
	f := &life{
		alive: make([]bool, n), next: make([]bool, n),
		col: make([]rgb, n), colNew: make([]rgb, n), trail: make([]float64, n),
	}
	threshold := math.Max(s.brightest(0.3), 0.08)
	for i, l := range s.lum {
		f.alive[i], f.col[i] = l >= threshold, s.col[i]
	}
	f.wake = int(20 / lifeTick)
	return f
}

func (f *life) step(s *scene, dt float64) {
	fade := math.Exp(-dt / 2.5)
	for i := range f.trail {
		f.trail[i] *= fade
	}
	for f.owed += dt / lifeTick; f.owed >= 1; f.owed-- {
		f.tick(s)
	}
}

func (f *life) tick(s *scene) {
	count := 0
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			i := r*s.cols + c
			n, parent := 0, -1
			for dr := -1; dr <= 1; dr++ {
				for dc := -1; dc <= 1; dc++ {
					if dr == 0 && dc == 0 {
						continue
					}
					j := (r+dr+s.rows)%s.rows*s.cols + (c+dc+s.cols)%s.cols
					if f.alive[j] {
						n++
						parent = j
					}
				}
			}
			f.next[i] = n == 3 || (n == 2 && f.alive[i])
			f.colNew[i] = f.col[i]
			switch {
			case f.next[i] && !f.alive[i]:
				f.colNew[i] = f.col[parent]
			case !f.next[i] && f.alive[i]:
				f.trail[i] = 0.5
			}
			if f.next[i] {
				count++
			}
		}
	}
	f.alive, f.next = f.next, f.alive
	f.col, f.colNew = f.colNew, f.col

	// Life left alone settles into still patterns, so every 20 seconds or
	// so, or when it has nearly died out, a patch of the picture wakes up.
	if f.wake--; count < len(f.alive)/60 || f.wake <= 0 {
		f.wake = int((14 + 12*s.rng.Float64()) / lifeTick)
		top, left := s.rng.Intn(s.rows), s.rng.Intn(s.cols)
		for k := 0; k < 500; k++ {
			r, c := (top+s.rng.Intn(24))%s.rows, (left+s.rng.Intn(36))%s.cols
			i := r*s.cols + c
			f.alive[i], f.col[i] = true, s.col[i]
		}
	}
}

func (f *life) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	if f.alive[i] {
		return 0.85 * s.vig[i], f.col[i]
	}
	return math.Max(s.lum[i]*0.3*s.vig[i], f.trail[i]), f.col[i]
}
