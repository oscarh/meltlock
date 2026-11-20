package main

import "math"

// canvas is the full-resolution picture that the painter effects draw on.
// It is BGRX and no channel is ever darker than the background, which is
// what lets fade work on eight bytes at a time. Embedding it makes an
// effect a painter.
//
// The desktop is only ever read through the scene's cells, so nothing in
// it becomes readable however fine the drawing is.
type canvas struct {
	w, h int
	pix  []byte
	bg   []byte // one row of background
}

// ink is a colour as blue, green, red in 0..255.
type ink [3]int

func inkOf(c rgb) ink { return ink(bgrOf(c)) }

func newCanvas(s *scene) *canvas {
	k := &canvas{w: s.cols * cell, h: s.rows * cell}
	k.pix, k.bg = make([]byte, k.w*k.h*4), make([]byte, k.w*4)
	for i, b := 0, bgrOf(crust); i < len(k.bg); i += 4 {
		k.bg[i], k.bg[i+1], k.bg[i+2] = uint8(b[0]), uint8(b[1]), uint8(b[2])
	}
	k.clear()
	return k
}

func (k *canvas) clear() {
	for y := 0; y < k.h; y++ {
		copy(k.pix[y*k.w*4:], k.bg)
	}
}

// picture draws the desktop as a soft blur, for an effect to open with.
func (k *canvas) picture(s *scene, strength float64) {
	glow := make([][3]float64, len(s.lum)) // how far each cell rises above the background
	for i := range glow {
		col := mix(surface0, s.col[i], 0.15+0.85*s.lum[i])
		for ch, v := range [3]float64{col.b - crust.b, col.g - crust.g, col.r - crust.r} {
			glow[i][ch] = math.Max(0, v) * 255 * strength * s.lum[i] * s.vig[i]
		}
	}
	for y := 0; y < k.h; y++ {
		for x := 0; x < k.w; x++ {
			i, j, fx, fy := s.corners(float64(x)+0.5, float64(y)+0.5)
			o := k.pix[(y*k.w+x)*4:]
			for ch := 0; ch < 3; ch++ {
				v := bilinear(glow[i[0]][ch], glow[i[1]][ch], glow[j[0]][ch], glow[j[1]][ch], fx, fy)
				o[ch] = k.bg[ch] + uint8(v)
			}
		}
	}
}

// put blends a colour into one pixel; a is the share of it, out of 256.
func (k *canvas) put(x, y, a int, c *ink) {
	if uint(x) >= uint(k.w) || uint(y) >= uint(k.h) || a <= 0 {
		return
	}
	o := k.pix[(y*k.w+x)*4:][:3]
	// Rounding down keeps each channel between its old value and the
	// colour, so never below the background.
	o[0] = uint8(int(o[0]) + (c[0]-int(o[0]))*a>>8)
	o[1] = uint8(int(o[1]) + (c[1]-int(o[1]))*a>>8)
	o[2] = uint8(int(o[2]) + (c[2]-int(o[2]))*a>>8)
}

// line draws a smooth line one pixel wide. The start point is left out so
// that joined lines do not draw their joints twice.
func (k *canvas) line(x0, y0, x1, y1, alpha float64, c *ink) {
	a := int(alpha * 256)
	dx, dy := x1-x0, y1-y0
	steep := math.Abs(dy) > math.Abs(dx)
	n := int(math.Ceil(math.Max(math.Abs(dx), math.Abs(dy))))
	for i := 1; i <= n; i++ {
		t := float64(i) / float64(n)
		along, across := x0+dx*t, y0+dy*t-0.5
		if steep {
			along, across = y0+dy*t, x0+dx*t-0.5
		}
		lo := math.Floor(across)
		f := int((across - lo) * 256)
		if steep {
			k.put(int(lo), int(along), a*(256-f)>>8, c)
			k.put(int(lo)+1, int(along), a*f>>8, c)
		} else {
			k.put(int(along), int(lo), a*(256-f)>>8, c)
			k.put(int(along), int(lo)+1, a*f>>8, c)
		}
	}
}

// splat blends a colour (blue, green, red in 0..255) into the canvas at a
// sub-pixel position.
func (k *canvas) splat(x, y, alpha float64, c *[3]float64) {
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
		// Truncation keeps each channel between its old value and the colour.
		o[0] = uint8(float64(o[0]) + (c[0]-float64(o[0]))*t)
		o[1] = uint8(float64(o[1]) + (c[1]-float64(o[1]))*t)
		o[2] = uint8(float64(o[2]) + (c[2]-float64(o[2]))*t)
	}
}

// fade moves the whole canvas towards the background by 1/2^shift of the
// distance, and at least one step, eight bytes at a time.
func (k *canvas) fade(shift uint) {
	const ones, low7 = 0x0101010101010101, 0x7f7f7f7f7f7f7f7f
	keep := uint64(0xff>>shift) * ones
	bg := le.Uint64(k.bg)
	for i := 0; i+8 <= len(k.pix); i += 8 {
		d := le.Uint64(k.pix[i:]) - bg // no byte borrows: none is below the background
		if d == 0 {
			continue
		}
		lit := ((d&low7 + low7) | d) >> 7 & ones // 1 in every byte that is not zero
		d -= d>>shift&keep + lit
		le.PutUint64(k.pix[i:], d+bg)
	}
}

// frame copies the canvas into a buffer of w by h pixels.
func (k *canvas) frame(s *scene, pix []byte, stride, w, h int) {
	for y := 0; y < min(h, k.h); y++ {
		copy(pix[y*stride:y*stride+min(w, k.w)*4], k.pix[y*k.w*4:])
	}
}

// cell is not used for drawing; it keeps painters usable as plain effects.
func (k *canvas) cell(s *scene, c, r int) (float64, rgb) {
	i := r*s.cols + c
	return s.lum[i] * 0.3 * s.vig[i], s.col[i]
}

// corners finds the four cells around a pixel position and the weights
// for blending between them.
func (s *scene) corners(x, y float64) (top, bottom [2]int, fx, fy float64) {
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

// at blends a per-cell field at a pixel position.
func (s *scene) at(field []float64, x, y float64) float64 {
	i, j, fx, fy := s.corners(x, y)
	return bilinear(field[i[0]], field[i[1]], field[j[0]], field[j[1]], fx, fy)
}

// cellAt is the cell under a pixel position, clamped to the grid.
func (s *scene) cellAt(x, y float64) int {
	return min(max(int(y)/cell, 0), s.rows-1)*s.cols + min(max(int(x)/cell, 0), s.cols-1)
}

// bright picks a random pixel position, preferring bright parts of the
// picture.
func (s *scene) bright() (x, y float64) {
	for try := 0; try < 4; try++ {
		x, y = s.rng.Float64()*float64(s.cols*cell), s.rng.Float64()*float64(s.rows*cell)
		if s.rng.Float64() < 0.15+0.85*s.lum[s.cellAt(x, y)] {
			break
		}
	}
	return x, y
}

// soft is the picture's brightness blurred over neighbouring cells.
func (s *scene) soft() []float64 { return s.blur(s.lum) }

// blur averages a per-cell field over each cell's neighbours.
func (s *scene) blur(field []float64) []float64 {
	out := make([]float64, len(field))
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			sum := 0.0
			for d := 0; d < 9; d++ {
				sum += field[min(max(r+d/3-1, 0), s.rows-1)*s.cols+min(max(c+d%3-1, 0), s.cols-1)]
			}
			out[r*s.cols+c] = sum / 9
		}
	}
	return out
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
