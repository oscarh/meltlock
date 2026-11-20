package main

import (
	"encoding/binary"
	"errors"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strconv"
	"syscall"
	"time"
)

// wl is a minimal Wayland client: just the wire format, object ids and
// file descriptor passing that a shared-memory wallpaper needs.
type wl struct {
	conn *net.UnixConn
	in   []byte
	out  []byte
	fds  []int
	next uint32
	free []uint32
}

type fd int

var le = binary.LittleEndian

func wlConnect() (*wl, error) {
	var c net.Conn
	var err error
	if s := os.Getenv("WAYLAND_SOCKET"); s != "" {
		n, e := strconv.Atoi(s)
		if e != nil {
			return nil, errors.New("bad WAYLAND_SOCKET")
		}
		f := os.NewFile(uintptr(n), "wayland")
		c, err = net.FileConn(f)
		f.Close()
	} else {
		name := os.Getenv("WAYLAND_DISPLAY")
		if name == "" {
			name = "wayland-0"
		}
		if !filepath.IsAbs(name) {
			name = filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), name)
		}
		c, err = net.Dial("unix", name)
	}
	if err != nil {
		return nil, err
	}
	u, ok := c.(*net.UnixConn)
	if !ok {
		return nil, errors.New("wayland socket is not a unix socket")
	}
	return &wl{conn: u, next: 2}, nil
}

// newID returns an unused object id, reusing ids the server has released.
func (w *wl) newID() uint32 {
	if n := len(w.free); n > 0 {
		id := w.free[n-1]
		w.free = w.free[:n-1]
		return id
	}
	w.next++
	return w.next - 1
}

// req queues a request. Arguments may be uint32, int32, string or fd.
func (w *wl) req(obj uint32, op uint16, args ...any) {
	start := len(w.out)
	w.out = append(w.out, 0, 0, 0, 0, 0, 0, 0, 0)
	for _, a := range args {
		switch v := a.(type) {
		case uint32:
			w.out = le.AppendUint32(w.out, v)
		case int32:
			w.out = le.AppendUint32(w.out, uint32(v))
		case string:
			w.out = le.AppendUint32(w.out, uint32(len(v)+1))
			w.out = append(w.out, v...)
			for pad := 4 - len(v)%4; pad > 0; pad-- {
				w.out = append(w.out, 0)
			}
		case fd:
			w.fds = append(w.fds, int(v))
		default:
			panic(fmt.Sprintf("wl: unsupported argument %T", a))
		}
	}
	le.PutUint32(w.out[start:], obj)
	le.PutUint32(w.out[start+4:], uint32(len(w.out)-start)<<16|uint32(op))
}

func (w *wl) flush() error {
	if len(w.out) == 0 {
		return nil
	}
	var oob []byte
	if len(w.fds) > 0 {
		oob = syscall.UnixRights(w.fds...)
	}
	_, _, err := w.conn.WriteMsgUnix(w.out, oob, nil)
	w.out, w.fds = w.out[:0], w.fds[:0]
	return err
}

type event struct {
	obj  uint32
	op   uint16
	body []byte
}

func (e event) u32(i int) uint32 { return le.Uint32(e.body[i*4:]) }

// str reads a string argument that starts at byte offset off and returns
// it with the offset of the next argument.
func (e event) str(off int) (string, int) {
	n := int(le.Uint32(e.body[off:]))
	if n == 0 {
		return "", off + 4
	}
	return string(e.body[off+4 : off+4+n-1]), off + 4 + (n+3)&^3
}

// poll flushes pending requests and returns the next event. With a
// deadline it returns ok=false when the deadline passes first.
func (w *wl) poll(deadline time.Time) (ev event, ok bool, err error) {
	if err = w.flush(); err != nil {
		return
	}
	for {
		if len(w.in) >= 8 {
			size := int(le.Uint32(w.in[4:]) >> 16)
			if size < 8 {
				return ev, false, errors.New("wl: malformed event")
			}
			if len(w.in) >= size {
				ev = event{le.Uint32(w.in), uint16(le.Uint32(w.in[4:])), append([]byte(nil), w.in[8:size]...)}
				w.in = w.in[size:]
				return ev, true, nil
			}
		}
		w.conn.SetReadDeadline(deadline)
		buf := make([]byte, 4096)
		n, e := w.conn.Read(buf)
		if n > 0 {
			w.in = append(w.in, buf[:n]...)
			continue
		}
		if errors.Is(e, os.ErrDeadlineExceeded) {
			return ev, false, nil
		}
		if e == nil {
			e = errors.New("wl: connection closed")
		}
		return ev, false, e
	}
}
