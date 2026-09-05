package tui

import (
	"bytes"
	"os"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"

	"github.com/narcilee7/reel/pkg/reel"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

// scrollResetDelay is the debounce interval between the last scroll key and
// restoring hidden images in snapshot mode.
const scrollResetDelay = 150 * time.Millisecond

// showImagesMsg restores images hidden by scrolling.
type showImagesMsg struct{}

// Image is an inline image component for Bubble Tea models. It renders a
// prepared reel.Document and occupies a fixed cell rectangle; the actual
// image escape sequences ride on the first placeholder line so Bubble Tea's
// line diffing drives (re)transmission.
//
// The View output is stable for a given version: the sequences are rendered
// once per version change, and a version bump (new content, resize, or
// restore after scrolling) yields a new string — and for Kitty new placement
// ids — triggering a redraw.
type Image struct {
	engine  *reel.Engine
	doc     *reel.Document
	version int
	rect    layout.CellRect
	managed bool // kitty: images scroll with the grid, never hidden
	hidden  bool // snapshot protocols: hidden while scrolling

	renderedVersion int
	seq             string
}

// NewImage prepares content on the engine and returns an inline image
// component fitted to the engine's current grid.
func NewImage(e *reel.Engine, c reel.Content) (*Image, error) {
	doc, err := e.Prepare(c)
	if err != nil {
		return nil, err
	}
	return &Image{
		engine:  e,
		doc:     doc,
		rect:    doc.ImageRect(),
		managed: e.ProtocolName() == "kitty",
	}, nil
}

// Init implements tea.Model.
func (m *Image) Init() tea.Cmd { return nil }

// Update implements tea.Model. WindowSizeMsg triggers the resize loop
// (reprobe, refit, delete old placements); scroll keys debounce image
// visibility in snapshot mode.
func (m *Image) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		_ = m.engine.Reprobe()
		m.doc.ReFit(m.engine.Grid())
		m.engine.DeleteImages(os.Stdout, m.doc.ImageIDs()...)
		m.version++
		m.hidden = false
		return m, nil
	case showImagesMsg:
		if m.hidden {
			m.hidden = false
			m.version++
		}
		return m, nil
	case tea.KeyMsg:
		if !m.managed && isScrollKey(msg) {
			m.hidden = true
			return m, tea.Tick(scrollResetDelay, func(time.Time) tea.Msg {
				return showImagesMsg{}
			})
		}
		return m, nil
	}
	return m, nil
}

// View implements tea.Model. It returns the image's escape sequences on the
// first line plus blank placeholder lines up to the image's cell height.
func (m *Image) View() string {
	if m.hidden || m.rect.Height <= 0 {
		return ""
	}
	if m.renderedVersion != m.version || m.seq == "" {
		var buf bytes.Buffer
		if err := m.doc.Render(&buf); err != nil {
			return ""
		}
		m.seq = buf.String()
		m.renderedVersion = m.version
	}
	if m.rect.Height == 1 {
		return m.seq
	}
	return m.seq + strings.Repeat("\n", m.rect.Height-1)
}

// Close deletes the component's placed images from the terminal, releasing
// the placements a viewer holds before returning to a previous screen.
func (m *Image) Close() error {
	return m.engine.DeleteImages(os.Stdout, m.doc.ImageIDs()...)
}

// isScrollKey reports whether the key message scrolls content.
func isScrollKey(msg tea.KeyMsg) bool {
	switch msg.String() {
	case "up", "down", "pgup", "pgdown", "ctrl+u", "ctrl+d", "j", "k", "g", "G", " ":
		return true
	}
	return false
}
