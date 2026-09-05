package content

import (
	"context"
	"image"
	"image/png"
	"os"
	"strings"
	"sync"
	"testing"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// fakePDFEnv installs a fake pdftoppm: LookPath succeeds and runCmd writes a
// tiny PNG next to the output root (the last argument).
func fakePDFEnv(t *testing.T) {
	t.Helper()
	pdfCache = sync.Map{}
	origLook, origRun := lookPath, runCmd
	lookPath = func(string) (string, error) { return "/usr/bin/pdftoppm", nil }
	runCmd = func(ctx context.Context, name string, args ...string) error {
		out := args[len(args)-1]
		f, err := os.Create(out + ".png")
		if err != nil {
			return err
		}
		defer f.Close()
		return png.Encode(f, image.NewRGBA(image.Rect(0, 0, 2, 2)))
	}
	t.Cleanup(func() { lookPath, runCmd = origLook, origRun })
}

func TestRasterWidth(t *testing.T) {
	cases := []struct {
		ctx  protocol.Context
		want int
	}{
		{protocol.Context{}, 800},
		{protocol.Context{MaxCells: detector.Size{Width: 80, Height: 24}, CellSize: detector.Size{Width: 10, Height: 20}}, 800},
		{protocol.Context{MaxCells: detector.Size{Width: 400, Height: 24}, CellSize: detector.Size{Width: 10, Height: 20}}, 1600},
	}
	for _, tc := range cases {
		if got := rasterWidth(tc.ctx); got != tc.want {
			t.Errorf("rasterWidth(%+v) = %d, want %d", tc.ctx, got, tc.want)
		}
	}
}

func TestRasterizePDFCache(t *testing.T) {
	fakePDFEnv(t)
	img1, err := rasterizePDF("/nonexistent.pdf", 0, 800)
	if err != nil {
		t.Fatal(err)
	}
	// The second call must hit the cache even though the file does not exist.
	img2, err := rasterizePDF("/nonexistent.pdf", 0, 800)
	if err != nil {
		t.Fatal(err)
	}
	if img1 != img2 {
		t.Error("cache miss: expected the same cached image")
	}
	// A different width must miss the cache (and re-run the fake command).
	if _, err := rasterizePDF("/nonexistent.pdf", 0, 400); err != nil {
		t.Fatal(err)
	}
}

func TestRasterizePDFLookPathFailure(t *testing.T) {
	pdfCache = sync.Map{}
	origLook := lookPath
	lookPath = func(string) (string, error) { return "", os.ErrNotExist }
	defer func() { lookPath = origLook }()

	_, err := rasterizePDF("x.pdf", 0, 800)
	if err == nil {
		t.Fatal("expected error when pdftoppm is missing")
	}
	for _, want := range []string{"pdftoppm", "poppler", "brew install", "apt install"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q missing %q", err, want)
		}
	}
}

func TestPDFToIR(t *testing.T) {
	fakePDFEnv(t)
	p := &PDF{Path: "/nonexistent.pdf", Page: 2}
	ir, err := p.ToIR(protocol.Context{})
	if err != nil {
		t.Fatal(err)
	}
	img, ok := ir.Fragments[0].(*protocol.ImageFragment)
	if !ok {
		t.Fatalf("fragment = %T", ir.Fragments[0])
	}
	if img.Alt != "nonexistent.pdf (page 3)" {
		t.Errorf("alt = %q", img.Alt)
	}
	if p.Kind() != protocol.ContentKindPDF {
		t.Errorf("kind = %v", p.Kind())
	}
}
