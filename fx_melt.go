package main

import "math"

type drip struct {
	c    int
	y, v float64
	lum  float64
	col  rgb
}

// melt: drips run down the picture and wear it away, and collect at the
// bottom as a thick liquid. Now and then a band of rows jumps sideways.
type melt struct {
	glow     []float64 // brightness left behind by drips
	glowCol  []rgb
	pool     []float64 // depth of the pool per column, in rows
	poolNext []float64
	poolCol  []rgb
	drips    []drip
	spawn    float64 // drips owed

	glitchTop, glitchRows, glitchBy int
	glitchEnd, glitchNext           float64
}

func newMelt(s *scene) effect {
	n := s.cols * s.rows
	m := &melt{
		glow: make([]float64, n), glowCol: make([]rgb, n),
		pool: make([]float64, s.cols), poolNext: make([]float64, s.cols), poolCol: make([]rgb, s.cols),
	}
	for c := range m.poolCol {
		m.poolCol[c] = lavender
	}
	m.glitchNext = 2 + 4*s.rng.Float64()
	return m
}

func (m *melt) step(s *scene, dt float64) {
	fade := math.Exp(-dt / 1.6)
	for i := range m.glow {
		m.glow[i] *= fade
	}

	// About one new drip per twenty columns each second.
	m.spawn += dt * float64(s.cols) / 20
	for ; m.spawn >= 1; m.spawn-- {
		m.addDrip(s)
	}

	live := m.drips[:0]
	for _, d := range m.drips {
		from := int(d.y)
		d.v += 6 * dt
		d.y += d.v * dt
		floor := s.rows - 1 - int(m.pool[d.c])
		for r := from + 1; r <= int(d.y) && r <= floor; r++ {
			i := r*s.cols + d.c
			m.glow[i], m.glowCol[i] = d.lum, d.col
			s.lum[i] *= 0.985 // the picture wears away where drips run
		}
		if int(d.y) >= floor {
			m.land(s, d)
			continue
		}
		live = append(live, d)
	}
	m.drips = live

	m.flow(s, dt)

	if s.t >= m.glitchNext {
		m.glitchTop, m.glitchRows = s.rng.Intn(s.rows), 1+s.rng.Intn(3)
		m.glitchBy = 3 + s.rng.Intn(13)
		if s.rng.Intn(2) == 0 {
			m.glitchBy = -m.glitchBy
		}
		m.glitchEnd = s.t + 0.12
		m.glitchNext = s.t + 3 + 6*s.rng.Float64()
	}
}

// land adds a drip to the pool as a small heap around its column.
func (m *melt) land(s *scene, d drip) {
	limit := float64(s.rows) * 0.3
	for off, amount := range [...]float64{0.3, 1.0, 2.0, 1.0, 0.3} {
		c := d.c + off - 2
		if c < 0 || c >= s.cols {
			continue
		}
		m.pool[c] = math.Min(limit, m.pool[c]+amount)
		m.poolCol[c] = mix(m.poolCol[c], d.col, math.Min(1, 0.25*amount))
	}
}

// flow moves the pool like a thick liquid: heaps slump sideways, run off
// at the screen edges, and the whole pool seeps away over time.
func (m *melt) flow(s *scene, dt float64) {
	const spread, seep = 0.32, 0.04
	k := math.Min(0.4, spread*dt)
	for c := range m.pool {
		left, right := 0.0, 0.0 // beyond the edges there is nothing
		colL, colR := m.poolCol[c], m.poolCol[c]
		if c > 0 {
			left, colL = m.pool[c-1], m.poolCol[c-1]
		}
		if c < s.cols-1 {
			right, colR = m.pool[c+1], m.poolCol[c+1]
		}
		m.poolNext[c] = (m.pool[c] + k*(left+right-2*m.pool[c])) * (1 - seep*dt)
		// Colour is carried along with whatever flows in.
		if in := k * (math.Max(0, left-m.pool[c]) + math.Max(0, right-m.pool[c])); in > 0 {
			from := colL
			if right > left {
				from = colR
			}
			m.poolCol[c] = mix(m.poolCol[c], from, math.Min(1, in/(m.pool[c]+0.5)))
		}
	}
	m.pool, m.poolNext = m.poolNext, m.pool
}

// addDrip starts a drip at a bright edge of the picture, or as rain from
// the top when the column has none.
func (m *melt) addDrip(s *scene) {
	c := s.rng.Intn(s.cols)
	floor := s.rows - 1 - int(m.pool[c])
	var edges []int
	for r := 0; r < floor-1; r++ {
		if i := r*s.cols + c; s.lum[i]-s.lum[i+s.cols] > 0.15 {
			edges = append(edges, r)
		}
	}
	d := drip{c: c, v: 2 + 5*s.rng.Float64()}
	if len(edges) > 0 && s.rng.Float64() < 0.75 {
		r := edges[s.rng.Intn(len(edges))]
		i := r*s.cols + c
		d.y, d.lum, d.col = float64(r), math.Max(s.lum[i], 0.55), s.col[i]
	} else {
		d.y, d.lum = float64(s.rng.Intn(s.rows/3+1)), 0.35+0.3*s.rng.Float64()
		d.col = accents[s.rng.Intn(len(accents))]
	}
	m.drips = append(m.drips, d)
}

func (m *melt) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	src := i
	if s.t < m.glitchEnd && r >= m.glitchTop && r < m.glitchTop+m.glitchRows {
		src = r*s.cols + ((c+m.glitchBy)%s.cols+s.cols)%s.cols
	}
	l, col := s.lum[src]*s.vig[i], s.col[src]
	if depth := m.pool[c] - float64(s.rows-1-r); depth > 0 {
		// Inside the pool: a slow swell below a bright surface whose top
		// dot grows as the cell fills.
		wave := 0.5 + 0.5*math.Sin(s.t*0.8+float64(c)*0.3-float64(r)*0.7)
		l = 0.28 + 0.14*wave
		if depth < 1.6 {
			l = 0.8 * math.Min(1, depth)
		}
		col = m.poolCol[c]
	}
	if g := m.glow[i]; g > l {
		l, col = g, m.glowCol[i]
	}
	return l, col
}
