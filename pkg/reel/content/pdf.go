package content

import (
	"context"
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoding
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// PDF is a preview of a single PDF page, rasterized on demand via the
// external pdftoppm command (poppler).
type PDF struct {
	Path string
	Page int // 0-based; defaults to 0
}

func (p *PDF) Kind() protocol.ContentKind { return protocol.ContentKindPDF }

// pdfMaxWidth caps the rasterization width: beyond 1600px the base64
// transfer volume of pixel protocols dominates while terminals only display
// a few hundred cells of width anyway.
const pdfMaxWidth = 1600

// pdfRasterTimeout bounds one pdftoppm invocation.
const pdfRasterTimeout = 10 * time.Second

// pdfCacheKey identifies one rasterization result in the process-wide cache.
type pdfCacheKey struct {
	path    string
	page    int
	targetW int
}

// pdfCache deduplicates rasterizations across repeated Prepare calls for the
// same (path, page, target width). File change detection is the caller's job
// (rebuild the Content).
var pdfCache sync.Map // pdfCacheKey → image.Image

// lookPath and runCmd are indirections so tests can inject fakes.
var lookPath = exec.LookPath

var runCmd = func(ctx context.Context, name string, args ...string) error {
	cmd := exec.CommandContext(ctx, name, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("pdftoppm: %w: %s", err, out)
	}
	return nil
}

// ToIR rasterizes the requested page and reuses the plain Image IR path.
func (p *PDF) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	targetW := rasterWidth(ctx)
	img, err := rasterizePDF(p.Path, p.Page, targetW)
	if err != nil {
		return nil, err
	}
	page := p.Page
	if page < 0 {
		page = 0
	}
	alt := fmt.Sprintf("%s (page %d)", filepath.Base(p.Path), page+1)
	return (&Image{Image: img, Alt: alt}).ToIR(ctx)
}

// rasterWidth computes the rasterization target width from the render area:
// MaxCells × CellSize pixels, capped at pdfMaxWidth.
func rasterWidth(ctx protocol.Context) int {
	w := ctx.MaxCells.Width * ctx.CellSize.Width
	if w <= 0 {
		w = 800
	}
	if w > pdfMaxWidth {
		w = pdfMaxWidth
	}
	return w
}

// rasterizePDF rasterizes one page of path to PNG via pdftoppm, caching the
// decoded image per (path, page, width).
func rasterizePDF(path string, page, targetW int) (image.Image, error) {
	if page < 0 {
		page = 0
	}
	key := pdfCacheKey{path: path, page: page, targetW: targetW}
	if v, ok := pdfCache.Load(key); ok {
		return v.(image.Image), nil
	}

	ppm, err := lookPath("pdftoppm")
	if err != nil {
		return nil, fmt.Errorf("reel: pdftoppm not found; install poppler "+
			"(brew install poppler / apt install poppler-utils): %w", err)
	}

	dir, err := os.MkdirTemp("", "reel-pdf-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	out := filepath.Join(dir, "page")

	ctx, cancel := context.WithTimeout(context.Background(), pdfRasterTimeout)
	defer cancel()
	// -f/-l select the page (1-based), -singlefile writes <out>.png,
	// -scale-to-x lets poppler compute the proportional height.
	if err := runCmd(ctx, ppm,
		"-f", strconv.Itoa(page+1), "-l", strconv.Itoa(page+1),
		"-png", "-singlefile", "-scale-to-x", strconv.Itoa(targetW),
		path, out,
	); err != nil {
		return nil, fmt.Errorf("reel: rasterize %s page %d: %w", path, page, err)
	}

	f, err := os.Open(out + ".png")
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reel: decode pdftoppm output: %w", err)
	}
	pdfCache.Store(key, img)
	return img, nil
}
