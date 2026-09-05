package tui

import (
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"

	"github.com/narcilee7/reel/pkg/reel"
	"github.com/narcilee7/reel/pkg/reel/content"
)

// Link is a hyperlink inside a Pager document.
type Link struct {
	URL   string
	Block int // block index
	Line  int // 0-based line within the block's source
}

// Block is one independently renderable slice of a Pager document (usually
// a top-level section). Blocks are the unit of lazy preparation and image
// deletion: only blocks near the viewport hold prepared Documents and live
// terminal images.
type Block struct {
	src      reel.Content
	doc      *reel.Document
	height   int // visual rows, known once prepared
	prepared bool
	live     bool // images currently placed in the terminal
	lines    []string
	err      error
}

// Pager is a scrolling long-document component. The document is split into
// blocks; entering the viewport (plus one screen of margin) prepares a block
// (its images are decoded and transmitted only then), and scrolling well past
// a block deletes its placed images, releasing the terminal side. The block's
// Document and height are retained, so re-entering only re-renders (fresh
// placement ids) without re-parsing.
//
// The one-screen margin prevents prepare/delete thrashing on quick
// round-trip scrolling; deletion sequences go to Out (default os.Stdout).
//
// Like Engine and Document, a Pager is not safe for concurrent use; drive it
// from the single Bubble Tea model goroutine.
type Pager struct {
	engine *reel.Engine
	blocks []Block
	links  []Link
	cur    int // current link cursor, -1 = none

	vp viewport.Model

	width, height int
	yoffset       int
	atBottom      bool
	clampShifted  bool  // yoffset moved during the last clamp; re-run pass 2
	offsets       []int // cumulative block offsets in rows

	// Out receives image deletion sequences on scroll-out; it defaults to
	// os.Stdout.
	Out io.Writer
}

// NewPager returns a Pager over the given block contents (typically one
// reel.Content per top-level Markdown section). Links are extracted eagerly
// from Markdown sources for link navigation.
func NewPager(e *reel.Engine, blocks []reel.Content) *Pager {
	p := &Pager{
		engine: e,
		blocks: make([]Block, len(blocks)),
		vp:     viewport.New(0, 0),
		cur:    -1,
		Out:    os.Stdout,
	}
	for i, src := range blocks {
		p.blocks[i].src = src
		p.blocks[i].height = max(1, estimateHeight(src))
		if m, ok := src.(*content.Markdown); ok {
			for _, l := range content.ExtractLinks(m.Source) {
				p.links = append(p.links, Link{URL: l.URL, Block: i, Line: l.Line})
			}
		}
	}
	p.offsets = make([]int, len(blocks)+1)
	p.reconcile()
	return p
}

// Init implements tea.Model.
func (m *Pager) Init() tea.Cmd { return nil }

// Update implements tea.Model.
func (m *Pager) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		m.width, m.height = msg.Width, msg.Height
		m.vp.Width, m.vp.Height = msg.Width, max(1, msg.Height-1)
	case tea.KeyMsg:
		page := max(1, m.vp.Height-1)
		m.atBottom = false
		switch msg.String() {
		case "down", "j":
			m.yoffset++
		case "up", "k":
			m.yoffset--
		case "pgdown", "ctrl+f", " ":
			m.yoffset += page
		case "pgup", "ctrl+b":
			m.yoffset -= page
		case "g", "home":
			m.yoffset = 0
		case "G", "end":
			m.atBottom = true
		case "n":
			m.moveLink(1)
		case "p":
			m.moveLink(-1)
		}
	}
	m.reconcile()
	return m, nil
}

// View implements tea.Model: the viewport content plus a one-line status
// bar.
func (m *Pager) View() string {
	if m.height == 0 {
		return ""
	}
	return m.vp.View() + "\n" + m.statusBar()
}

// CurrentLink returns the URL under the link cursor, if any. The caller
// (cmd layer) decides how to open it.
func (m *Pager) CurrentLink() (string, bool) {
	if m.cur < 0 || m.cur >= len(m.links) {
		return "", false
	}
	return m.links[m.cur].URL, true
}

func (m *Pager) moveLink(delta int) {
	if len(m.links) == 0 {
		return
	}
	if m.cur < 0 {
		if delta > 0 {
			m.cur = 0
		} else {
			m.cur = len(m.links) - 1
		}
		return
	}
	m.cur = (m.cur + delta + len(m.links)) % len(m.links)
}

func (m *Pager) statusBar() string {
	var sb strings.Builder
	if m.cur >= 0 && m.cur < len(m.links) {
		l := m.links[m.cur]
		fmt.Fprintf(&sb, "link %d/%d  %s", m.cur+1, len(m.links), l.URL)
	} else if n := len(m.links); n > 0 {
		fmt.Fprintf(&sb, "%d links (n/p to move, enter to open)", n)
	}
	sb.WriteString("  j/k/space scroll, q quit")
	return sb.String()
}

// reconcile drives the block lifecycle against the current viewport: prepare
// blocks entering the one-screen margin, render blocks entering the
// viewport, delete images of blocks that left the margin.
//
// The Pager keeps its own yoffset: bubbles' viewport clamps scrolls against
// the content from the last SetContent, but the content grows as blocks go
// live, so reconciling must SetContent first and clamp afterwards.
func (m *Pager) reconcile() {
	if m.height == 0 {
		return
	}
	margin := m.vp.Height // one screen above and below

	// Pass 1: prepare blocks entering the margin. Heights become known here,
	// so offsets are recomputed before the view/delete pass.
	off := 0
	for i := range m.blocks {
		blk := &m.blocks[i]
		blkTop, blkBottom := off, off+blk.height
		off = blkBottom
		inPrep := blkBottom > m.yoffset-margin && blkTop < m.yoffset+margin+margin
		if !inPrep || blk.prepared {
			continue
		}
		doc, err := m.engine.Prepare(blk.src)
		blk.prepared = true
		if err != nil {
			blk.err = err
			blk.height = 1
		} else {
			blk.doc = doc
			blk.height = max(1, estimateHeight(blk.src))
		}
	}

	// Pass 2: render blocks entering the viewport, delete images of blocks
	// that left the margin.
	off = 0
	for i := range m.blocks {
		blk := &m.blocks[i]
		blkTop, blkBottom := off, off+blk.height
		off = blkBottom

		inView := blkBottom > m.yoffset && blkTop < m.yoffset+margin
		inPrep := blkBottom > m.yoffset-margin && blkTop < m.yoffset+margin+margin

		if !inPrep && blk.live && blk.doc != nil {
			m.engine.DeleteImages(m.out(), blk.doc.ImageIDs()...)
			blk.live = false
			blk.lines = nil
		}
		if inView && blk.prepared && !blk.live && blk.doc != nil {
			// The stream bytes are transmitted by Bubble Tea when it writes
			// the returned lines (the sequence rides on the first row), so
			// they are discarded here; only the line model is kept.
			lines, err := blk.doc.RenderLines(io.Discard)
			if err != nil {
				blk.err = err
				continue
			}
			blk.lines = lines
			blk.height = len(lines)
			blk.live = true
		}
	}

	// Recompute offsets with the new heights, publish the content, and clamp
	// the scroll position against the true height. A clamp shift can bring
	// new blocks into view, so re-run pass 2 once in that case.
	m.setContentAndClamp()
	if m.clampShifted {
		m.clampShifted = false
		off = 0
		for i := range m.blocks {
			blk := &m.blocks[i]
			blkTop, blkBottom := off, off+blk.height
			off = blkBottom
			inView := blkBottom > m.yoffset && blkTop < m.yoffset+margin
			inPrep := blkBottom > m.yoffset-margin && blkTop < m.yoffset+margin+margin
			if !inPrep && blk.live && blk.doc != nil {
				m.engine.DeleteImages(m.out(), blk.doc.ImageIDs()...)
				blk.live = false
				blk.lines = nil
			}
			if inView && blk.prepared && !blk.live && blk.doc != nil {
				lines, err := blk.doc.RenderLines(io.Discard)
				if err != nil {
					blk.err = err
					continue
				}
				blk.lines = lines
				blk.height = len(lines)
				blk.live = true
			}
		}
		m.setContentAndClamp()
	}
}

func (m *Pager) out() io.Writer {
	if m.Out != nil {
		return m.Out
	}
	return os.Stdout
}

// setContentAndClamp publishes the block lines to the viewport, rebuilds the
// offset table, and clamps yoffset (or pins it to the bottom) against the
// true content height.
func (m *Pager) setContentAndClamp() {
	var b strings.Builder
	off := 0
	for i := range m.blocks {
		blk := &m.blocks[i]
		m.offsets[i] = off
		off += blk.height
		if blk.live {
			b.WriteString(strings.Join(blk.lines, "\n"))
		} else {
			b.WriteString(strings.Repeat("\n", blk.height))
		}
		if i < len(m.blocks)-1 {
			b.WriteString("\n")
		}
	}
	m.offsets[len(m.blocks)] = off

	m.vp.SetContent(b.String())
	maxY := max(0, off-m.vp.Height)
	y := m.yoffset
	if m.atBottom {
		y = maxY
	}
	y = min(max(y, 0), maxY)
	if y != m.yoffset {
		m.clampShifted = true
	}
	m.yoffset = y
	m.vp.YOffset = y
}

// estimateHeight gives unprepared blocks a placeholder height so the
// scrollbar roughly reflects the document; Prepare corrects it once the
// block renders.
func estimateHeight(src reel.Content) int {
	if md, ok := src.(*content.Markdown); ok {
		return max(1, strings.Count(string(md.Source), "\n")+1)
	}
	return 1
}
