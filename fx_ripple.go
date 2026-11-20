package main

import "math"

type ring struct{ x, y, born float64 }

// ripple: the picture lies under water. Drops fall on it and send out
// rings that bend the picture as they pass, and it slowly sinks out of
// sight.
type ripple struct {
	rings []ring
	next  float64
}

const (
	ringLife  = 7.0 // seconds
	ringSpeed = 7.0 // cells per second
	ringWidth = 5.0 // cells
)

func newRipple(s *scene) effect { return &ripple{} }

func (p *ripple) step(s *scene, dt float64) {
	live := p.rings[:0]
	for _, g := range p.rings {
		if s.t-g.born < ringLife {
			live = append(live, g)
		}
	}
	p.rings = live
	if s.t >= p.next {
		p.rings = append(p.rings, ring{s.rng.Float64() * float64(s.cols), s.rng.Float64() * float64(s.rows), s.t})
		p.next = s.t + 0.6 + 1.6*s.rng.Float64()
	}
}

func (p *ripple) cell(s *scene, c, r int) (float64, rgb) {
	var dx, dy, crest float64
	for _, g := range p.rings {
		ox, oy := float64(c)-g.x, float64(r)-g.y
		d := math.Hypot(ox, oy)
		age := s.t - g.born
		k := (d - age*ringSpeed) / ringWidth
		if k < -1 || k > 1 || d < 0.5 {
			continue
		}
		wave := math.Sin(k*math.Pi) * (1 - k*k) * (1 - age/ringLife)
		dx += wave * ox / d * 3
		dy += wave * oy / d * 3
		crest += math.Max(0, wave)
	}
	sc := min(max(c+int(math.Round(dx)), 0), s.cols-1)
	sr := min(max(r+int(math.Round(dy)), 0), s.rows-1)
	i, src := r*s.cols+c, sr*s.cols+sc
	// The picture fades to a faint trace over about ten minutes.
	sink := 0.25 + 0.75*math.Exp(-s.t/300)
	l := s.lum[src]*sink*s.vig[i] + 0.45*crest
	return l, mix(s.col[src], accents[6], math.Min(1, crest))
}
