package tui

import (
	"bytes"
	"image"
	"strings"
	"testing"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/narcilee7/reel/pkg/reel"
	"github.com/narcilee7/reel/pkg/reel/content"
)

func testPagerEngine() *reel.Engine {
	return reel.New(reel.WithTerminalProfile(&reel.TerminalProfile{
		Program:    "kitty",
		Protocols:  []reel.ProtocolHint{{Name: "kitty", Level: reel.SupportNative}},
		CellSize:   reel.Size{Width: 10, Height: 20},
		GridSize:   reel.Size{Width: 80, Height: 24},
		ColorDepth: 1 << 24,
		IsTTY:      true,
	}))
}

func solidImg() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 8, 8))
}

// tallImg renders as 8×8 cells at the 10×20 test cell size, so 20 blocks
// span 160 rows and the one-screen prep margin actually excludes blocks.
func tallImg() image.Image {
	return image.NewRGBA(image.Rect(0, 0, 80, 160))
}

// TestBlockLifecycle drives the block state machine: unprepared → prepared →
// live → (scroll out) deleted → (scroll back) live again.
func TestBlockLifecycle(t *testing.T) {
	engine := testPagerEngine()

	// 20 image blocks, each 8 rows tall once fitted (80×160px at 10×20
	// cells), so the document is 160 rows against a 9-row viewport.
	blocks := make([]reel.Content, 20)
	for i := range blocks {
		blocks[i] = reel.NewImageContent(tallImg(), "x")
	}
	p := NewPager(engine, blocks)
	var out bytes.Buffer
	p.Out = &out

	// Window: 10 rows -> viewport shows ~2 blocks (8px image -> 1 cell).
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 10})
	p.reconcile()

	count := func(pred func(*Block) bool) int {
		n := 0
		for i := range p.blocks {
			if pred(&p.blocks[i]) {
				n++
			}
		}
		return n
	}
	prepared := count(func(b *Block) bool { return b.prepared })
	live := count(func(b *Block) bool { return b.live })
	if prepared == 0 || live == 0 {
		t.Fatalf("after init: prepared=%d live=%d", prepared, live)
	}
	if prepared == len(p.blocks) {
		t.Errorf("prepared=%d, want only blocks within the 1-screen margin", prepared)
	}

	// Scroll to the bottom: early blocks leave the margin and are deleted.
	for i := 0; i < 10; i++ {
		p.Update(tea.KeyMsg{Type: tea.KeyPgDown})
	}
	if !strings.Contains(out.String(), "a=d,d=i") {
		t.Errorf("expected delete sequences after scrolling away, out=%.60q", out.String())
	}
	if p.blocks[0].live {
		t.Error("block 0 must not be live after scrolling past it")
	}
	if !p.blocks[0].prepared {
		t.Error("block 0 keeps its prepared state (layout info retained)")
	}
	h0 := p.blocks[0].height
	if h0 <= 0 {
		t.Errorf("block 0 height = %d, want retained", h0)
	}

	// Scroll back to the top: block 0 re-renders with fresh ids (no delete of
	// stale ids needed because deletion already happened).
	out.Reset()
	for i := 0; i < 10; i++ {
		p.Update(tea.KeyMsg{Type: tea.KeyPgUp})
	}
	if !p.blocks[0].live {
		t.Error("block 0 must be live again after scrolling back")
	}
	if strings.Contains(out.String(), "a=d,d=i,i=1\x1b\\") {
		t.Errorf("re-entering a deleted block must not re-delete its old ids: %.40q", out.String())
	}
}

// TestPagerLinks exercises link extraction and cursor movement.
func TestPagerLinks(t *testing.T) {
	engine := testPagerEngine()
	md := "# A\n\n[one](https://a.example) and [two](https://b.example)\n"
	blocks := []reel.Content{
		content.NewMarkdown("/tmp/x.md", []byte(md)),
	}
	p := NewPager(engine, blocks)
	if len(p.links) != 2 {
		t.Fatalf("links = %d, want 2", len(p.links))
	}
	if p.links[0].URL != "https://a.example" || p.links[0].Line != 2 {
		t.Errorf("link 0 = %+v", p.links[0])
	}
	if _, ok := p.CurrentLink(); ok {
		t.Error("no link selected initially")
	}
	p.moveLink(1)
	if url, ok := p.CurrentLink(); !ok || url != "https://a.example" {
		t.Errorf("current = %q %v", url, ok)
	}
	p.moveLink(-1) // wrap backwards
	if url, _ := p.CurrentLink(); url != "https://b.example" {
		t.Errorf("wrapped current = %q", url)
	}
}

func TestPagerViewSmoke(t *testing.T) {
	engine := testPagerEngine()
	blocks := []reel.Content{
		content.NewMarkdown("/tmp/x.md", []byte("# T\n\nhello **world**\n")),
	}
	p := NewPager(engine, blocks)
	p.Update(tea.WindowSizeMsg{Width: 40, Height: 12})
	v := p.View()
	if !strings.Contains(v, "hello") {
		t.Errorf("view missing content: %.80q", v)
	}
	if !strings.Contains(v, "q quit") {
		t.Errorf("view missing status bar: %.80q", v)
	}
}
