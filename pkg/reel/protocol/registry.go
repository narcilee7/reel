// Package protocol defines the image protocol interface, the
// protocol-agnostic intermediate representation (IR) it consumes, and the
// registry that routes a terminal profile to the best protocol.
//
// The IR types live in this package (rather than the root reel package) so
// that protocol implementations never import their consumer; the root package
// re-exports them as aliases.
package protocol

import (
	"image"
	"io"
	"strings"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

// ContentKind classifies source content.
type ContentKind int

const (
	ContentKindImage ContentKind = iota
	ContentKindMarkdown
	ContentKindChart
)

func (k ContentKind) String() string {
	switch k {
	case ContentKindImage:
		return "image"
	case ContentKindMarkdown:
		return "markdown"
	case ContentKindChart:
		return "chart"
	default:
		return "unknown"
	}
}

// Context carries ambient per-render information into Content.ToIR.
type Context struct {
	// SourceDir is the directory of the source file, when there is one.
	SourceDir string
}

// SupportLevel describes how fully a terminal supports an image protocol.
type SupportLevel = detector.SupportLevel

const (
	SupportNone      = detector.SupportNone
	SupportStatic    = detector.SupportStatic
	SupportAnimation = detector.SupportAnimation
	SupportNative    = detector.SupportNative
)

// IntermediateRep is the protocol-agnostic rendering intermediate
// representation: an ordered stream of fragments.
type IntermediateRep struct {
	Fragments []Fragment
}

// Fragment is a sealed interface implemented only by TextFragment and
// ImageFragment.
type Fragment interface {
	fragmentTag()
}

// Style holds ANSI text styling: attributes plus raw SGR color parameters
// (e.g. FG: "38;2;255;0;0").
type Style struct {
	Bold, Italic, Underline bool
	FG, BG                  string
}

// sequence returns the SGR escape sequence selecting the style.
func (s Style) sequence() string {
	var params []string
	if s.Bold {
		params = append(params, "1")
	}
	if s.Italic {
		params = append(params, "3")
	}
	if s.Underline {
		params = append(params, "4")
	}
	if s.FG != "" {
		params = append(params, s.FG)
	}
	if s.BG != "" {
		params = append(params, s.BG)
	}
	return "\x1b[" + strings.Join(params, ";") + "m"
}

// TextFragment is a run of styled text.
type TextFragment struct {
	Text  string
	Style Style
}

// ImageFragment is an inline image and its desired cell rectangle
// (zero means auto-fit during layout).
type ImageFragment struct {
	Image image.Image
	Alt   string
	Rect  layout.CellRect
}

func (*TextFragment) fragmentTag()  {}
func (*ImageFragment) fragmentTag() {}

// RenderOptions controls how a protocol renders an IR.
type RenderOptions struct {
	MaxWidth  int // max image width in terminal cells; zero = no limit
	MaxHeight int // max image height in terminal cells; zero = no limit
}

// Capabilities describes the upper limits of a protocol.
type Capabilities struct {
	Animation    bool
	Transparency bool
	MaxSizeBytes int // zero = no limit
}

// Protocol renders an IntermediateRep to a terminal escape sequence stream.
type Protocol interface {
	Name() string
	// Detect reports whether the environment supports this protocol.
	Detect(env *detector.Environment) (SupportLevel, error)
	// Write renders the IR to w as a stream of escape sequences.
	Write(w io.Writer, ir *IntermediateRep, opts *RenderOptions) error
	// Capabilities returns the protocol's upper limits.
	Capabilities() Capabilities
}

// Registry is an ordered set of protocols, best first.
type Registry struct {
	protocols []Protocol
}

// DefaultRegistry returns the built-in protocol registry, ordered by
// preference: Kitty, iTerm2, Sixel, AnsiArt, Plain.
func DefaultRegistry() *Registry {
	return &Registry{protocols: []Protocol{
		&kittyProtocol{},
		&iterm2Protocol{},
		&sixelProtocol{},
		&ansiArtProtocol{},
		&plainProtocol{},
	}}
}

// Select returns the highest-priority protocol hinted by the profile,
// falling back to Plain when nothing is supported.
func (r *Registry) Select(profile *detector.TerminalProfile) Protocol {
	for _, p := range r.protocols {
		if hint, ok := profile.Supports(p.Name()); ok && hint.Level >= detector.SupportStatic {
			return p
		}
	}
	return &plainProtocol{}
}
