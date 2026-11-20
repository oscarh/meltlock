package main

import (
	"bufio"
	"errors"
	"fmt"
	"image"
	"io"
	"math"
	"math/rand"
	"sort"
)

// cell is the size of one dot in pixels. It is large enough that no text
// from the desktop survives.
const cell = 12

type rgb struct{ r, g, b float64 }

func hex(v uint32) rgb {
	return rgb{float64(v>>16&0xff) / 255, float64(v>>8&0xff) / 255, float64(v&0xff) / 255}
}

func mix(a, b rgb, t float64) rgb {
	return rgb{a.r + (b.r-a.r)*t, a.g + (b.g-a.g)*t, a.b + (b.b-a.b)*t}
}

// Catppuccin Mocha.
var (
	crust    = hex(0x11111b)
	surface0 = hex(0x313244)
	text     = hex(0xcdd6f4)
	lavender = hex(0xb4befe)
	// accents by hue, 30 degrees each, starting at red
	accents = [12]rgb{
		hex(0xf38ba8), hex(0xfab387), hex(0xf9e2af), hex(0xa6e3a1),
		hex(0xa6e3a1), hex(0x94e2d5), hex(0x74c7ec), hex(0x89b4fa),
		hex(0xb4befe), hex(0xcba6f7), hex(0xf5c2e7), hex(0xf38ba8),
	}
)

// Cell marks.
const (
	plain = iota
	inked // part of the clock
	quiet // dimmed surroundings of the clock
)

// grid is the desktop reduced to one brightness and colour per cell.
type grid struct {
	cols, rows int
	lum        []float64
	col        []rgb
	mark       []uint8
}

// readPPM reads a binary PPM (P6, 8 bit) as written by `grim -t ppm`.
func readPPM(r io.Reader) (pix []byte, w, h int, err error) {
	br := bufio.NewReader(r)
	var magic string
	var max int
	if _, err = fmt.Fscan(br, &magic, &w, &h, &max); err != nil {
		return nil, 0, 0, err
	}
	if magic != "P6" || max != 255 || w <= 0 || h <= 0 || w > 16384 || h > 16384 {
		return nil, 0, 0, errors.New("unsupported image")
	}
	if _, err = br.ReadByte(); err != nil { // single whitespace after the header
		return nil, 0, 0, err
	}
	pix = make([]byte, w*h*3)
	_, err = io.ReadFull(br, pix)
	return pix, w, h, err
}

// sample averages the image into cells and maps each to the palette.
func sample(pix []byte, w, h int) *grid {
	g := &grid{cols: (w + cell - 1) / cell, rows: (h + cell - 1) / cell}
	n := g.cols * g.rows
	g.lum, g.col, g.mark = make([]float64, n), make([]rgb, n), make([]uint8, n)
	sum := make([][4]float64, n)
	for y := 0; y < h; y++ {
		row := (y / cell) * g.cols
		for x := 0; x < w; x++ {
			p := pix[(y*w+x)*3:]
			s := &sum[row+x/cell]
			s[0] += float64(p[0])
			s[1] += float64(p[1])
			s[2] += float64(p[2])
			s[3]++
		}
	}
	for i, s := range sum {
		c := rgb{s[0] / s[3] / 255, s[1] / s[3] / 255, s[2] / s[3] / 255}
		g.lum[i] = 0.299*c.r + 0.587*c.g + 0.114*c.b
		g.col[i] = accent(c)
	}
	g.stretch()
	return g
}

// accent picks the palette colour closest in hue; greys become lavender.
func accent(c rgb) rgb {
	hi, lo := math.Max(c.r, math.Max(c.g, c.b)), math.Min(c.r, math.Min(c.g, c.b))
	d := hi - lo
	if d < 0.02 {
		return lavender
	}
	var hue float64
	switch hi {
	case c.r:
		hue = math.Mod((c.g-c.b)/d+6, 6)
	case c.g:
		hue = (c.b-c.r)/d + 2
	default:
		hue = (c.r-c.g)/d + 4
	}
	sat := d / (hi + 1e-9)
	return mix(lavender, accents[int(hue*2)%12], math.Min(1, sat*2.5))
}

// stretch spreads brightness over the full range so that a dark desktop
// still gives a picture.
func (g *grid) stretch() {
	s := append([]float64(nil), g.lum...)
	sort.Float64s(s)
	lo, hi := s[len(s)*5/100], s[len(s)*995/1000]
	if hi-lo < 0.05 {
		hi = lo + 0.05
	}
	for i, l := range g.lum {
		g.lum[i] = math.Max(0, math.Min(1, (l-lo)/(hi-lo)))
	}
}

// melt lets bright edges drip down and shifts a few bands sideways.
func (g *grid) melt(rng *rand.Rand) {
	for c := 0; c < g.cols; c++ {
		for r := 0; r < g.rows-1; r++ {
			i := r*g.cols + c
			edge := g.lum[i]-g.lum[i+g.cols] > 0.15
			if !(edge && rng.Float64() < 0.7) && rng.Float64() > 0.006 {
				continue
			}
			src, col := math.Max(g.lum[i], 0.55), g.col[i]
			length := 3 + rng.Intn(g.rows/3)
			for k := 1; k <= length && r+k < g.rows; k++ {
				j := i + k*g.cols
				l := src * math.Pow(1-float64(k)/float64(length+1), 0.9)
				if l > g.lum[j] {
					g.lum[j], g.col[j] = l, col
				}
			}
		}
	}
	for n := 3 + rng.Intn(4); n > 0; n-- {
		top, height, shift := rng.Intn(g.rows), 1+rng.Intn(3), rng.Intn(31)-15
		for r := top; r < top+height && r < g.rows; r++ {
			lum, col := g.lum[r*g.cols:(r+1)*g.cols], g.col[r*g.cols:(r+1)*g.cols]
			rotate(lum, shift)
			rotate(col, shift)
		}
	}
}

func rotate[T any](s []T, by int) {
	n := len(s)
	by = ((by % n) + n) % n
	tmp := append(append([]T(nil), s[n-by:]...), s[:n-by]...)
	copy(s, tmp)
}

// stamp writes the label and the time into the grid, centred in the upper
// part so that swaylock's indicator in the middle stays free.
func (g *grid) stamp(label, clock string) {
	const big, small, gap = 2, 1, 3
	wBig, wSmall := textWidth(clock)*big, textWidth(label)*small
	height := glyphH*small + gap + glyphH*big
	top := g.rows/4 - height/2
	if top < 2 {
		top = 2
	}
	width := wBig
	if wSmall > width {
		width = wSmall
	}
	left := (g.cols - width) / 2
	for r := top - 3; r < top+height+3; r++ {
		for c := left - 4; c < left+width+4; c++ {
			g.set(c, r, quiet)
		}
	}
	g.write(label, (g.cols-wSmall)/2, top, small)
	g.write(clock, (g.cols-wBig)/2, top+glyphH*small+gap, big)
}

func (g *grid) write(s string, left, top, scale int) {
	for n, ch := range s {
		glyph := font[ch]
		for y := 0; y < glyphH*scale; y++ {
			for x := 0; x < glyphW*scale; x++ {
				if glyph[y/scale]>>(glyphW-1-x/scale)&1 == 1 {
					g.set(left+n*(glyphW+1)*scale+x, top+y, inked)
				}
			}
		}
	}
}

func (g *grid) set(c, r int, m uint8) {
	if c >= 0 && c < g.cols && r >= 0 && r < g.rows {
		g.mark[r*g.cols+c] = m
	}
}

// draw renders every cell as a dot whose size and colour follow its
// brightness, darkened towards the corners.
func (g *grid) draw(w, h int) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	const half = cell / 2.0
	for r := 0; r < g.rows; r++ {
		for c := 0; c < g.cols; c++ {
			i := r*g.cols + c
			dx, dy := float64(c)/float64(g.cols)-0.5, float64(r)/float64(g.rows)-0.5
			l := g.lum[i] * (1 - 1.3*(dx*dx+dy*dy))
			col := mix(surface0, g.col[i], 0.15+0.85*l)
			switch g.mark[i] {
			case inked:
				l, col = 1, text
			case quiet:
				l = l * 0.2
			}
			radius := 1 + (half-1.6)*math.Pow(l, 0.7)
			for y := r * cell; y < (r+1)*cell && y < h; y++ {
				for x := c * cell; x < (c+1)*cell && x < w; x++ {
					d := math.Hypot(float64(x-c*cell)+0.5-half, float64(y-r*cell)+0.5-half)
					p := mix(crust, col, math.Max(0, math.Min(1, radius-d+0.5)))
					o := img.Pix[y*img.Stride+x*4:]
					o[0], o[1], o[2], o[3] = uint8(p.r*255+0.5), uint8(p.g*255+0.5), uint8(p.b*255+0.5), 255
				}
			}
		}
	}
	return img
}
