package main

import (
	"bytes"
	"fmt"
	"math/rand"
	"testing"
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
	for _, ch := range "LOCKED SINCE0123456789:" {
		if _, ok := font[ch]; !ok {
			t.Errorf("no glyph for %q", ch)
		}
	}
}
