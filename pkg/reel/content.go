// Package reel is the public SDK: construct an Engine with New, load content
// with Load (or LoadImage/LoadMarkdown), and render with Engine.Render.
package reel

import (
	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// ContentKind classifies source content.
type ContentKind = protocol.ContentKind

const (
	ContentKindImage    = protocol.ContentKindImage
	ContentKindMarkdown = protocol.ContentKindMarkdown
	ContentKindChart    = protocol.ContentKindChart
)

// Context carries ambient per-render information into Content.ToIR.
type Context = protocol.Context

// SupportLevel describes how fully a terminal supports an image protocol.
type SupportLevel = detector.SupportLevel

const (
	SupportNone      = detector.SupportNone
	SupportStatic    = detector.SupportStatic
	SupportAnimation = detector.SupportAnimation
	SupportNative    = detector.SupportNative
)

// IntermediateRep is the protocol-agnostic rendering intermediate
// representation, re-exported from the protocol package.
type IntermediateRep = protocol.IntermediateRep

// Fragment is a sealed interface implemented by TextFragment and ImageFragment.
type Fragment = protocol.Fragment

// TextFragment is a run of styled text.
type TextFragment = protocol.TextFragment

// ImageFragment is an inline image and its desired cell rectangle.
type ImageFragment = protocol.ImageFragment

// Style holds ANSI text styling.
type Style = protocol.Style

// Content is the source material rendered by Engine. It converts itself to
// a protocol-agnostic IntermediateRep.
type Content interface {
	Kind() ContentKind
	ToIR(ctx Context) (*IntermediateRep, error)
}
