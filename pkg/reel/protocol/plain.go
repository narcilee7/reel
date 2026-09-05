package protocol

import (
	"fmt"
	"io"

	"github.com/narcilee7/reel/pkg/reel/detector"
)

// plainProtocol is the ultimate fallback: text passes through and images
// become "[image: alt]" placeholders.
type plainProtocol struct{}

func (p *plainProtocol) Name() string { return "plain" }

func (p *plainProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	return detector.SupportStatic, nil
}

func (p *plainProtocol) Capabilities() Capabilities {
	return Capabilities{}
}

func (p *plainProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	for _, f := range ir.Fragments {
		var err error
		switch t := f.(type) {
		case *TextFragment:
			_, err = io.WriteString(w, t.Text)
		case *AnimationFragment:
			alt := t.Alt
			if alt == "" {
				alt = "animation"
			}
			_, err = fmt.Fprintf(w, "[animation: %s]\n", alt)
		default:
			if img, ok := StaticOf(f); ok {
				alt := img.Alt
				if alt == "" {
					alt = "image"
				}
				_, err = fmt.Fprintf(w, "[image: %s]\n", alt)
			}
		}
		if err != nil {
			return err
		}
	}
	return nil
}

// writeText renders a TextFragment, applying its style when non-zero.
func writeText(w io.Writer, f *TextFragment) error {
	if f.Text == "" {
		return nil
	}
	if f.Style == (Style{}) {
		_, err := io.WriteString(w, f.Text)
		return err
	}
	if _, err := io.WriteString(w, f.Style.sequence()); err != nil {
		return err
	}
	if _, err := io.WriteString(w, f.Text); err != nil {
		return err
	}
	_, err := io.WriteString(w, "\x1b[0m")
	return err
}
