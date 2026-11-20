package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
	"time"
)

func TestPipeline(t *testing.T) {
	const w, h = 250, 130
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "P6\n%d %d\n255\n", w, h)
	for i := 0; i < w*h; i++ {
		buf.Write([]byte{byte(i), byte(i / w), 200})
	}
	pix, gw, gh, err := readPPM(&buf)
	if err != nil || gw != w || gh != h {
		t.Fatalf("readPPM: %v %dx%d", err, gw, gh)
	}
	g := sample(pix, w, h)
	if g.cols != 21 || g.rows != 11 {
		t.Fatalf("grid is %dx%d", g.cols, g.rows)
	}
	g.melt(rand.New(rand.NewSource(1)))
	g.stamp("LOCKED SINCE", "10:30") // wider than the grid: must not panic
	for i, l := range g.lum {
		if l < 0 || l > 1 {
			t.Fatalf("lum[%d] = %v", i, l)
		}
	}
	if img := g.draw(w, h); img.Bounds().Dx() != w || img.Bounds().Dy() != h {
		t.Errorf("image is %v", img.Bounds())
	}
}

func TestReadPPMRejectsOtherFormats(t *testing.T) {
	if _, _, _, err := readPPM(bytes.NewBufferString("P5\n1 1\n255\n\x00")); err == nil {
		t.Error("P5 accepted")
	}
}

func TestFontCoversStamp(t *testing.T) {
	for _, ch := range "LOCKED SINCE0123456789: MONTUEWEDTHUFRISATSUNJANFEBMARAPRMAYJUNJULAUGSEPOCTNOVDEC" {
		if _, ok := font[ch]; !ok {
			t.Errorf("no glyph for %q", ch)
		}
	}
}

func TestSnapshotRoundTripAndScene(t *testing.T) {
	g := sample(make([]byte, 120*60*3), 120, 60)
	g.lum[3], g.col[3] = 0.7, rgb{0.1, 0.2, 0.3}
	path := t.TempDir() + "/out.snap"
	if err := g.snapshot().save(path); err != nil {
		t.Fatal(err)
	}
	snap, err := loadSnapshot(path)
	if err != nil || snap.Cols != 10 || snap.Rows != 5 || snap.Col[3] != [3]float64{0.1, 0.2, 0.3} {
		t.Fatalf("loadSnapshot: %v %+v", err, snap)
	}
	sc := newScene(10, 5, snap, 1, "melt")
	if sc.lum[3] != 0.7 || sc.col[3] != (rgb{0.1, 0.2, 0.3}) {
		t.Error("scene did not take the snapshot")
	}
	// A snapshot of another size is ignored rather than trusted.
	if other := newScene(7, 3, snap, 1, "melt"); len(other.lum) != 21 {
		t.Error("scene used a snapshot of the wrong size")
	}
}

func TestEveryEffectRunsAndPaints(t *testing.T) {
	const cols, rows = 40, 30
	now := time.Date(2026, 11, 20, 10, 45, 0, 0, time.UTC)
	for _, e := range effects {
		g := sample(make([]byte, cols*cell*rows*cell*3), cols*cell, rows*cell)
		rng := rand.New(rand.NewSource(2))
		for i := range g.lum {
			g.lum[i], g.col[i] = rng.Float64(), accents[rng.Intn(len(accents))]
		}
		sc := newScene(cols, rows, g.snapshot(), 1, e.name)
		want, drawn := make([]uint32, cols*rows), make([]uint32, cols*rows)
		pix := make([]byte, cols*cell*rows*cell*4)
		for i := 0; i < 1500; i++ {
			sc.step(1.0/fps, now.Add(time.Duration(i)*time.Second/fps))
		}
		sc.cells(want)
		for i, k := range want {
			if k>>29 >= shapes {
				t.Fatalf("%s: cell %d has level %d", e.name, i, k>>24)
			}
		}
		if len(paint(pix, cols*cell*4, cols*cell, rows*cell, cols, rows, want, drawn)) == 0 {
			t.Errorf("%s: first paint reported no damage", e.name)
		}
		if len(paint(pix, cols*cell*4, cols*cell, rows*cell, cols, rows, want, drawn)) != 0 {
			t.Errorf("%s: repainting the same cells reported damage", e.name)
		}
	}
}

func TestPickEffect(t *testing.T) {
	rng := rand.New(rand.NewSource(3))
	for i := 0; i < 50; i++ {
		if got := pickEffect("sand, life,nonsense", rng); got != "sand" && got != "life" {
			t.Fatalf("pickEffect chose %q", got)
		}
	}
	if got := pickEffect("nonsense", rng); got == "" || got == "nonsense" {
		t.Errorf("unknown effect gave %q", got)
	}
}
