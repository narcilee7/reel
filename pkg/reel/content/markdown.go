package content

import (
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoding
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"os"
	"path/filepath"
	"strings"

	"github.com/yuin/goldmark"
	"github.com/yuin/goldmark/ast"
	"github.com/yuin/goldmark/text"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Markdown is a Markdown document rendered to styled text and inline image
// fragments.
type Markdown struct {
	Path   string // source path, used to resolve relative image references
	Source []byte
}

// NewMarkdown returns a Markdown content value for the given source.
func NewMarkdown(path string, source []byte) *Markdown {
	return &Markdown{Path: path, Source: source}
}

func (m *Markdown) Kind() protocol.ContentKind { return protocol.ContentKindMarkdown }

// mdTheme holds the SGR parameters of the default Markdown style mapping.
// Colors follow the xterm palette so truecolor and 256-color terminals share
// one scheme; code blocks use xterm 236 (#303030) on both.
type mdTheme struct {
	headingMajor string // Heading 1/2 foreground
	headingMinor string // Heading 3-6 foreground
	link         string // link foreground
	codeBG       string // code background
	quote        string // blockquote foreground
}

func themeForDepth(depth int) mdTheme {
	if depth > 256 {
		return mdTheme{
			headingMajor: "38;2;80;250;123",
			headingMinor: "38;2;139;233;253",
			link:         "38;2;98;114;164",
			codeBG:       "48;5;236",
			quote:        "38;5;245",
		}
	}
	return mdTheme{
		headingMajor: "38;5;119",
		headingMinor: "38;5;117",
		link:         "38;5;104",
		codeBG:       "48;5;236",
		quote:        "38;5;245",
	}
}

// mdRenderer walks the goldmark AST with an inline style stack: entering an
// inline node pushes a Style, exiting pops it; block nodes emit line breaks
// and indentation prefixes.
type mdRenderer struct {
	ir    *protocol.IntermediateRep
	src   []byte
	m     *Markdown
	theme mdTheme
	width int // ThematicBreak width in cells
	stack []protocol.Style
	quote int // blockquote nesting depth
	listN []int
}

// ToIR parses the document with goldmark and maps AST nodes to styled
// fragments; image references become ImageFragments whose files are decoded
// eagerly.
func (m *Markdown) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	gm := goldmark.New()
	doc := gm.Parser().Parse(text.NewReader(m.Source))

	width := ctx.MaxCells.Width
	if width <= 0 {
		width = 80
	}
	r := &mdRenderer{
		ir:    &protocol.IntermediateRep{},
		src:   m.Source,
		m:     m,
		theme: themeForDepth(ctx.ColorDepth),
		width: width,
	}
	if err := ast.Walk(doc, r.walk); err != nil {
		return nil, err
	}
	return r.ir, nil
}

func (r *mdRenderer) walk(node ast.Node, entering bool) (ast.WalkStatus, error) {
	switch n := node.(type) {
	case *ast.Text:
		if entering {
			r.text(string(n.Segment.Value(r.src)))
			if n.SoftLineBreak() {
				r.softbreak()
			}
		}
	case *ast.String:
		if entering {
			r.text(string(n.Value))
		}
	case *ast.CodeSpan:
		if entering {
			r.push(protocol.Style{BG: r.theme.codeBG})
		} else {
			r.pop()
		}
	case *ast.Emphasis:
		if entering {
			if n.Level == 2 {
				r.push(protocol.Style{Bold: true})
			} else {
				r.push(protocol.Style{Italic: true})
			}
		} else {
			r.pop()
		}
	case *ast.Link:
		if entering {
			r.push(protocol.Style{Underline: true, FG: r.theme.link})
		} else {
			r.text(" (" + string(n.Destination) + ")")
			r.pop()
		}
	case *ast.AutoLink:
		if entering {
			r.push(protocol.Style{Underline: true, FG: r.theme.link})
			r.text(string(n.URL(r.src)))
			r.pop()
		}
	case *ast.Image:
		if entering {
			r.blockOpen()
			r.ir.Fragments = append(r.ir.Fragments, r.m.imageFragment(n))
			r.linebreak()
			return ast.WalkSkipChildren, nil
		}
	case *ast.Heading:
		if entering {
			r.blockOpen()
			style := protocol.Style{Bold: true, FG: r.theme.headingMinor}
			if n.Level <= 2 {
				style.FG = r.theme.headingMajor
			}
			r.push(style)
		} else {
			r.pop()
			r.linebreak()
		}
	case *ast.Paragraph:
		if entering {
			r.blockOpen()
		} else {
			r.linebreak()
		}
	case *ast.List:
		if entering {
			start := 0
			if n.IsOrdered() {
				start = listStart(n) - 1
			}
			r.listN = append(r.listN, start)
		} else {
			r.listN = r.listN[:len(r.listN)-1]
			if len(r.listN) == 0 {
				r.linebreak()
			}
		}
	case *ast.ListItem:
		if entering {
			r.blockOpen()
			r.listN[len(r.listN)-1]++
			indent := strings.Repeat("  ", len(r.listN)-1)
			bullet := "• "
			if list, ok := n.Parent().(*ast.List); ok && list.IsOrdered() {
				bullet = fmt.Sprintf("%d. ", r.listN[len(r.listN)-1])
			}
			r.text(indent + "  " + bullet)
		} else {
			r.linebreak()
		}
	case *ast.Blockquote:
		if entering {
			r.blockOpen()
			r.quote++
			r.push(protocol.Style{FG: r.theme.quote})
			r.text(strings.Repeat("│ ", r.quote))
		} else {
			r.pop()
			r.quote--
			if r.quote == 0 {
				r.linebreak()
			}
		}
	case *ast.FencedCodeBlock, *ast.CodeBlock:
		if entering {
			r.blockOpen()
			r.push(protocol.Style{BG: r.theme.codeBG})
			lines := n.Lines()
			for i := 0; i < lines.Len(); i++ {
				seg := lines.At(i)
				r.text("  " + string(seg.Value(r.src)))
			}
		} else {
			r.pop()
		}
	case *ast.ThematicBreak:
		if entering {
			r.blockOpen()
			r.push(protocol.Style{FG: r.theme.quote})
			r.text(strings.Repeat("─", r.width))
			r.pop()
			r.linebreak()
		}
	case *ast.HTMLBlock, *ast.RawHTML:
		return ast.WalkSkipChildren, nil
	}
	return ast.WalkContinue, nil
}

// listStart returns the ordered list's start number, defaulting to 1.
func listStart(l *ast.List) int {
	if v, ok := l.AttributeString("start"); ok {
		if n, ok := v.(int); ok && n > 0 {
			return n
		}
	}
	return 1
}

// push merges s onto the style stack.
func (r *mdRenderer) push(s protocol.Style) { r.stack = append(r.stack, s) }

func (r *mdRenderer) pop() {
	if len(r.stack) > 0 {
		r.stack = r.stack[:len(r.stack)-1]
	}
}

// current merges the style stack into the effective style.
func (r *mdRenderer) current() protocol.Style {
	var s protocol.Style
	for _, st := range r.stack {
		s.Bold = s.Bold || st.Bold
		s.Italic = s.Italic || st.Italic
		s.Underline = s.Underline || st.Underline
		if st.FG != "" {
			s.FG = st.FG
		}
		if st.BG != "" {
			s.BG = st.BG
		}
	}
	return s
}

func (r *mdRenderer) text(s string) { appendText(r.ir, s, r.current()) }

// linebreak terminates the current line.
func (r *mdRenderer) linebreak() { appendText(r.ir, "\n", protocol.Style{}) }

// softbreak handles a soft line break inside a block, re-emitting the
// blockquote prefix so quoted lines stay decorated.
func (r *mdRenderer) softbreak() {
	if r.quote > 0 {
		r.text("\n" + strings.Repeat("│ ", r.quote))
		return
	}
	r.linebreak()
}

// blockOpen ensures the output is at the start of a fresh line before a
// block node opens.
func (r *mdRenderer) blockOpen() {
	if !atLineStart(r.ir) {
		r.linebreak()
	}
}

// atLineStart reports whether the IR's last fragment ends with a newline (or
// the IR is empty).
func atLineStart(ir *protocol.IntermediateRep) bool {
	if len(ir.Fragments) == 0 {
		return true
	}
	t, ok := ir.Fragments[len(ir.Fragments)-1].(*protocol.TextFragment)
	return ok && strings.HasSuffix(t.Text, "\n")
}

// imageFragment decodes the image file an *ast.Image points at; a missing or
// undecodable file degrades to a styled placeholder text fragment. The
// markdown alt text takes precedence as the image's alt; the file name is
// the fallback.
func (m *Markdown) imageFragment(n *ast.Image) protocol.Fragment {
	dest := string(n.Destination)
	alt := imageAltText(n, m.Source)
	if alt == "" {
		alt = filepath.Base(dest)
	}
	path := dest
	if m.Path != "" && !filepath.IsAbs(path) {
		path = filepath.Join(filepath.Dir(m.Path), dest)
	}
	f, err := os.Open(path)
	if err != nil {
		return &protocol.TextFragment{Text: "[image: " + alt + "]"}
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return &protocol.TextFragment{Text: "[image: " + alt + "]"}
	}
	return &protocol.ImageFragment{Image: img, Alt: alt}
}

// imageAltText collects the text of an image node's children, which is the
// alt text of ![alt](src).
func imageAltText(n ast.Node, src []byte) string {
	var sb strings.Builder
	for c := n.FirstChild(); c != nil; c = c.NextSibling() {
		switch t := c.(type) {
		case *ast.Text:
			seg := t.Segment
			sb.Write(seg.Value(src))
		case *ast.String:
			sb.Write(t.Value)
		}
	}
	return sb.String()
}

// appendText appends to the trailing TextFragment when its style matches,
// and starts a new fragment otherwise.
func appendText(ir *protocol.IntermediateRep, s string, style protocol.Style) {
	if s == "" {
		return
	}
	if len(ir.Fragments) > 0 {
		if t, ok := ir.Fragments[len(ir.Fragments)-1].(*protocol.TextFragment); ok && t.Style == style {
			t.Text += s
			return
		}
	}
	ir.Fragments = append(ir.Fragments, &protocol.TextFragment{Text: s, Style: style})
}
