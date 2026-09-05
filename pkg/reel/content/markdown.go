package content

import (
	"image"
	_ "image/gif"  // register GIF decoding
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"os"
	"path/filepath"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Markdown is a Markdown document rendered to text and inline image fragments.
type Markdown struct {
	Path   string // source path, used to resolve relative image references
	Source []byte
}

// NewMarkdown returns a Markdown content value for the given source.
func NewMarkdown(path string, source []byte) *Markdown {
	return &Markdown{Path: path, Source: source}
}

func (m *Markdown) Kind() protocol.ContentKind { return protocol.ContentKindMarkdown }

// ToIR parses the document with goldmark and maps AST nodes to fragments:
// text becomes TextFragments, image references become ImageFragments whose
// files are decoded eagerly.
func (m *Markdown) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	gm := goldmark.New()
	doc := gm.Parser().Parse(text.NewReader(m.Source))

	ir := &protocol.IntermediateRep{}
	err := ast.Walk(doc, func(node ast.Node, entering bool) (ast.WalkStatus, error) {
		if !entering {
			return ast.WalkContinue, nil
		}
		switch n := node.(type) {
		case *ast.Text:
			appendText(ir, string(n.Segment.Value(m.Source)))
		case *ast.Image:
			ir.Fragments = append(ir.Fragments, m.imageFragment(n))
		case *ast.Paragraph, *ast.Heading, *ast.ListItem,
			*ast.CodeBlock, *ast.FencedCodeBlock, *ast.Blockquote:
			appendText(ir, "\n")
		}
		return ast.WalkContinue, nil
	})
	if err != nil {
		return nil, err
	}
	return ir, nil
}

// imageFragment decodes the image file an *ast.Image points at; a missing or
// undecodable file degrades to a styled placeholder text fragment.
func (m *Markdown) imageFragment(n *ast.Image) protocol.Fragment {
	dest := string(n.Destination)
	path := dest
	if m.Path != "" && !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(m.Path), dest)
	}
	f, err := os.Open(path)
	if err != nil {
		return &protocol.TextFragment{Text: "[image: " + dest + "]"}
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return &protocol.TextFragment{Text: "[image: " + dest + "]"}
	}
	return &protocol.ImageFragment{Image: img, Alt: filepath.Base(dest)}
}

// appendText merges consecutive text into the trailing TextFragment.
func appendText(ir *protocol.IntermediateRep, s string) {
	if s == "" {
		return
	}
	if len(ir.Fragments) > 0 {
		if t, ok := ir.Fragments[len(ir.Fragments)-1].(*protocol.TextFragment); ok {
			t.Text += s
			return
		}
	}
	ir.Fragments = append(ir.Fragments, &protocol.TextFragment{Text: s})
}
