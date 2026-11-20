package main

import "math"

// ridge: the desktop as a landscape. Lines cross the screen and rise where
// the picture is bright, each hiding the ones behind it, and the whole
// stack travels slowly upwards over a ground that heaves a little.
type ridge struct {
	*canvas
	soft    []float64
	horizon []float64 // per pixel column: the highest point drawn so far
	hs      []float64 // heights of one line at the cell columns
	scroll  float64
}

const (
	ridgeGap   = 22.0  // pixels between lines
	ridgeAmp   = 150.0 // height of the brightest parts
	ridgeSpeed = 5.0   // pixels per second upwards
)

func newRidge(s *scene) effect {
	k := &ridge{canvas: newCanvas(s), soft: s.soft()}
	k.horizon, k.hs = make([]float64, k.w), make([]float64, s.cols+4)
	return k
}

func (k *ridge) step(s *scene, dt float64) {
	k.scroll = math.Mod(k.scroll+ridgeSpeed*dt, ridgeGap)
	k.clear()
	for x := range k.horizon {
		k.horizon[x] = math.Inf(1)
	}
	// Front to back: a line shows only where it rises above all those
	// already drawn.
	h := float64(k.h)
	for base := h + 2*ridgeGap - k.scroll; base > 0; base -= ridgeGap {
		// Lines come in flat at the bottom and go out faint at the top.
		amp := ridgeAmp * math.Min(1, math.Max(0, (h+ridgeGap-base)/120))
		fade := math.Min(1, base/90)
		for c := range k.hs {
			x := (float64(c-2) + 0.5) * cell
			swell := noise3(x/280, base/280, s.t*0.11)
			k.hs[c] = amp * (s.at(k.soft, x, base)*(0.45+0.8*swell) + 0.12*swell)
		}
		row := min(max(int(base)/cell, 0), s.rows-1) * s.cols
		var col ink
		prev, seen := 0.0, false
		for x := 0; x < k.w; x++ {
			// Smooth curve through the heights at the cell columns.
			cx := (float64(x)+0.5)/cell - 0.5
			c0 := int(math.Floor(cx))
			t := cx - float64(c0)
			p := k.hs[c0+1 : c0+5]
			height := p[1] + 0.5*t*(p[2]-p[0]+t*(2*p[0]-5*p[1]+4*p[2]-p[3]+t*(3*(p[1]-p[2])+p[3]-p[0])))
			y := base - height

			if x%cell == 0 {
				col = inkOf(mix(s.col[row+x/cell], text, 0.15))
			}
			visible := y < k.horizon[x]
			if visible {
				k.horizon[x] = y
				from := y
				if seen {
					from = prev
				}
				a := (0.45 + 0.55*math.Min(1, height/(0.3*ridgeAmp))) * fade * s.vig[row+x/cell]
				k.span(x, from, y, int(a*256), &col)
			}
			prev, seen = y, visible
		}
	}
}

// span draws the piece of a line that falls in one pixel column, between
// two heights.
func (k *canvas) span(x int, y0, y1 float64, a int, c *ink) {
	if y0 > y1 {
		y0, y1 = y1, y0
	}
	y0, y1 = y0-0.5, y1-0.5
	top, bottom := int(math.Floor(y0)), int(math.Floor(y1))+1
	k.put(x, top, a*(256-int((y0-math.Floor(y0))*256))>>8, c)
	for y := top + 1; y < bottom; y++ {
		k.put(x, y, a, c)
	}
	k.put(x, bottom, a*int((y1-math.Floor(y1))*256)>>8, c)
}
