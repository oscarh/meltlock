package main

import "math"

// pulse: the picture breathes. A slow swell travels out from the centre,
// growing and shrinking the dots, while the picture gradually softens
// into blurred patches of colour.
type pulse struct {
	owed float64
	next []float64
}

const (
	pulsePeriod = 10.0 // seconds per breath
	pulseSoften = 0.5  // seconds between softening passes
)

func newPulse(s *scene) effect { return &pulse{next: make([]float64, s.cols*s.rows)} }

func (p *pulse) step(s *scene, dt float64) {
	for p.owed += dt / pulseSoften; p.owed >= 1; p.owed-- {
		for r := 0; r < s.rows; r++ {
			for c := 0; c < s.cols; c++ {
				i := r*s.cols + c
				sum := s.lum[r*s.cols+max(c-1, 0)] + s.lum[r*s.cols+min(c+1, s.cols-1)] +
					s.lum[max(r-1, 0)*s.cols+c] + s.lum[min(r+1, s.rows-1)*s.cols+c]
				p.next[i] = s.lum[i] + 0.03*(sum/4-s.lum[i])
			}
		}
		copy(s.lum, p.next)
	}
}

func (p *pulse) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	d := math.Hypot(float64(c)-float64(s.cols)/2, float64(r)-float64(s.rows)/2)
	// The main breath plus a slower, weaker one so it never repeats exactly.
	breath := 0.5 + 0.3*math.Sin(2*math.Pi*s.t/pulsePeriod-d*0.07) +
		0.2*math.Sin(2*math.Pi*s.t/(pulsePeriod*1.7)+float64(c)*0.04)
	return (s.lum[i]*(0.3+0.7*breath) + 0.05*breath) * s.vig[i], s.col[i]
}
