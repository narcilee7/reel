package protocol

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"io"
	"time"

	"github.com/narcilee7/reel/pkg/reel/detector"
)

// kittyChunkSize is the base64 payload size per APC chunk; Kitty reassembles
// chunks flagged with m=1 and terminates on m=0.
const kittyChunkSize = 4096

type kittyProtocol struct {
	animation bool
}

func (k *kittyProtocol) Name() string { return "kitty" }

// EnableAnimation implements AnimationCapable: it gates the terminal-driven
// animation path (kitty >= 0.20). When false, animations degrade to their
// first frame via StaticOf.
func (k *kittyProtocol) EnableAnimation(on bool) { k.animation = on }

func (k *kittyProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	if env.Get("KITTY_WINDOW_ID") != "" || env.Get("TERM") == "xterm-kitty" {
		return detector.SupportNative, nil
	}
	return detector.SupportNone, nil
}

func (k *kittyProtocol) Capabilities() Capabilities {
	return Capabilities{Animation: true, Transparency: true}
}

func (k *kittyProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	imageIndex := 0
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *AnimationFragment:
			err = k.writeAnimation(w, t, opts, imageIndex)
			imageIndex++
		default:
			if img, ok := StaticOf(f); ok {
				err = k.writeImage(w, img, opts, imageIndex)
				imageIndex++
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// DeleteImages removes the images with the given placement ids (Kitty a=d).
func (k *kittyProtocol) DeleteImages(w io.Writer, ids ...uint32) error {
	for _, id := range ids {
		if _, err := fmt.Fprintf(w, "\x1b_Ga=d,d=i,i=%d\x1b\\", id); err != nil {
			return err
		}
	}
	return nil
}

// transmitChunks writes a base64 pixel payload in m-flagged chunks. head and
// cont build the per-chunk control string (without the ESC _G / ST
// wrappers) for the first and continuation chunks, given the m flag.
// Continuation chunks of animation frames must repeat a=f (spec requirement),
// which cont being an arbitrary builder accommodates.
func transmitChunks(w io.Writer, payload string, head, cont func(more int) string) error {
	first := true
	for len(payload) > 0 {
		n := kittyChunkSize
		if len(payload) < n {
			n = len(payload)
		}
		chunk := payload[:n]
		payload = payload[n:]
		more := 0
		if len(payload) > 0 {
			more = 1
		}
		ctl := cont
		if first {
			ctl = head
			first = false
		}
		if _, err := fmt.Fprintf(w, "\x1b_G%s;%s\x1b\\", ctl(more), chunk); err != nil {
			return err
		}
	}
	return nil
}

// rgbaPixels returns img as RGBA pixels along with its bounds.
func rgbaPixels(img image.Image) (*image.RGBA, image.Rectangle) {
	b := img.Bounds()
	if rgba, ok := img.(*image.RGBA); ok {
		return rgba, b
	}
	rgba := image.NewRGBA(b)
	draw.Draw(rgba, b, img, b.Min, draw.Src)
	return rgba, b
}

// frameRGBA composites fr onto an RGBA canvas of b's dimensions, so every
// animation frame has the same pixel geometry as the root frame.
func frameRGBA(fr image.Image, b image.Rectangle) *image.RGBA {
	if fr.Bounds().Eq(b) {
		if rgba, ok := fr.(*image.RGBA); ok {
			return rgba
		}
	}
	dst := image.NewRGBA(b)
	draw.Draw(dst, b, fr, fr.Bounds().Min, draw.Src)
	return dst
}

// writeImage transmits raw RGBA pixels (f=32) in base64 chunks to avoid a
// single escape sequence exceeding terminal input buffer limits.
//
// When the fragment carries a valid cell rectangle and the caller supplied a
// PlacementBase, the placement is addressable: c/r pin the display size to
// the layout rectangle and i assigns a placement id. Otherwise the legacy
// unaddressable path (raw pixels, i=0) is used.
func (k *kittyProtocol) writeImage(w io.Writer, f *ImageFragment, opts *RenderOptions, imageIndex int) error {
	b := f.Image.Bounds()
	if b.Empty() {
		return writeText(w, &TextFragment{Text: "[image: " + f.Alt + "]\n"})
	}
	rgba, b := rgbaPixels(f.Image)
	payload := base64.StdEncoding.EncodeToString(rgba.Pix)

	var placement string
	addressable := opts != nil && f.Rect.Width > 0 && f.Rect.Height > 0 && opts.PlacementBase > 0
	if addressable {
		placement = fmt.Sprintf(",c=%d,r=%d,i=%d", f.Rect.Width, f.Rect.Height, opts.PlacementBase+uint32(imageIndex))
	}

	err := transmitChunks(w, payload,
		func(more int) string {
			return fmt.Sprintf("f=32,s=%d,v=%d%s,m=%d", b.Dx(), b.Dy(), placement, more)
		},
		func(more int) string { return fmt.Sprintf("m=%d", more) },
	)
	if err != nil {
		return err
	}
	if addressable {
		// After placement the cursor has moved right by c cols and down by r
		// rows (one row below the image); return it to column zero so
		// following text starts on a fresh line under the image.
		_, err := fmt.Fprint(w, "\r")
		return err
	}
	return nil
}

// writeAnimation implements terminal-driven animation per the kitty graphics
// protocol: a normal root frame, then one a=f chunk sequence per frame with
// the gap in milliseconds via z, and finally a=a,s=3 to start autonomous
// looping playback. The whole animation occupies a single placement id, so
// DeleteImages on that id removes it entirely.
func (k *kittyProtocol) writeAnimation(w io.Writer, f *AnimationFragment, opts *RenderOptions, imageIndex int) error {
	if !k.animation {
		img, _ := StaticOf(f)
		return k.writeImage(w, img, opts, imageIndex)
	}
	b := f.Image.Bounds()
	if b.Empty() {
		return writeText(w, &TextFragment{Text: "[animation: " + f.Alt + "]\n"})
	}
	addressable := opts != nil && f.Rect.Width > 0 && f.Rect.Height > 0 && opts.PlacementBase > 0
	if !addressable {
		// Animation frames require an explicit image id.
		img, _ := StaticOf(f)
		return k.writeImage(w, img, opts, imageIndex)
	}
	id := opts.PlacementBase + uint32(imageIndex)

	root, b := rgbaPixels(f.Image)
	placement := fmt.Sprintf(",c=%d,r=%d,i=%d", f.Rect.Width, f.Rect.Height, id)
	if err := transmitChunks(w, base64.StdEncoding.EncodeToString(root.Pix),
		func(more int) string {
			return fmt.Sprintf("a=T,f=32,s=%d,v=%d%s,m=%d", b.Dx(), b.Dy(), placement, more)
		},
		func(more int) string { return fmt.Sprintf("m=%d", more) },
	); err != nil {
		return err
	}

	for i := 1; i < len(f.Frames); i++ {
		fr := frameRGBA(f.Frames[i], b)
		var delay time.Duration
		if i < len(f.Delay) {
			delay = f.Delay[i]
		}
		z := delay.Milliseconds()
		if z < 1 {
			z = 1
		}
		// Every chunk of a frame carries a=f (spec requirement).
		ctl := func(more int) string { return fmt.Sprintf("a=f,i=%d,z=%d,m=%d", id, z, more) }
		if err := transmitChunks(w, base64.StdEncoding.EncodeToString(fr.Pix), ctl, ctl); err != nil {
			return err
		}
	}

	if _, err := fmt.Fprintf(w, "\x1b_Ga=a,i=%d,s=3\x1b\\", id); err != nil {
		return err
	}
	_, err := fmt.Fprint(w, "\r")
	return err
}
