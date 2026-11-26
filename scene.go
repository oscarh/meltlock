package main

import (
	"encoding/gob"
	"math"
	"math/rand"
	"os"
	"sort"
	"strings"
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

// effect is one way of animating the dot wall. The scene owns the picture
// (lum and col, which an effect may change), the clock and the palette.
type effect interface {
	// step advances the effect by dt seconds; s.t is already updated.
	step(s *scene, dt float64)
	// cell returns the brightness (0..1) and colour of one cell.
	cell(s *scene, c, r int) (float64, rgb)
}

// effects in the order they are listed to the user.
var effects = []struct {
	name  string
	shape uint32
	make  func(*scene) effect
}{
	{"melt", shapeDot, newMelt},
	{"sand", shapeSquare, newSand},
	{"ripple", shapeDiamond, newRipple},
	{"drift", shapeBar, newDrift},
	{"life", shapeSquare, newLife},
	{"pulse", shapeDot, newPulse},
	{"silk", shapeDot, newSilk},
	{"ridge", shapeDot, newRidge},
	{"contour", shapeDot, newContour},
	{"frost", shapeDot, newFrost},
}

// painter is an effect that draws the pixels itself, at full resolution,
// instead of leaving its cells to be drawn as shapes.
type painter interface {
	// frame fills a BGRX buffer of w by h pixels.
	frame(s *scene, pix []byte, stride, w, h int)
}

// pickEffect chooses an effect from spec: empty or "random" means any, a
// comma-separated list means one of those. Unknown names are ignored.
func pickEffect(spec string, rng *rand.Rand) string {
	var known []string
	for _, want := range strings.Split(spec, ",") {
		for _, e := range effects {
			if e.name == strings.TrimSpace(want) {
				known = append(known, e.name)
			}
		}
	}
	if len(known) == 0 {
		return effects[rng.Intn(len(effects))].name
	}
	return known[rng.Intn(len(known))]
}

// scene is the animated dot wall.
type scene struct {
	grid
	vig      []float64 // darkening towards the corners
	rng      *rand.Rand
	t        float64
	fx       effect
	shape    uint32 // how cells are drawn, see paint.go
	stamped  string
	shade    []uint8 // per pixel of shadeBox: how strongly the clock dims a painter's frame
	shadeBox rect
	shadeFor string
}

// newScene builds a scene of the given size. A snapshot of another size
// (or none) is replaced by a faint random field, and an unknown effect by
// the first one.
func newScene(cols, rows int, snap snapshot, seed int64, fx string) *scene {
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
	s.vig = make([]float64, n)
	for r := 0; r < rows; r++ {
		for c := 0; c < cols; c++ {
			dx, dy := float64(c)/float64(cols)-0.5, float64(r)/float64(rows)-0.5
			s.vig[r*cols+c] = 1 - 1.3*(dx*dx+dy*dy)
		}
	}
	pick := effects[0]
	for _, e := range effects[1:] {
		if e.name == fx {
			pick = e
		}
	}
	s.shape, s.fx = pick.shape, pick.make(s)
	return s
}

// step advances the animation by dt seconds.
func (s *scene) step(dt float64, now time.Time) {
	s.t += dt
	s.fx.step(s, dt)

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
		s.stamp(strings.ToUpper(now.Format("Mon 2 Jan")), clock)
	}
}

// cells returns what every cell should look like now: shape (3 bits) and
// size level (5 bits) in the top byte, colour as 0xRRGGBB below.
func (s *scene) cells(out []uint32) {
	for r := 0; r < s.rows; r++ {
		for c := 0; c < s.cols; c++ {
			i := r*s.cols + c
			l, col := s.fx.cell(s, c, r)
			switch s.mark[i] {
			case inked:
				l, col = 1, text
			case quiet:
				l *= 0.2
			}
			l = math.Max(0, math.Min(1, l))
			level := int(math.Pow(l, 0.7)*(levels-1) + 0.5)
			p := mix(surface0, col, 0.15+0.85*float64(level)/(levels-1))
			out[i] = (s.shape<<5|uint32(level))<<24 | uint32(p.r*255+0.5)<<16 | uint32(p.g*255+0.5)<<8 | uint32(p.b*255+0.5)
		}
	}
}

// brightest returns the brightness that the given share of cells exceed.
func (s *scene) brightest(share float64) float64 {
	sorted := append([]float64(nil), s.lum...)
	sort.Float64s(sorted)
	return sorted[int(float64(len(sorted)-1)*(1-share))]
}
