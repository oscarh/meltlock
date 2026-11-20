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
	sc := newScene(10, 5, snap, 1)
	if sc.lum[3] != 0.7 || sc.col[3] != (rgb{0.1, 0.2, 0.3}) {
		t.Error("scene did not take the snapshot")
	}
	want := make([]uint32, 50)
	drawn := make([]uint32, 50)
	pix := make([]byte, 120*60*4)
	now := time.Date(2026, 11, 20, 10, 45, 0, 0, time.UTC)
	for i := 0; i < 600; i++ {
		sc.step(1.0/fps, now)
	}
	sc.cells(want)
	for i, k := range want {
		if k>>24 >= levels {
			t.Fatalf("cell %d has level %d", i, k>>24)
		}
	}
	if len(paint(pix, 120*4, 120, 60, 10, 5, want, drawn)) == 0 {
		t.Error("first paint reported no damage")
	}
	if len(paint(pix, 120*4, 120, 60, 10, 5, want, drawn)) != 0 {
		t.Error("repainting the same cells reported damage")
	}
	// A snapshot of another size is ignored rather than trusted.
	if other := newScene(7, 3, snap, 1); len(other.lum) != 21 {
		t.Error("scene used a snapshot of the wrong size")
	}
}
