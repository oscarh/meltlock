package main

// drift: a wind sorts the picture. Brighter dots are blown past darker
// ones, in opposite directions in alternating bands, until each row is a
// smooth gradient and only the colours hint at what was there.
type drift struct{ owed float64 }

const (
	driftBand = 5    // rows per band
	driftRate = 9000 // comparisons per second
)

func newDrift(s *scene) effect { return &drift{} }

func (d *drift) step(s *scene, dt float64) {
	for d.owed += dt * driftRate; d.owed >= 1; d.owed-- {
		r, c := s.rng.Intn(s.rows), s.rng.Intn(s.cols-1)
		i := r*s.cols + c
		rightward := (r/driftBand)%2 == 0
		if (s.lum[i] > s.lum[i+1]) == rightward && s.lum[i] != s.lum[i+1] {
			s.lum[i], s.lum[i+1] = s.lum[i+1], s.lum[i]
			s.col[i], s.col[i+1] = s.col[i+1], s.col[i]
		}
	}
}

func (d *drift) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	return s.lum[i] * s.vig[i], s.col[i]
}
