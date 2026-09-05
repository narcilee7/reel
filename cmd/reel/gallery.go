package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"os"
	"path/filepath"
	"sort"
	"strings"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/narcilee7/reel/pkg/reel"
	"github.com/narcilee7/reel/pkg/reel/content"
	"github.com/narcilee7/reel/pkg/tui"
)

// Thumbnail slot geometry, in cells: each thumbnail occupies thumbCellsX ×
// thumbCellsY with a one-cell gap between slots.
const (
	thumbCellsX = 16
	thumbCellsY = 8
)

var imageExts = map[string]bool{
	".png": true, ".jpg": true, ".jpeg": true, ".gif": true,
	".bmp": true, ".webp": true,
}

// runGallery implements `reel gallery [dir]`: a keyboard-driven image
// browser. The grid page is a single composed image (one Document per page);
// every redraw deletes the previous page's placements before preparing the
// new one — delete-before-draw is the invariant that keeps the terminal's
// image table from growing without bound.
func runGallery(args []string) error {
	fs := flag.NewFlagSet("gallery", flag.ContinueOnError)
	if err := fs.Parse(args); err != nil {
		return err
	}
	dir := "."
	if fs.NArg() > 1 {
		return fmt.Errorf("usage: reel gallery [dir]")
	}
	if fs.NArg() == 1 {
		dir = fs.Arg(0)
	}

	names, err := scanImages(dir)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		return fmt.Errorf("%s: no images found", dir)
	}

	engine := reel.New(reel.WithAutoDetect())
	g := &gallery{
		engine: engine,
		dir:    dir,
		names:  names,
		thumbs: make(map[string]image.Image),
	}
	_, err = tea.NewProgram(g, tea.WithAltScreen()).Run()
	return err
}

func scanImages(dir string) ([]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, err
	}
	var names []string
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if imageExts[strings.ToLower(filepath.Ext(e.Name()))] {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)
	return names, nil
}

// gallery is the Bubble Tea model for `reel gallery`.
type gallery struct {
	engine *reel.Engine
	dir    string
	names  []string // sorted image file names

	winW, winH int
	slotsX     int // thumbnails per row
	slotsY     int // thumbnail rows per page
	perPage    int

	cursor int // index into names
	page   int

	thumbs                   map[string]image.Image // name → scaled thumbnail (cache)
	doc                      *reel.Document         // current grid page
	seq                      string                 // rendered page sequences (valid for version)
	version, renderedVersion int

	viewing bool
	viewer  *tui.Image
}

func (g *gallery) Init() tea.Cmd { return nil }

func (g *gallery) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	if g.viewing {
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "esc" {
			g.viewing = false
			if g.viewer != nil {
				_ = g.viewer.Close()
				g.viewer = nil
			}
			return g, nil
		}
		if key, ok := msg.(tea.KeyMsg); ok && key.String() == "q" {
			return g, tea.Quit
		}
		m, cmd := g.viewer.Update(msg)
		g.viewer = m.(*tui.Image)
		return g, cmd
	}

	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		g.winW, g.winH = msg.Width, msg.Height
		_ = g.engine.Reprobe()
		g.layout()
		g.rebuildPage()
	case tea.KeyMsg:
		switch msg.String() {
		case "q", "ctrl+c":
			return g, tea.Quit
		case "right", "l":
			g.move(1)
		case "left", "h":
			g.move(-1)
		case "down", "j":
			g.move(g.slotsX)
		case "up", "k":
			g.move(-g.slotsX)
		case "n", "pgdown":
			g.flipPage(1)
		case "p", "pgup":
			g.flipPage(-1)
		case "enter":
			return g, g.openViewer()
		}
	}
	return g, nil
}

// layout derives the slot grid from the current window size.
func (g *gallery) layout() {
	g.slotsX = max(1, (g.winW+1)/(thumbCellsX+1))
	g.slotsY = max(1, g.winH/(thumbCellsY+1))
	g.perPage = g.slotsX * g.slotsY
}

func (g *gallery) move(delta int) {
	n := len(g.names)
	if n == 0 {
		return
	}
	g.cursor = (g.cursor + delta + n) % n
	g.page = g.cursor / g.perPage
	g.rebuildPage()
}

func (g *gallery) flipPage(delta int) {
	pages := max(1, (len(g.names)+g.perPage-1)/g.perPage)
	g.page = (g.page + delta + pages) % pages
	g.cursor = min(g.page*g.perPage, len(g.names)-1)
	g.rebuildPage()
}

// thumbnail returns the cached thumbnail for name, loading and scaling on
// first use. Animated files contribute their first frame (StaticOf
// semantics; the full view restores animation via the protocol layer).
func (g *gallery) thumbnail(name string) image.Image {
	if t, ok := g.thumbs[name]; ok {
		return t
	}
	c, err := reel.LoadImage(filepath.Join(g.dir, name))
	if err != nil {
		return nil
	}
	img, ok := firstFrame(c)
	if !ok {
		return nil
	}
	grid := g.engine.Grid()
	t := tui.Thumbnail(img, grid.CellWidth, grid.CellHeight, thumbCellsX, thumbCellsY)
	g.thumbs[name] = t
	return t
}

// rebuildPage composes the current page of thumbnails into one image,
// deletes the previous page's placements and prepares the new page
// Document. Delete-before-draw on every redraw.
func (g *gallery) rebuildPage() {
	if g.winW == 0 || g.perPage == 0 {
		return
	}
	if g.doc != nil {
		_ = g.engine.DeleteImages(os.Stdout, g.doc.ImageIDs()...)
		g.doc = nil
	}
	grid := g.engine.Grid()
	pitchX := (thumbCellsX + 1) * grid.CellWidth
	pitchY := (thumbCellsY + 1) * grid.CellHeight
	page := image.NewRGBA(image.Rect(0, 0, g.slotsX*pitchX, g.slotsY*pitchY))
	for slot := 0; slot < g.perPage; slot++ {
		idx := g.page*g.perPage + slot
		if idx >= len(g.names) {
			break
		}
		thumb := g.thumbnail(g.names[idx])
		if thumb == nil {
			continue
		}
		col, row := slot%g.slotsX, slot/g.slotsX
		pt := image.Pt(col*pitchX, row*pitchY)
		draw.Draw(page, thumb.Bounds().Add(pt), thumb, thumb.Bounds().Min, draw.Src)
		if idx == g.cursor {
			drawBorder(page, thumb.Bounds().Add(pt), color.RGBA{0x8a, 0xd5, 0xff, 0xff})
		}
	}
	doc, err := g.engine.Prepare(reel.NewImageContent(page, "gallery"))
	if err != nil {
		return
	}
	g.doc = doc
	g.version++
}

func (g *gallery) openViewer() tea.Cmd {
	c, err := reel.LoadImage(filepath.Join(g.dir, g.names[g.cursor]))
	if err != nil {
		return nil
	}
	v, err := tui.NewImage(g.engine, c)
	if err != nil {
		return nil
	}
	// The grid page stays placed underneath; the viewer draws over it and
	// Close() releases only its own placements on exit.
	g.viewer = v
	g.viewing = true
	return nil
}

func (g *gallery) View() string {
	if g.viewing && g.viewer != nil {
		return g.viewer.View()
	}
	if g.winW == 0 {
		return ""
	}
	var b strings.Builder
	if g.doc != nil {
		if g.renderedVersion != g.version {
			var buf strings.Builder
			if err := g.doc.Render(&buf); err == nil {
				g.seq = buf.String()
				g.renderedVersion = g.version
			}
		}
		b.WriteString(g.seq)
		pageRows := (g.slotsY*(thumbCellsY+1) - 1)
		for i := pageRows; i < g.winH-1; i++ {
			b.WriteString("\n")
		}
	}
	b.WriteString(g.statusBar())
	return b.String()
}

func (g *gallery) statusBar() string {
	total := len(g.names)
	pages := max(1, (total+g.perPage-1)/g.perPage)
	return fmt.Sprintf("%s  %d/%d  page %d/%d  h/j/k/l move  n/p page  enter view  q quit",
		g.names[g.cursor], g.cursor+1, total, g.page+1, pages)
}

func drawBorder(dst *image.RGBA, r image.Rectangle, c color.Color) {
	for x := r.Min.X; x < r.Max.X; x++ {
		dst.Set(x, r.Min.Y, c)
		dst.Set(x, r.Max.Y-1, c)
	}
	for y := r.Min.Y; y < r.Max.Y; y++ {
		dst.Set(r.Min.X, y, c)
		dst.Set(r.Max.X-1, y, c)
	}
}

// firstFrame extracts the still image from a loaded image Content: the
// first frame of an AnimationFragment, or the image of an ImageFragment.
func firstFrame(c reel.Content) (image.Image, bool) {
	switch f := c.(type) {
	case *content.Image:
		return f.Image, true
	default:
		return nil, false
	}
}
