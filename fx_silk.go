package main

import "math"

// silk: thousands of fine threads flow across the screen, drawn pixel by
// pixel instead of in cells. They follow the outlines of the desktop and a
// slowly turning wind, take their colour from what lies beneath them and
// fade behind them. The desktop itself is only ever used at cell
// resolution, so nothing in it becomes readable.
type silk struct {
	w, h   int       // canvas size in pixels
	pix    []byte    // BGRX canvas, no channel ever darker than the background
	tx, ty []float64 // per cell: direction along the outlines, scaled by their strength
	ps     []thread
	owed   float64
}

type thread struct {
	x, y        float64 // pixels
	c           [3]float64
	life, speed float64
}

const (
	silkFade  = 15.0  // fading passes per second
	silkWind  = 0.035 // how fast the wind turns
	silkScale = 320.0 // size of the wind's swirls in pixels
)

func newSilk(s *scene) effect {
	k := &silk{w: s.cols * cell, h: s.rows * cell}
	k.pix = make([]byte, k.w*k.h*4)
	n := s.cols * s.rows

	// Outlines: the gradient of the slightly blurred picture, turned a
	// quarter so that threads run along edges rather than across them.
	soft := make([]float64, n)
	at := func(c, r int) float64 {
		return s.lum[min(max(r, 0), s.rows-1)*s.cols+min(max(c, 0), s.cols-1)]
	}
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			sum := 0.0
			for d := 0; d < 9; d++ {
				sum += at(c+d%3-1, r+d/3-1)
			}
			soft[r*s.cols+c] = sum / 9
		}
	}
	k.tx, k.ty = make([]float64, n), make([]float64, n)
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			i := r*s.cols + c
			gx := soft[r*s.cols+min(c+1, s.cols-1)] - soft[r*s.cols+max(c-1, 0)]
			gy := soft[min(r+1, s.rows-1)*s.cols+c] - soft[max(r-1, 0)*s.cols+c]
			if m := math.Hypot(gx, gy); m > 1e-6 {
				strength := math.Min(1, m*5)
				k.tx[i], k.ty[i] = -gy/m*strength, gx/m*strength
			}
		}
	}

	// Start from the picture as a soft blur; it fades as the threads grow.
	bg := bgrOf(crust)
	glow := make([][3]float64, n) // how far each cell rises above the background
	for i := range glow {
		col := mix(surface0, s.col[i], 0.15+0.85*s.lum[i])
		for ch, v := range [3]float64{col.b - crust.b, col.g - crust.g, col.r - crust.r} {
			glow[i][ch] = math.Max(0, v) * 255 * 0.6 * s.lum[i] * s.vig[i]
		}
	}
	for y := 0; y < k.h; y++ {
		for x := 0; x < k.w; x++ {
			i, j, fx, fy := k.corners(s, float64(x)+0.5, float64(y)+0.5)
			o := k.pix[(y*k.w+x)*4:]
			for ch := 0; ch < 3; ch++ {
				v := bilinear(glow[i[0]][ch], glow[i[1]][ch], glow[j[0]][ch], glow[j[1]][ch], fx, fy)
				o[ch] = uint8(float64(bg[ch]) + v)
			}
		}
	}

	k.ps = make([]thread, n/2)
	for i := range k.ps {
		k.spawn(s, &k.ps[i])
		k.ps[i].life *= s.rng.Float64() // so they do not all end together
	}
	return k
}

// corners finds the four cells around a pixel position and the weights
// for blending between them.
func (k *silk) corners(s *scene, x, y float64) (top, bottom [2]int, fx, fy float64) {
	cx, cy := x/cell-0.5, y/cell-0.5
	c0, r0 := int(math.Floor(cx)), int(math.Floor(cy))
	fx, fy = cx-float64(c0), cy-float64(r0)
	ca, cb := min(max(c0, 0), s.cols-1), min(max(c0+1, 0), s.cols-1)
	ra, rb := min(max(r0, 0), s.rows-1), min(max(r0+1, 0), s.rows-1)
	return [2]int{ra*s.cols + ca, ra*s.cols + cb}, [2]int{rb*s.cols + ca, rb*s.cols + cb}, fx, fy
}

func bilinear(a, b, c, d, fx, fy float64) float64 {
	return (a*(1-fx)+b*fx)*(1-fy) + (c*(1-fx)+d*fx)*fy
}

func (k *silk) spawn(s *scene, p *thread) {
	for try := 0; try < 4; try++ { // prefer the bright parts
		p.x, p.y = s.rng.Float64()*float64(k.w), s.rng.Float64()*float64(k.h)
		if s.rng.Float64() < 0.15+0.85*s.lum[k.cellAt(s, p.x, p.y)] {
			break
		}
	}
	col := s.col[k.cellAt(s, p.x, p.y)]
	p.c = [3]float64{col.b * 255, col.g * 255, col.r * 255}
	p.life, p.speed = 3+7*s.rng.Float64(), 30+50*s.rng.Float64()
}

func (k *silk) cellAt(s *scene, x, y float64) int {
	return min(max(int(y)/cell, 0), s.rows-1)*s.cols + min(max(int(x)/cell, 0), s.cols-1)
}

func (k *silk) step(s *scene, dt float64) {
	for k.owed += dt * silkFade; k.owed >= 1; k.owed-- {
		k.fade()
	}
	wind := s.t * silkWind
	for n := range k.ps {
		p := &k.ps[n]
		if p.life -= dt; p.life <= 0 || p.x < 0 || p.y < 0 || p.x >= float64(k.w) || p.y >= float64(k.h) {
			k.spawn(s, p)
			continue
		}
		i, j, fx, fy := k.corners(s, p.x, p.y)
		tx := bilinear(k.tx[i[0]], k.tx[i[1]], k.tx[j[0]], k.tx[j[1]], fx, fy)
		ty := bilinear(k.ty[i[0]], k.ty[i[1]], k.ty[j[0]], k.ty[j[1]], fx, fy)
		lum := bilinear(s.lum[i[0]], s.lum[i[1]], s.lum[j[0]], s.lum[j[1]], fx, fy)

		// The wind alone in open areas; outlines pull the thread along.
		a := 4 * math.Pi * noise3(p.x/silkScale, p.y/silkScale, wind)
		vx, vy := math.Cos(a)+3*tx, math.Sin(a)+3*ty
		norm := math.Hypot(vx, vy) + 1e-9
		dist := p.speed * dt
		vx, vy = vx/norm, vy/norm

		// Take on the colour of what is underneath, gradually.
		here := k.cellAt(s, p.x, p.y)
		col := mix(s.col[here], text, 0.12)
		for ch, v := range [3]float64{col.b * 255, col.g * 255, col.r * 255} {
			p.c[ch] += (v - p.c[ch]) * math.Min(1, 3*dt)
		}
		alpha := (0.14 + 0.8*lum) * s.vig[here] * math.Min(1, p.life) // fade out at the end
		for left := dist; left > 0; left-- {
			d := math.Min(left, 1)
			p.x, p.y = p.x+vx*d, p.y+vy*d
			k.splat(p.x, p.y, alpha*d, &p.c)
		}
	}
}

// splat blends a colour into the canvas at a sub-pixel position.
func (k *silk) splat(x, y, alpha float64, c *[3]float64) {
	x, y = x-0.5, y-0.5
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	fx, fy := x-float64(x0), y-float64(y0)
	for n, wgt := range [4]float64{(1 - fx) * (1 - fy), fx * (1 - fy), (1 - fx) * fy, fx * fy} {
		px, py := x0+n&1, y0+n>>1
		if px < 0 || py < 0 || px >= k.w || py >= k.h {
			continue
		}
		o := k.pix[(py*k.w+px)*4:]
		t := alpha * wgt
		// Truncation keeps each channel between its old value and the
		// colour, so never below the background.
		o[0] = uint8(float64(o[0]) + (c[0]-float64(o[0]))*t)
		o[1] = uint8(float64(o[1]) + (c[1]-float64(o[1]))*t)
		o[2] = uint8(float64(o[2]) + (c[2]-float64(o[2]))*t)
	}
}

// fade moves the whole canvas a sixteenth of the way to the background,
// and at least one step, eight bytes at a time.
func (k *silk) fade() {
	const ones, low7, low4 = 0x0101010101010101, 0x7f7f7f7f7f7f7f7f, 0x0f0f0f0f0f0f0f0f
	b := bgrOf(crust)
	px := uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16
	bg := px | px<<32
	for i := 0; i+8 <= len(k.pix); i += 8 {
		d := le.Uint64(k.pix[i:]) - bg // no byte borrows: none is below the background
		if d == 0 {
			continue
		}
		lit := ((d&low7 + low7) | d) >> 7 & ones // 1 in every byte that is not zero
		d -= d>>4&low4 + lit
		le.PutUint64(k.pix[i:], d+bg)
	}
}

// frame copies the canvas into a buffer of w by h pixels.
func (k *silk) frame(s *scene, pix []byte, stride, w, h int) {
	for y := 0; y < min(h, k.h); y++ {
		copy(pix[y*stride:y*stride+min(w, k.w)*4], k.pix[y*k.w*4:])
	}
}

// cell is not used for drawing; it keeps silk usable as a plain effect.
func (k *silk) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	return s.lum[i] * 0.3 * s.vig[i], s.col[i]
}

// noise3 is smooth value noise in 0..1.
func noise3(x, y, z float64) float64 {
	x0, y0, z0 := math.Floor(x), math.Floor(y), math.Floor(z)
	fx, fy, fz := smooth(x-x0), smooth(y-y0), smooth(z-z0)
	ix, iy, iz := int(x0), int(y0), int(z0)
	plane := func(z int) float64 {
		return bilinear(lattice(ix, iy, z), lattice(ix+1, iy, z), lattice(ix, iy+1, z), lattice(ix+1, iy+1, z), fx, fy)
	}
	return plane(iz)*(1-fz) + plane(iz+1)*fz
}

func smooth(t float64) float64 { return t * t * (3 - 2*t) }

func lattice(x, y, z int) float64 {
	h := uint32(x)*0x8da6b343 ^ uint32(y)*0xd8163841 ^ uint32(z)*0xcb1ab31f
	h ^= h >> 13
	h *= 0x5bd1e995
	h ^= h >> 15
	return float64(h&0xffff) / 0xffff
}
