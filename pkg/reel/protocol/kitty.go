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
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			err = writeText(w, t)
		case *ImageFragment:
			err = k.writeImage(w, t)
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeImage transmits raw RGBA pixels (f=32) in base64 chunks to avoid a
// single escape sequence exceeding terminal input buffer limits.
func (k *kittyProtocol) writeImage(w io.Writer, f *ImageFragment) error {
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
			_, err := fmt.Fprintf(w, "\x1b_Gf=32,s=%d,v=%d,m=%d;%s\x1b\\", b.Dx(), b.Dy(), more, chunk)
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
	return nil
}
