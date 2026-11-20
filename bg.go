package main

import (
	"errors"
	"fmt"
	"image"
	"image/png"
	"os"
	"path/filepath"
	"syscall"
	"time"
)

const (
	fps          = 30
	layerBack    = 0
	layerOverlay = 3
	formatXRGB   = 1
	anchorAll    = 15
)

type buffer struct {
	id    uint32
	pix   []byte
	drawn []uint32
	busy  bool
}

// background is the animated wallpaper: one layer surface on one output,
// drawn into two shared-memory buffers.
type background struct {
	w                              *wl
	dir                            string
	layer                          uint32
	effect                         string
	registry, compositor, shm      uint32
	shell, output, surface, lsurf  uint32
	frame                          uint32
	outName                        string
	width, height                  int
	bufs                           [2]buffer
	sc                             *scene
	want                           []uint32
	pending, closed, needConfigure bool
}

// runBackground draws the animation until the compositor closes it. It is
// what swaylock-plugin starts once per output.
func runBackground(dir string, layer uint32, effect string) error {
	w, err := wlConnect()
	if err != nil {
		return err
	}
	b := &background{w: w, dir: dir, layer: layer, effect: effect, needConfigure: true}
	b.registry = w.newID()
	w.req(1, 1, b.registry) // wl_display.get_registry
	if err := b.roundtrip(); err != nil {
		return err
	}
	if b.compositor == 0 || b.shm == 0 || b.shell == 0 || b.output == 0 {
		return errors.New("compositor lacks wl_compositor, wl_shm, layer shell or an output")
	}
	if err := b.roundtrip(); err != nil { // output name
		return err
	}

	b.surface, b.lsurf = w.newID(), w.newID()
	w.req(b.compositor, 0, b.surface)                                   // create_surface
	w.req(b.shell, 0, b.lsurf, b.surface, b.output, layer, "wallpaper") // get_layer_surface
	w.req(b.lsurf, 0, uint32(0), uint32(0))                             // set_size: fill
	w.req(b.lsurf, 1, uint32(anchorAll))                                // set_anchor
	w.req(b.lsurf, 2, int32(-1))                                        // set_exclusive_zone
	w.req(b.surface, 6)                                                 // commit

	last, next := time.Now(), time.Now()
	for !b.closed {
		deadline := next
		if b.pending || b.sc == nil {
			deadline = time.Time{} // wait for the frame callback or configure
		}
		ev, ok, err := w.poll(deadline)
		if err != nil {
			return err
		}
		if ok {
			if err := b.handle(ev); err != nil {
				return err
			}
			continue
		}
		now := time.Now()
		dt := now.Sub(last).Seconds()
		if dt > 0.1 {
			dt = 0.1 // after a pause, carry on rather than jump ahead
		}
		last, next = now, now.Add(time.Second/fps)
		b.draw(dt, now)
	}
	return nil
}

func (b *background) roundtrip() error {
	cb := b.w.newID()
	b.w.req(1, 0, cb) // wl_display.sync
	for {
		ev, _, err := b.w.poll(time.Time{})
		if err != nil {
			return err
		}
		if ev.obj == cb {
			return nil
		}
		if err := b.handle(ev); err != nil {
			return err
		}
	}
}

func (b *background) handle(ev event) error {
	w := b.w
	switch ev.obj {
	case 1: // wl_display
		if ev.op == 0 {
			msg, _ := ev.str(8)
			return fmt.Errorf("wayland error on object %d: %s", ev.u32(0), msg)
		}
		w.free = append(w.free, ev.u32(0)) // delete_id
	case b.registry:
		if ev.op != 0 {
			break
		}
		name := ev.u32(0)
		iface, off := ev.str(4)
		version := le.Uint32(ev.body[off:])
		bind := func(target *uint32, want uint32) {
			if *target == 0 {
				*target = w.newID()
				w.req(b.registry, 0, name, iface, min(version, want), *target)
			}
		}
		switch iface {
		case "wl_compositor":
			bind(&b.compositor, 4)
		case "wl_shm":
			bind(&b.shm, 1)
		case "zwlr_layer_shell_v1":
			bind(&b.shell, 1)
		case "wl_output":
			bind(&b.output, 4)
		}
	case b.output:
		if ev.op == 4 { // name
			b.outName, _ = ev.str(0)
		}
	case b.lsurf:
		if ev.op == 1 {
			b.closed = true
			break
		}
		w.req(b.lsurf, 6, ev.u32(0)) // ack_configure
		if width, height := int(ev.u32(1)), int(ev.u32(2)); width > 0 && height > 0 &&
			(width != b.width || height != b.height) {
			return b.resize(width, height)
		}
	case b.frame:
		b.pending = false
	default:
		for i := range b.bufs {
			if ev.obj == b.bufs[i].id && ev.op == 0 {
				b.bufs[i].busy = false
			}
		}
	}
	return nil
}

// resize allocates the buffers and starts the scene for a surface size.
func (b *background) resize(width, height int) error {
	stride := width * 4
	size := stride * height * len(b.bufs)
	f, err := os.CreateTemp(os.Getenv("XDG_RUNTIME_DIR"), "meltlock-shm-")
	if err != nil {
		return err
	}
	defer f.Close()
	os.Remove(f.Name())
	if err := f.Truncate(int64(size)); err != nil {
		return err
	}
	data, err := syscall.Mmap(int(f.Fd()), 0, size, syscall.PROT_READ|syscall.PROT_WRITE, syscall.MAP_SHARED)
	if err != nil {
		return err
	}
	cols, rows := (width+cell-1)/cell, (height+cell-1)/cell
	pool := b.w.newID()
	b.w.req(b.shm, 0, pool, fd(f.Fd()), int32(size)) // create_pool
	for i := range b.bufs {
		drawn := make([]uint32, cols*rows)
		for k := range drawn {
			drawn[k] = ^uint32(0)
		}
		off := i * stride * height
		b.bufs[i] = buffer{id: b.w.newID(), pix: data[off : off+stride*height], drawn: drawn}
		b.w.req(pool, 0, b.bufs[i].id, int32(off), int32(width), int32(height), int32(stride), uint32(formatXRGB))
	}
	if err := b.w.flush(); err != nil { // the fd must be sent before it is closed
		return err
	}
	snap, _ := loadSnapshot(filepath.Join(b.dir, b.outName+".snap"))
	b.width, b.height = width, height
	b.sc = newScene(cols, rows, snap, time.Now().UnixNano(), b.effect)
	b.want = make([]uint32, cols*rows)
	b.pending = false
	return nil
}

func (b *background) draw(dt float64, now time.Time) {
	var buf *buffer
	for i := range b.bufs {
		if !b.bufs[i].busy {
			buf = &b.bufs[i]
			break
		}
	}
	if buf == nil {
		return
	}
	b.sc.step(dt, now)
	damage := b.sc.render(buf.pix, b.width*4, b.width, b.height, b.want, buf.drawn)
	if len(damage) == 0 {
		return
	}
	w := b.w
	w.req(b.surface, 1, buf.id, int32(0), int32(0)) // attach
	for _, d := range damage {
		w.req(b.surface, 9, int32(d.x), int32(d.y), int32(d.w), int32(d.h)) // damage_buffer
	}
	b.frame = w.newID()
	w.req(b.surface, 3, b.frame) // frame
	w.req(b.surface, 6)          // commit
	buf.busy, b.pending = true, true
}

// dumpFrames simulates the scene without a display and writes it as PNG
// at the given times, for judging the animation.
func dumpFrames(dir string, snap snapshot, at []float64, effect string) error {
	snap.Lum = append([]float64(nil), snap.Lum...) // effects change the picture
	width, height := snap.Cols*cell, snap.Rows*cell
	sc := newScene(snap.Cols, snap.Rows, snap, 1, effect)
	want := make([]uint32, snap.Cols*snap.Rows)
	pix := make([]byte, width*height*4)
	now := time.Now()
	t := 0.0
	for _, stop := range at {
		for ; t < stop; t += 1.0 / fps {
			sc.step(1.0/fps, now.Add(time.Duration(t*float64(time.Second))))
		}
		drawn := make([]uint32, len(want))
		for k := range drawn {
			drawn[k] = ^uint32(0)
		}
		sc.render(pix, width*4, width, height, want, drawn)
		img := image.NewRGBA(image.Rect(0, 0, width, height))
		for i := 0; i < len(pix); i += 4 {
			img.Pix[i], img.Pix[i+1], img.Pix[i+2], img.Pix[i+3] = pix[i+2], pix[i+1], pix[i], 255
		}
		f, err := os.Create(filepath.Join(dir, fmt.Sprintf("%s-%04.0fs.png", effect, stop)))
		if err != nil {
			return err
		}
		err = png.Encode(f, img)
		f.Close()
		if err != nil {
			return err
		}
	}
	return nil
}
