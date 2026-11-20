package main

import "testing"

func BenchmarkSilk(b *testing.B) {
	g := sample(make([]byte, 1920*1200*3), 1920, 1200)
	for i := range g.lum {
		g.lum[i] = float64(i%97) / 97
	}
	pix := make([]byte, 1920*1200*4)
	b.Run("start", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			newScene(g.cols, g.rows, g.snapshot(), 1, "silk")
		}
	})
	sc := newScene(g.cols, g.rows, g.snapshot(), 1, "silk")
	k := sc.fx.(*silk)
	b.Run("step", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			k.step(sc, 1.0/fps)
		}
	})
	b.Run("fade", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			k.fade()
		}
	})
	b.Run("frame", func(b *testing.B) {
		for i := 0; i < b.N; i++ {
			k.frame(sc, pix, 1920*4, 1920, 1200)
			sc.overlay(pix, 1920*4, 1920, 1200)
		}
	})
}
