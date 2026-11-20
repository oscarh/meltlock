package main

import (
	"encoding/gob"
	"math"
	"math/rand"
	"os"
	"time"
)

// levels is the number of dot sizes.
const levels = 32

// snapshot is the desktop as handed from the locker to the background
// program: one brightness and palette colour per cell, nothing readable.
type snapshot struct {
	Cols, Rows int
	Lum        []float64
	Col        [][3]float64 // red, green, blue
}

func (g *grid) snapshot() snapshot {
	s := snapshot{g.cols, g.rows, g.lum, make([][3]float64, len(g.col))}
	for i, c := range g.col {
		s.Col[i] = [3]float64{c.r, c.g, c.b}
	}
	return s
}

func (s snapshot) save(path string) error {
	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	if err := gob.NewEncoder(f).Encode(s); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func loadSnapshot(path string) (snapshot, error) {
	var s snapshot
	f, err := os.Open(path)
	if err != nil {
		return s, err
	}
	defer f.Close()
	err = gob.NewDecoder(f).Decode(&s)
	return s, err
}

type drip struct {
	c    int
	y, v float64
	lum  float64
	col  rgb
}

// scene is the animated dot wall: the desktop picture slowly melts as
// drips run down it and collect in a pool at the bottom.
type scene struct {
	grid
	glow     []float64 // brightness left behind by drips
	glowCol  []rgb
	vig      []float64 // darkening towards the corners
	pool     []float64 // depth of the pool per column, in rows
	poolNext []float64
	poolCol  []rgb
	drips    []drip
	rng      *rand.Rand
	t        float64
	spawn    float64 // drips owed

	glitchTop, glitchRows, glitchBy int
	glitchEnd, glitchNext           float64

	stamped string
}

// newScene builds a scene of the given size. A snapshot of another size
// (or none) is replaced by a faint random field.
func newScene(cols, rows int, snap snapshot, seed int64) *scene {
	n := cols * rows
	s := &scene{rng: rand.New(rand.NewSource(seed))}
	s.cols, s.rows = cols, rows
	s.mark = make([]uint8, n)
	if snap.Cols == cols && snap.Rows == rows && len(snap.Lum) == n && len(snap.Col) == n {
		s.lum, s.col = snap.Lum, make([]rgb, n)
		for i, c := range snap.Col {
			s.col[i] = rgb{c[0], c[1], c[2]}
		}
	} else {
		s.lum, s.col = make([]float64, n), make([]rgb, n)
		for i := range s.lum {
			s.lum[i], s.col[i] = 0.25*s.rng.Float64(), accents[s.rng.Intn(len(accents))]
		}
	}
	s.glow, s.glowCol, s.vig = make([]float64, n), make([]rgb, n), make([]float64, n)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			dx, dy := float64(c)/float64(cols)-0.5, float64(r)/float64(rows)-0.5
			s.vig[r*cols+c] = 1 - 1.3*(dx*dx+dy*dy)
		}
	}
	s.pool, s.poolCol = make([]float64, cols), make([]rgb, cols)
	for c := range s.poolCol {
		s.poolCol[c] = lavender
	}
	s.glitchNext = 2 + 4*s.rng.Float64()
	return s
}

// step advances the animation by dt seconds.
func (s *scene) step(dt float64, now time.Time) {
	s.t += dt

	fade := math.Exp(-dt / 1.6)
	for i := range s.glow {
		s.glow[i] *= fade
	}

	// About one new drip per twenty columns each second.
	s.spawn += dt * float64(s.cols) / 20
	for ; s.spawn >= 1; s.spawn-- {
		s.addDrip()
	}

	live := s.drips[:0]
	for _, d := range s.drips {
		from := int(d.y)
		d.v += 6 * dt
		d.y += d.v * dt
		floor := s.rows - 1 - int(s.pool[d.c])
		for r := from + 1; r <= int(d.y) && r <= floor; r++ {
			i := r*s.cols + d.c
			s.glow[i], s.glowCol[i] = d.lum, d.col
			s.lum[i] *= 0.985 // the picture wears away where drips run
		}
		if int(d.y) >= floor {
			s.land(d)
			continue
		}
		live = append(live, d)
	}
	s.drips = live

	s.flow(dt)

	if s.t >= s.glitchNext {
		s.glitchTop, s.glitchRows = s.rng.Intn(s.rows), 1+s.rng.Intn(3)
		s.glitchBy = 3 + s.rng.Intn(13)
		if s.rng.Intn(2) == 0 {
			s.glitchBy = -s.glitchBy
		}
		s.glitchEnd = s.t + 0.12
		s.glitchNext = s.t + 3 + 6*s.rng.Float64()
	}

	// Clock with a colon that blinks once a second.
	clock := now.Format("15:04")
	if now.Second()%2 == 1 {
		clock = now.Format("15 04")
	}
	if label := now.Format("Mon 2 Jan") + "|" + clock; label != s.stamped {
		s.stamped = label
		for i := range s.mark {
			s.mark[i] = plain
		}
		s.stamp(upper(now.Format("Mon 2 Jan")), clock)
	}
}

// land adds a drip to the pool as a small heap around its column.
func (s *scene) land(d drip) {
	limit := float64(s.rows) * 0.3
	for off, amount := range [...]float64{0.3, 1.0, 2.0, 1.0, 0.3} {
		c := d.c + off - 2
		if c < 0 || c >= s.cols {
			continue
		}
		s.pool[c] = math.Min(limit, s.pool[c]+amount)
		s.poolCol[c] = mix(s.poolCol[c], d.col, math.Min(1, 0.25*amount))
	}
}

// flow moves the pool like a thick liquid: heaps slump slowly sideways,
// run off at the screen edges, and the whole pool seeps away over time.
func (s *scene) flow(dt float64) {
	const spread, seep = 0.22, 0.035
	k := math.Min(0.4, spread*dt)
	if len(s.poolNext) != s.cols {
		s.poolNext = make([]float64, s.cols)
	}
	for c := range s.pool {
		left, right := 0.0, 0.0 // beyond the edges there is nothing
		colL, colR := s.poolCol[c], s.poolCol[c]
		if c > 0 {
			left, colL = s.pool[c-1], s.poolCol[c-1]
		}
		if c < s.cols-1 {
			right, colR = s.pool[c+1], s.poolCol[c+1]
		}
		s.poolNext[c] = (s.pool[c] + k*(left+right-2*s.pool[c])) * (1 - seep*dt)
		// Colour is carried along with whatever flows in.
		if in := k * (math.Max(0, left-s.pool[c]) + math.Max(0, right-s.pool[c])); in > 0 {
			from := colL
			if right > left {
				from = colR
			}
			s.poolCol[c] = mix(s.poolCol[c], from, math.Min(1, in/(s.pool[c]+0.5)))
		}
	}
	s.pool, s.poolNext = s.poolNext, s.pool
}

// addDrip starts a drip at a bright edge of the picture, or as rain from
// the top when the column has none.
func (s *scene) addDrip() {
	c := s.rng.Intn(s.cols)
	floor := s.rows - 1 - int(s.pool[c])
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
	s.drips = append(s.drips, d)
}

// cells returns what every cell should look like now: the dot size level
// in the top byte and its colour as 0xRRGGBB below.
func (s *scene) cells(out []uint32) {
	glitch := s.t < s.glitchEnd
	for r := 0; r < s.rows; r++ {
		shift := 0
		if glitch && r >= s.glitchTop && r < s.glitchTop+s.glitchRows {
			shift = s.glitchBy
		}
		for c := 0; c < s.cols; c++ {
			i := r*s.cols + c
			src := i
			if shift != 0 {
				src = r*s.cols + ((c+shift)%s.cols+s.cols)%s.cols
			}
			l, col := s.lum[src]*s.vig[i], s.col[src]
			if depth := s.pool[c] - float64(s.rows-1-r); depth > 0 {
				// Inside the pool: a slow swell below a bright surface whose
				// top dot grows as the cell fills.
				wave := 0.5 + 0.5*math.Sin(s.t*0.8+float64(c)*0.3-float64(r)*0.7)
				l = 0.28 + 0.14*wave
				if depth < 1.6 {
					l = 0.8 * math.Min(1, depth)
				}
				col = s.poolCol[c]
			}
			if g := s.glow[i]; g > l {
				l, col = g, s.glowCol[i]
			}
			switch s.mark[i] {
			case inked:
				l, col = 1, text
			case quiet:
				l *= 0.2
			}
			level := int(math.Pow(l, 0.7)*(levels-1) + 0.5)
			p := mix(surface0, col, 0.15+0.85*float64(level)/(levels-1))
			out[i] = uint32(level)<<24 | uint32(p.r*255+0.5)<<16 | uint32(p.g*255+0.5)<<8 | uint32(p.b*255+0.5)
		}
	}
}

func upper(s string) string {
	b := []byte(s)
	for i, ch := range b {
		if ch >= 'a' && ch <= 'z' {
			b[i] = ch - 32
		}
	}
	return string(b)
}
