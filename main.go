// meltlock locks the screen with swaylock, showing the current desktop
// melted into dots in the Catppuccin Mocha palette.
//
// swaylock does the locking and the password check. meltlock only makes
// the picture, and always starts swaylock even if the picture fails.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image/png"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"sync"
	"syscall"
	"time"
)

// swaylock indicator colours, rrggbbaa.
var lockArgs = []string{
	"--daemonize", "--ignore-empty-password", "--show-failed-attempts",
	"--scaling", "fill", "--color", "11111b",
	"--font", "FiraCode Nerd Font",
	"--indicator-radius", "80", "--indicator-thickness", "8",
	"--inside-color", "11111bdd", "--inside-clear-color", "11111bdd",
	"--inside-ver-color", "11111bdd", "--inside-wrong-color", "11111bdd",
	"--ring-color", "b4befe", "--ring-clear-color", "f9e2af",
	"--ring-ver-color", "89b4fa", "--ring-wrong-color", "f38ba8",
	"--key-hl-color", "cba6f7", "--bs-hl-color", "f38ba8",
	"--text-color", "cdd6f4", "--text-clear-color", "f9e2af",
	"--text-ver-color", "89b4fa", "--text-wrong-color", "f38ba8",
	"--line-uses-inside", "--separator-color", "00000000",
}

func main() {
	preview := len(os.Args) > 1 && os.Args[1] == "preview"
	if !preview && exec.Command("pgrep", "-x", "swaylock").Run() == nil {
		return // already locked
	}
	dir := filepath.Join(os.Getenv("XDG_RUNTIME_DIR"), "meltlock")
	if preview && len(os.Args) > 2 {
		dir = os.Args[2]
	}
	images := render(dir)
	if preview {
		for _, i := range images {
			fmt.Println(i)
		}
		return
	}
	args := append([]string{"swaylock"}, lockArgs...)
	for _, i := range images {
		args = append(args, "--image", i)
	}
	path, err := exec.LookPath("swaylock")
	if err == nil {
		err = syscall.Exec(path, args, os.Environ())
	}
	fmt.Fprintln(os.Stderr, "meltlock:", err)
	os.Exit(1)
}

// render makes one picture per output and returns them as output:path.
// Outputs that fail are left out and get swaylock's plain colour.
func render(dir string) []string {
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	now := time.Now()
	outs := outputs(ctx)
	images := make([]string, len(outs))
	var wg sync.WaitGroup
	for i, out := range outs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			path := filepath.Join(dir, out+".png")
			if err := renderOutput(ctx, out, path, now); err != nil {
				fmt.Fprintf(os.Stderr, "meltlock: %s: %v\n", out, err)
				return
			}
			images[i] = out + ":" + path
		}()
	}
	wg.Wait()
	var ok []string
	for _, i := range images {
		if i != "" {
			ok = append(ok, i)
		}
	}
	return ok
}

func outputs(ctx context.Context) []string {
	raw, err := exec.CommandContext(ctx, "swaymsg", "-t", "get_outputs", "-r").Output()
	if err != nil {
		return nil
	}
	var outs []struct {
		Name   string `json:"name"`
		Active bool   `json:"active"`
	}
	json.Unmarshal(raw, &outs)
	var names []string
	for _, o := range outs {
		if o.Active {
			names = append(names, o.Name)
		}
	}
	return names
}

func renderOutput(ctx context.Context, out, path string, now time.Time) error {
	shot, err := exec.CommandContext(ctx, "grim", "-o", out, "-t", "ppm", "-").Output()
	if err != nil {
		return err
	}
	pix, w, h, err := readPPM(bytes.NewReader(shot))
	if err != nil {
		return err
	}
	g := sample(pix, w, h)
	g.melt(rand.New(rand.NewSource(now.UnixNano() + int64(len(out)))))
	g.stamp("LOCKED SINCE", now.Format("15:04"))

	tmp := path + ".tmp"
	f, err := os.OpenFile(tmp, os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	enc := png.Encoder{CompressionLevel: png.BestSpeed}
	if err := enc.Encode(f, g.draw(w, h)); err != nil {
		f.Close()
		return err
	}
	if err := f.Close(); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}
