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

// paint draws the cells that differ from what the buffer already shows.
// pix is BGRX, drawn remembers the buffer's contents per cell. It returns
// the changed area as one rectangle per row of cells.
func paint(pix []byte, stride, w, h, cols, rows int, want, drawn []uint32) []rect {
	bg := [3]int{int(crust.b*255 + 0.5), int(crust.g*255 + 0.5), int(crust.r*255 + 0.5)}
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
