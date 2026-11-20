package main

import "math"

// Cell shapes. Each effect draws its cells in one of them.
const (
	shapeDot = iota
	shapeSquare
	shapeBar // a horizontal bar across the whole cell
	shapeDiamond
	shapes
)

// masks holds the coverage of each shape within its cell for every size
// level.
var masks = func() (m [shapes][levels][cell * cell]uint8) {
	const half = cell / 2.0
	for shape := range m {
		for k := range m[shape] {
			f := float64(k) / (levels - 1)
			for y := 0; y < cell; y++ {
				for x := 0; x < cell; x++ {
					dx, dy := math.Abs(float64(x)+0.5-half), math.Abs(float64(y)+0.5-half)
					var inside float64 // distance inside the shape's edge
					switch shape {
					case shapeDot:
						inside = 1 + (half-1.6)*f - math.Hypot(dx, dy)
					case shapeSquare:
						inside = 1 + (half-1.5)*f - math.Max(dx, dy)
					case shapeBar:
						inside = 0.5 + (half-1.5)*f - dy
					case shapeDiamond:
						inside = 1.4 + (half-0.4)*f - (dx + dy)
					}
					m[shape][k][y*cell+x] = uint8(255*math.Max(0, math.Min(1, inside+0.5)) + 0.5)
				}
			}
		}
	}
	return m
}()

type rect struct{ x, y, w, h int }

func bgrOf(c rgb) [3]int {
	return [3]int{int(c.b*255 + 0.5), int(c.g*255 + 0.5), int(c.r*255 + 0.5)}
}

// render draws the scene into a BGRX buffer and returns the changed area.
// want and drawn are per-cell scratch and memory for the cell effects.
func (s *scene) render(pix []byte, stride, w, h int, want, drawn []uint32) []rect {
	p, ok := s.fx.(painter)
	if !ok {
		s.cells(want)
		return paint(pix, stride, w, h, s.cols, s.rows, want, drawn)
	}
	p.frame(s, pix, stride, w, h)
	s.overlay(pix, stride, w, h)
	return []rect{{0, 0, w, h}}
}

// overlay draws the clock as dots over a painter's frame and dims the
// area around it, with a soft edge.
func (s *scene) overlay(pix []byte, stride, w, h int) {
	const reach = 3 // cells over which the dimming tapers off
	const full = (2*reach + 1) * (2*reach + 1)
	if s.shadeFor != s.stamped || len(s.shade) == 0 {
		// How strongly each cell is dimmed, then that blended per pixel
		// over the cells around the clock.
		s.shadeFor = s.stamped
		cells := make([]float64, len(s.mark))
		box := rect{s.cols, s.rows, 0, 0} // left, top, right, bottom in cells
		for i, m := range s.mark {
			if m == plain {
				continue
			}
			for r := max(i/s.cols-reach, 0); r <= min(i/s.cols+reach, s.rows-1); r++ {
				for c := max(i%s.cols-reach, 0); c <= min(i%s.cols+reach, s.cols-1); c++ {
					cells[r*s.cols+c] += 1.0 / full
					// One cell further, where the blend reaches nothing.
					box = rect{max(min(box.x, c-1), 0), max(min(box.y, r-1), 0), min(max(box.w, c+2), s.cols), min(max(box.h, r+2), s.rows)}
				}
			}
		}
		for i, share := range cells {
			cells[i] = 200 * smooth(share) // eased, so the taper has no visible start or end
		}
		s.shadeBox = rect{box.x * cell, box.y * cell, max(0, box.w-box.x) * cell, max(0, box.h-box.y) * cell}
		s.shade = make([]uint8, s.shadeBox.w*s.shadeBox.h+1)
		for y := 0; y < s.shadeBox.h; y++ {
			for x := 0; x < s.shadeBox.w; x++ {
				s.shade[y*s.shadeBox.w+x] = uint8(s.at(cells, float64(s.shadeBox.x+x)+0.5, float64(s.shadeBox.y+y)+0.5))
			}
		}
	}
	bg, fg := bgrOf(crust), bgrOf(text)
	box := s.shadeBox
	for y := box.y; y < min(box.y+box.h, h); y++ {
		row := pix[y*stride:]
		for x := box.x; x < min(box.x+box.w, w); x++ {
			keep := 256 - int(s.shade[(y-box.y)*box.w+x-box.x]) // share of the picture that shows through
			o := row[x*4:]
			for ch := 0; ch < 3; ch++ {
				o[ch] = uint8(bg[ch] + (int(o[ch])-bg[ch])*keep/256)
			}
		}
	}
	mask := &masks[shapeDot][levels-1]
	for i, m := range s.mark {
		if m != inked {
			continue
		}
		c, r := i%s.cols, i/s.cols
		for y := 0; y < cell && r*cell+y < h; y++ {
			row := pix[(r*cell+y)*stride:]
			for x := 0; x < cell && c*cell+x < w; x++ {
				o, a := row[(c*cell+x)*4:], int(mask[y*cell+x])
				for ch := 0; ch < 3; ch++ {
					o[ch] = uint8(int(o[ch]) + (fg[ch]-int(o[ch]))*a/255)
				}
			}
		}
	}
}

// paint draws the cells that differ from what the buffer already shows.
// pix is BGRX, drawn remembers the buffer's contents per cell. It returns
// the changed area as one rectangle per row of cells.
func paint(pix []byte, stride, w, h, cols, rows int, want, drawn []uint32) []rect {
	bg := bgrOf(crust)
	var damage []rect
	for r := 0; r < rows; r++ {
		first, last := -1, -1
		for c := 0; c < cols; c++ {
			i := r*cols + c
			key := want[i]
			if drawn[i] == key {
				continue
			}
			drawn[i] = key
			if first < 0 {
				first = c
			}
			last = c
			mask := &masks[key>>29][key>>24&(levels-1)]
			fg := [3]int{int(key & 0xff), int(key >> 8 & 0xff), int(key >> 16 & 0xff)}
			for y := 0; y < cell && r*cell+y < h; y++ {
				row := pix[(r*cell+y)*stride:]
				for x := 0; x < cell && c*cell+x < w; x++ {
					a := int(mask[y*cell+x])
					o := row[(c*cell+x)*4:]
					o[0] = uint8(bg[0] + (fg[0]-bg[0])*a/255)
					o[1] = uint8(bg[1] + (fg[1]-bg[1])*a/255)
					o[2] = uint8(bg[2] + (fg[2]-bg[2])*a/255)
					o[3] = 255
				}
			}
		}
		if first >= 0 {
			x, y := first*cell, r*cell
			damage = append(damage, rect{x, y, min((last+1)*cell, w) - x, min(y+cell, h) - y})
		}
	}
	return damage
}
