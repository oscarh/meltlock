package main

import "math"

// lava: a lava lamp. Blobs of wax start where the desktop was bright, in
// its colours, then warm up, rise, cool and sink, merging and parting as
// they pass. Wax pools along the bottom, and a dim blur of the desktop
// stays behind it all.
type lava struct {
	*canvas
	base  []byte // the dim desktop that the wax is drawn over
	blobs []blob
	floor [][3]float64 // colour of the pool at each lattice column
	// Wax on the lattice of cell corners: how much, and its colour.
	wax        []float64
	tint       [][3]float64
	amount     []int    // wax as a position in glow, 16.16 fixed point
	colour     [][3]int // 16.16 fixed point
	lit        []bool   // per cell: drawn over the base right now
	glow       [1024]int
	owed, heat float64
}

type blob struct {
	x, y, vx, vy float64
	r            float64 // reach in pixels; it shows out to about 0.6 of that
	temp         float64 // 0 cold and sinking, 1 hot and rising
	heating      bool
	low, high    float64 // where it turns: starts heating below low, cooling above high (share of the height)
	phase        float64
	c            [3]float64
}

const (
	lavaFrames = 15.0  // redraws per second; wax is slow
	lavaEdge   = 0.4   // how much wax makes a surface
	lavaHalo   = 0.15  // where the glow around it starts
	lavaScale  = 512.0 // glow entries per unit of wax
	lavaLift   = 40.0  // how hard heat pushes, pixels per second squared
)

func newLava(s *scene) effect {
	k := &lava{canvas: newCanvas(s)}
	k.picture(s, 0.3)
	k.base = append([]byte(nil), k.pix...)
	n := (s.cols + 1) * (s.rows + 1)
	k.wax, k.tint = make([]float64, n), make([][3]float64, n)
	k.amount, k.colour = make([]int, n), make([][3]int, n)
	k.lit = make([]bool, s.cols*s.rows)

	// Brightness for an amount of wax: a faint halo, a soft edge, then a
	// body that is brighter towards its core.
	for i := range k.glow {
		f := float64(i) / lavaScale
		halo := 0.16 * math.Pow(math.Max(0, (f-lavaHalo)/(lavaEdge-lavaHalo)), 2)
		edge := smooth(math.Max(0, math.Min(1, (f-lavaEdge+0.03)/0.06)))
		body := 0.6 + 0.4*math.Min(1, math.Max(0, f-lavaEdge)/0.9)
		k.glow[i] = int(256 * (halo*(1-edge) + edge*body))
	}

	// The pool takes the colours along the bottom of the desktop, smoothed.
	k.floor = make([][3]float64, s.cols+1)
	for col := range k.floor {
		const reach = 8
		for d := -reach; d <= reach; d++ {
			c := s.col[(s.rows-1)*s.cols+min(max(col+d, 0), s.cols-1)]
			for ch, v := range [3]float64{c.b, c.g, c.r} {
				k.floor[col][ch] += v * 255 / (2*reach + 1)
			}
		}
	}

	k.blobs = make([]blob, max(6, s.cols*s.rows/450))
	for i := range k.blobs {
		b := &k.blobs[i]
		b.x, b.y = s.bright()
		at := s.cellAt(b.x, b.y)
		col := mix(s.col[at], text, 0.1)
		b.c = [3]float64{col.b * 255, col.g * 255, col.r * 255}
		b.r = 70 + 130*s.rng.Float64()*(0.4+0.6*s.lum[at])
		b.heating = s.rng.Float64() < b.y/float64(k.h)
		b.temp = 0.5
		b.low, b.high = 0.7+0.22*s.rng.Float64(), 0.08+0.22*s.rng.Float64()
		b.phase = s.rng.Float64() * 2 * math.Pi
	}
	return k
}

func (k *lava) step(s *scene, dt float64) {
	w, h := float64(k.w), float64(k.h)
	k.heat = math.Min(1, k.heat+dt/10) // the lamp takes a while to get going
	for i := range k.blobs {
		b := &k.blobs[i]
		// Heated at the bottom until it has risen to the top, then cooled
		// until it has sunk again.
		if b.y > b.low*h {
			b.heating = true
		} else if b.y < b.high*h {
			b.heating = false
		}
		target := 0.0
		if b.heating {
			target = 1
		}
		b.temp += (target - b.temp) * 0.2 * dt
		size := 1.5 - b.r/200 // small blobs are quicker
		b.vy += (-(b.temp-0.5)*lavaLift*size - b.vy) * dt
		sway := noise3(b.x/400, b.y/400, s.t*0.05+b.phase) - 0.5
		b.vx += (sway*24 - b.vx) * dt
		b.x += b.vx * dt * k.heat
		b.y += b.vy * dt * k.heat
		// Keep them in the lamp.
		b.x = math.Max(0, math.Min(w, b.x))
		if b.y < 0 || b.y > h {
			b.y, b.vy = math.Max(0, math.Min(h, b.y)), 0
		}
	}
	if k.owed += dt * lavaFrames; k.owed >= 1 {
		k.owed = 0
		k.spread(s)
		k.draw(s)
	}
}

// spread works out how much wax there is at every lattice point.
func (k *lava) spread(s *scene) {
	clear(k.wax)
	clear(k.tint)
	cols, rows := s.cols+1, s.rows+1
	add := func(i int, wgt float64, c *[3]float64) {
		k.wax[i] += wgt
		k.tint[i][0] += wgt * c[0]
		k.tint[i][1] += wgt * c[1]
		k.tint[i][2] += wgt * c[2]
	}
	for i := range k.blobs {
		b := &k.blobs[i]
		r := b.r * (1 + 0.06*math.Sin(s.t*0.7+b.phase)) // breathing a little
		for row := max(0, int((b.y-r)/cell)); row <= min(rows-1, int((b.y+r)/cell)+1); row++ {
			for col := max(0, int((b.x-r)/cell)); col <= min(cols-1, int((b.x+r)/cell)+1); col++ {
				dx, dy := float64(col*cell)-b.x, float64(row*cell)-b.y
				if u := 1 - (dx*dx+dy*dy)/(r*r); u > 0 {
					add(row*cols+col, u*u, &b.c)
				}
			}
		}
	}
	// The pool along the bottom, in the colours of the desktop there,
	// with a slow swell.
	for col := 0; col < cols; col++ {
		reach := 30 + 190*noise3(float64(col*cell)/200, s.t*0.05, 7)
		for n := 0; n < rows; n++ {
			u := 1 - float64(n*cell)/reach
			if u <= 0 {
				break
			}
			add((rows-1-n)*cols+col, 1.1*u*u, &k.floor[col])
		}
	}
	for i, wax := range k.wax {
		k.amount[i] = int(math.Min(wax*lavaScale, float64(len(k.glow)-1)) * 65536)
		if wax > 1e-6 {
			for ch := range k.colour[i] {
				k.colour[i][ch] = int(k.tint[i][ch] / wax * 65536)
			}
		}
	}
}

// draw paints the wax over the base, one cell at a time, blending the
// lattice values smoothly across each cell. Cells without wax are left
// alone, or put back if they had some.
func (k *lava) draw(s *scene) {
	cols := s.cols + 1
	visible := int(math.Round(lavaHalo * lavaScale * 65536))
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			i := r*cols + c
			corner := [4]int{i, i + 1, i + cols, i + cols + 1}
			if max(k.amount[corner[0]], k.amount[corner[1]], k.amount[corner[2]], k.amount[corner[3]]) <= visible {
				if k.lit[r*s.cols+c] {
					k.lit[r*s.cols+c] = false
					for y := r * cell; y < (r+1)*cell; y++ {
						o := (y*k.w + c*cell) * 4
						copy(k.pix[o:o+cell*4], k.base[o:])
					}
				}
				continue
			}
			k.lit[r*s.cols+c] = true
			// Four values per corner: the amount and the three colours.
			var v [4][4]int
			for n, at := range corner {
				v[n] = [4]int{k.amount[at], k.colour[at][0], k.colour[at][1], k.colour[at][2]}
			}
			for y := 0; y < cell; y++ {
				var at, inc [4]int
				for ch := range at {
					left := v[0][ch] + (v[2][ch]-v[0][ch])*(2*y+1)/(2*cell)
					right := v[1][ch] + (v[3][ch]-v[1][ch])*(2*y+1)/(2*cell)
					inc[ch] = (right - left) / cell
					at[ch] = left + inc[ch]/2
				}
				o := ((r*cell+y)*k.w + c*cell) * 4
				pix, base := k.pix[o:o+cell*4], k.base[o:o+cell*4]
				for x := 0; x < cell*4; x += 4 {
					g := k.glow[at[0]>>16]
					b0, b1, b2 := int(base[x]), int(base[x+1]), int(base[x+2])
					pix[x] = uint8(b0 + (at[1]>>16-b0)*g>>8)
					pix[x+1] = uint8(b1 + (at[2]>>16-b1)*g>>8)
					pix[x+2] = uint8(b2 + (at[3]>>16-b2)*g>>8)
					at[0], at[1], at[2], at[3] = at[0]+inc[0], at[1]+inc[1], at[2]+inc[2], at[3]+inc[3]
				}
			}
		}
	}
}
