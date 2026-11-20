package main

import "math"

// silk: thousands of fine threads flow across the screen. They follow the
// outlines of the desktop and a slowly turning wind, take their colour
// from what lies beneath them and fade behind them.
type silk struct {
	*canvas
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
	k := &silk{canvas: newCanvas(s)}
	n := s.cols * s.rows

	// Outlines: the gradient of the slightly blurred picture, turned a
	// quarter so that threads run along edges rather than across them.
	soft := s.soft()
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

	k.picture(s, 0.6) // fades as the threads grow
	k.ps = make([]thread, n/2)
	for i := range k.ps {
		k.spawn(s, &k.ps[i])
		k.ps[i].life *= s.rng.Float64() // so they do not all end together
	}
	return k
}

func (k *silk) spawn(s *scene, p *thread) {
	p.x, p.y = s.bright()
	col := s.col[s.cellAt(p.x, p.y)]
	p.c = [3]float64{col.b * 255, col.g * 255, col.r * 255}
	p.life, p.speed = 3+7*s.rng.Float64(), 30+50*s.rng.Float64()
}

func (k *silk) step(s *scene, dt float64) {
	for k.owed += dt * silkFade; k.owed >= 1; k.owed-- {
		k.fade(4)
	}
	wind := s.t * silkWind
	for n := range k.ps {
		p := &k.ps[n]
		if p.life -= dt; p.life <= 0 || p.x < 0 || p.y < 0 || p.x >= float64(k.w) || p.y >= float64(k.h) {
			k.spawn(s, p)
			continue
		}
		i, j, fx, fy := s.corners(p.x, p.y)
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
		here := s.cellAt(p.x, p.y)
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
