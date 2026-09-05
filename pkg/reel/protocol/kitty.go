package protocol

import (
	"encoding/base64"
	"fmt"
	"image"
	"image/draw"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
)

// kittyChunkSize is the base64 payload size per APC chunk; Kitty reassembles
// chunks flagged with m=1 and terminates on m=0.
const kittyChunkSize = 4096

type kittyProtocol struct{}

func (k *kittyProtocol) Name() string { return "kitty" }

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
		case *ImageFragment:
			err = k.writeImage(w, t, opts, imageIndex)
			imageIndex++
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
	rgba, ok := f.Image.(*image.RGBA)
	if !ok {
		rgba = image.NewRGBA(b)
		draw.Draw(rgba, b, f.Image, b.Min, draw.Src)
	}
	payload := base64.StdEncoding.EncodeToString(rgba.Pix)

	var placement string
	addressable := opts != nil && f.Rect.Width > 0 && f.Rect.Height > 0 && opts.PlacementBase > 0
	if addressable {
		placement = fmt.Sprintf(",c=%d,r=%d,i=%d", f.Rect.Width, f.Rect.Height, opts.PlacementBase+uint32(imageIndex))
	}

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
		if first {
			_, err := fmt.Fprintf(w, "\x1b_Gf=32,s=%d,v=%d%s,m=%d;%s\x1b\\", b.Dx(), b.Dy(), placement, more, chunk)
			if err != nil {
				return err
			}
			first = false
		} else {
			if _, err := fmt.Fprintf(w, "\x1b_Gm=%d;%s\x1b\\", more, chunk); err != nil {
				return err
			}
		}
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
