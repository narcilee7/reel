package protocol

import (
	"io"
	"strings"

	"github.com/narcilee7/reel/pkg/reel/detector"
)

type sixelProtocol struct{}

func (p *sixelProtocol) Name() string { return "sixel" }

func (p *sixelProtocol) Detect(env *detector.Environment) (SupportLevel, error) {
	term := env.Get("TERM")
	if strings.Contains(term, "sixel") || term == "foot" || term == "mlterm" || term == "yaft" {
		return detector.SupportStatic, nil
	}
	return detector.SupportNone, nil
}

func (p *sixelProtocol) Capabilities() Capabilities {
	return Capabilities{}
}

// Write degrades to the AnsiArt half-block renderer until sixel encoding is
// implemented, so selection never dead-ends in an error.
func (p *sixelProtocol) Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error {
	return (&ansiArtProtocol{}).Write(w, ir, opts)
}
