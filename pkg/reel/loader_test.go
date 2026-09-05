package reel

import (
	"bytes"
	"image"
	"image/color"
	"image/gif"
	"image/png"
	"os"
	"path/filepath"
	"testing"

	"github.com/narcilee7/reel/pkg/reel/content"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(path, data, 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}

func pngBytes(t *testing.T) []byte {
	t.Helper()
	var buf bytes.Buffer
	img := image.NewRGBA(image.Rect(0, 0, 2, 2))
	img.SetRGBA(0, 0, color.RGBA{1, 2, 3, 255})
	if err := png.Encode(&buf, img); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func gifBytes(t *testing.T, frames int) []byte {
	t.Helper()
	var buf bytes.Buffer
	pal := color.Palette{color.RGBA{0, 0, 0, 255}, color.RGBA{255, 255, 255, 255}}
	g := &gif.GIF{
		Image:     make([]*image.Paletted, frames),
		Delay:     make([]int, frames),
		LoopCount: 0,
	}
	for i := range g.Image {
		fr := image.NewPaletted(image.Rect(0, 0, 2, 2), pal)
		fr.SetColorIndex(0, 0, uint8(i%2))
		g.Image[i] = fr
		g.Delay[i] = 5
	}
	if err := gif.EncodeAll(&buf, g); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

func TestLoadSniffDispatch(t *testing.T) {
	cases := []struct {
		name string
		data []byte
		want ContentKind
	}{
		{"photo.png", pngBytes(t), ContentKindImage},
		{"photo.dat", pngBytes(t), ContentKindImage}, // lying extension: magic wins
		{"noext", pngBytes(t), ContentKindImage},     // no extension at all
		{"anim.gif", gifBytes(t, 3), ContentKindImage},
		{"doc.pdf", append([]byte("%PDF-1.4 fake"), 0), ContentKindPDF},
		{"doc.dat", append([]byte("%PDF-1.4 fake"), 0), ContentKindPDF},
	}
	for _, tc := range cases {
		c, err := Load(writeFile(t, tc.name, tc.data))
		if err != nil {
			t.Errorf("%s: %v", tc.name, err)
			continue
		}
		if c.Kind() != tc.want {
			t.Errorf("%s: kind = %v, want %v", tc.name, c.Kind(), tc.want)
		}
	}
}

func TestLoadExtensionFallback(t *testing.T) {
	// Truncated/empty head: extension decides.
	md := writeFile(t, "doc.md", []byte("# hi"))
	if c, err := Load(md); err != nil || c.Kind() != ContentKindMarkdown {
		t.Errorf("md fallback: %v %v", c, err)
	}
	emptyPNG := writeFile(t, "empty.png", nil)
	if _, err := Load(emptyPNG); err == nil {
		t.Error("empty .png: expected decode error from extension fallback")
	}
}

func TestLoadImageGIFMultiFrame(t *testing.T) {
	path := writeFile(t, "anim.gif", gifBytes(t, 3))
	c, err := LoadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	img, ok := c.(*content.Image)
	if !ok {
		t.Fatalf("content = %T", c)
	}
	if len(img.Frames) != 3 {
		t.Errorf("frames = %d, want 3", len(img.Frames))
	}
	if len(img.Delay) != 3 || img.Delay[0] != 50_000_000 { // 5 * 10ms
		t.Errorf("delays = %v", img.Delay)
	}
	if len(img.Source) == 0 || img.SourceMIME != "image/gif" {
		t.Error("raw GIF bytes not captured")
	}
	// ToIR must produce an AnimationFragment.
	ir, err := c.ToIR(Context{})
	if err != nil {
		t.Fatal(err)
	}
	if _, ok := ir.Fragments[0].(*AnimationFragment); !ok {
		t.Errorf("fragment = %T, want *AnimationFragment", ir.Fragments[0])
	}
}

func TestLoadImageGIFFrameGuard(t *testing.T) {
	// 300 frames must be subsampled to at most 256.
	path := writeFile(t, "big.gif", gifBytes(t, 300))
	c, err := LoadImage(path)
	if err != nil {
		t.Fatal(err)
	}
	img := c.(*content.Image)
	if len(img.Frames) > 256 {
		t.Errorf("frames = %d, want <= 256", len(img.Frames))
	}
}
