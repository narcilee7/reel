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
	"time"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

// ContentKind classifies source content.
type ContentKind int

const (
	ContentKindImage ContentKind = iota
	ContentKindMarkdown
	ContentKindChart
	ContentKindPDF
)

func (k ContentKind) String() string {
	switch k {
	case ContentKindImage:
		return "image"
	case ContentKindMarkdown:
		return "markdown"
	case ContentKindChart:
		return "chart"
	case ContentKindPDF:
		return "pdf"
	default:
		return "unknown"
	}
}

// Context carries ambient per-render information into Content.ToIR.
type Context struct {
	// SourceDir is the directory of the source file, when there is one.
	SourceDir string
	// CellSize is the terminal cell in pixels; zero means unknown.
	CellSize detector.Size
	// MaxCells is the render area in cells (terminal grid or
	// MaxWidth/MaxHeight); zero means unknown.
	MaxCells detector.Size
	// ColorDepth is the terminal color depth (256 / 1<<24); zero means unknown.
	ColorDepth int
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

// AnimationFragment is a "moving picture": the protocol-agnostic animation
// intermediate representation. Everything a static view needs (first frame,
// alt, cell rectangle) is self-contained in this type and does not depend on
// ImageFragment.
type AnimationFragment struct {
	// Image is the first frame: the entry point for layout.Fit sizing,
	// capability degradation and any path without raw source bytes.
	Image image.Image
	Alt   string
	Rect  layout.CellRect

	// Frames holds all frames (Frames[0] always equals Image); Delay[i] is
	// the display duration of Frames[i]. An AnimationFragment with fewer
	// than 2 frames is invalid (the constructor guarantees this, see
	// content.Image.ToIR).
	Frames []image.Image
	Delay  []time.Duration

	// Source/SourceMIME are the optional original file bytes (e.g. a whole
	// GIF file). When non-empty the AnimationFragment implements RawSourcer
	// (see below) for protocols that can embed native files. An empty
	// Source does not degrade the animation itself; it only affects the
	// iterm2 path.
	Source     []byte
	SourceMIME string
}

// RawSource returns the original file bytes and MIME type, implementing the
// RawSourcer optional capability.
func (f *AnimationFragment) RawSource() ([]byte, string) { return f.Source, f.SourceMIME }

func (*TextFragment) fragmentTag()      {}
func (*ImageFragment) fragmentTag()     {}
func (*AnimationFragment) fragmentTag() {}

// StaticOf folds any image-class fragment into its static view: an
// ImageFragment is returned as-is; an AnimationFragment yields an
// ImageFragment view of its first frame. Protocol non-animation paths must
// go through it, so "not recognizing animation" is always equivalent to
// "rendering the first frame", never to "ignoring".
func StaticOf(f Fragment) (*ImageFragment, bool) {
	switch t := f.(type) {
	case *ImageFragment:
		return t, true
	case *AnimationFragment:
		return &ImageFragment{Image: t.Image, Alt: t.Alt, Rect: t.Rect}, true
	}
	return nil, false
}

// RawSourcer is an optional Fragment capability: it provides the original
// file bytes and MIME type for protocols that can embed native-format files
// (iTerm2 embedding animated GIFs). Assert, don't require — when a Fragment
// does not implement it, protocols fall back to pixel frames.
type RawSourcer interface {
	RawSource() (data []byte, mime string)
}

// AnimationCapable is an optional Protocol capability (asserted like
// ImageDeleter): it declares whether this instance may safely use
// terminal-driven animation. The Engine calls it after Select, based on the
// profile hint level; when not implemented, or set false, the protocol
// layer degrades to the StaticOf first frame.
type AnimationCapable interface{ EnableAnimation(bool) }

// RenderOptions controls how a protocol renders an IR.
type RenderOptions struct {
	MaxWidth  int // max image width in terminal cells; zero = no limit
	MaxHeight int // max image height in terminal cells; zero = no limit

	// CellSize is the terminal cell in pixels (from detector L4). Zero means
	// unknown; protocols fall back to their Phase 1 assumptions.
	CellSize detector.Size

	// PlacementBase is the first placement id this Write call may use. Kitty
	// assigns ids PlacementBase, PlacementBase+1, ... per image, so the caller
	// can later delete them. Zero means ids are not addressable (the protocol
	// picks its own, e.g. i=0).
	PlacementBase uint32
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

// Registry is an ordered set of protocol factories, best first. Factories
// (rather than shared protocol values) let each Engine hold protocol
// instances with independent mutable state such as placement id counters.
type Registry struct {
	factories []func() Protocol
}

// DefaultRegistry returns the built-in protocol registry, ordered by
// preference: Kitty, iTerm2, Sixel, AnsiArt, Plain.
func DefaultRegistry() *Registry {
	return &Registry{factories: []func() Protocol{
		func() Protocol { return &kittyProtocol{} },
		func() Protocol { return &iterm2Protocol{} },
		func() Protocol { return &sixelProtocol{} },
		func() Protocol { return &ansiArtProtocol{} },
		func() Protocol { return &plainProtocol{} },
	}}
}

// Select returns the highest-priority protocol hinted by the profile,
// falling back to Plain when nothing is supported.
func (r *Registry) Select(profile *detector.TerminalProfile) Protocol {
	for _, f := range r.factories {
		p := f()
		if hint, ok := profile.Supports(p.Name()); ok && hint.Level >= detector.SupportStatic {
			return p
		}
	}
	return &plainProtocol{}
}

// ImageDeleter is an optional capability implemented by protocols that can
// remove previously placed images (Kitty a=d). Assert, don't require.
type ImageDeleter interface {
	// DeleteImages removes the images with the given placement ids.
	DeleteImages(w io.Writer, ids ...uint32) error
}
