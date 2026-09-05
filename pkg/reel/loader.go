package reel

import (
	"bytes"
	"fmt"
	"image"
	"image/draw"
	"image/gif"
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/narcilee7/reel/pkg/reel/content"
)

// magic-number prefixes for content sniffing (Load tries these before
// falling back to the file extension).
var (
	pngMagic  = []byte("\x89PNG\r\n\x1a\n")
	jpegMagic = []byte("\xff\xd8\xff")
	gifMagic  = []byte("GIF8")
	pdfMagic  = []byte("%PDF")
)

// maxGIFFrames caps the number of animation frames kept: kitty animation
// transmits full pixels per frame, so unbounded frames would produce
// hundreds of megabytes of escape sequences. Common GIFs stay well under it.
const maxGIFFrames = 256

// Load reads a file and dispatches on magic numbers first, then on the file
// extension as a fallback (tolerating truncated file headers).
func Load(path string) (Content, error) {
	head := sniff(path)

	switch {
	case bytes.HasPrefix(head, pngMagic), bytes.HasPrefix(head, jpegMagic), bytes.HasPrefix(head, gifMagic):
		return LoadImage(path)
	case bytes.HasPrefix(head, pdfMagic):
		return LoadPDF(path)
	}
	switch strings.ToLower(filepath.Ext(path)) {
	case ".md", ".markdown", ".mdown":
		return LoadMarkdown(path)
	case ".png", ".jpg", ".jpeg", ".gif":
		return LoadImage(path)
	case ".pdf":
		return LoadPDF(path)
	}
	return nil, fmt.Errorf("reel: unsupported file type %q", path)
}

// sniff reads up to 512 bytes of the file's head; a missing or unreadable
// file yields a nil head so the extension fallback decides.
func sniff(path string) []byte {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	head := make([]byte, 512)
	n, _ := f.Read(head)
	return head[:n]
}

// LoadImage decodes an image file into an Image content value. GIF files are
// decoded with all frames; multi-frame GIFs become animated content carrying
// the original file bytes for protocols that embed native GIFs (iTerm2).
func LoadImage(path string) (Content, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	if bytes.HasPrefix(raw, gifMagic) {
		return loadGIF(path, raw)
	}
	img, _, err := image.Decode(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("reel: decode image %s: %w", path, err)
	}
	return &content.Image{Image: img, Alt: filepath.Base(path)}, nil
}

// loadGIF decodes a whole GIF file, normalizing frames onto the first
// frame's canvas and normalizing delays (GIF units are 10ms; zero delays are
// raised to the 20ms browser convention). More than maxGIFFrames frames are
// subsampled evenly.
func loadGIF(path string, raw []byte) (Content, error) {
	g, err := gif.DecodeAll(bytes.NewReader(raw))
	if err != nil {
		return nil, fmt.Errorf("reel: decode gif %s: %w", path, err)
	}
	if len(g.Image) == 0 {
		return nil, fmt.Errorf("reel: gif %s has no frames", path)
	}
	base := g.Image[0].Bounds()
	normalize := func(fr image.Image) *image.RGBA {
		dst := image.NewRGBA(base)
		draw.Draw(dst, fr.Bounds(), fr, fr.Bounds().Min, draw.Over)
		return dst
	}

	c := &content.Image{
		Image:      normalize(g.Image[0]),
		Alt:        filepath.Base(path),
		Source:     raw,
		SourceMIME: "image/gif",
	}
	if len(g.Image) > 1 {
		n := len(g.Image)
		subsample := 1
		for n > maxGIFFrames {
			n = (n + 1) / 2
			subsample *= 2
		}
		c.Frames = make([]image.Image, 0, n)
		c.Delay = make([]time.Duration, 0, n)
		for i := 0; i < len(g.Image); i += subsample {
			c.Frames = append(c.Frames, normalize(g.Image[i]))
			d := time.Duration(g.Delay[i]) * 10 * time.Millisecond
			if d < 20*time.Millisecond {
				d = 20 * time.Millisecond
			}
			c.Delay = append(c.Delay, d)
		}
		// GIF delay semantics: Delay[i] is the display duration of frame i;
		// keep Frames and Delay aligned after subsampling.
		if len(c.Delay) > len(c.Frames) {
			c.Delay = c.Delay[:len(c.Frames)]
		}
	}
	return c, nil
}

// NewImageContent wraps an in-memory image as Content (the image package's
// Content equivalent of LoadImage for images that do not come from a file).
func NewImageContent(img image.Image, alt string) Content {
	return &content.Image{Image: img, Alt: alt}
}

// LoadPDF returns a PDF content value for the given file. Rendering happens
// at Prepare time via the external pdftoppm command (poppler).
func LoadPDF(path string) (Content, error) {
	if _, err := os.Stat(path); err != nil {
		return nil, err
	}
	return &content.PDF{Path: path}, nil
}

// LoadMarkdown reads a Markdown file into a Markdown content value.
func LoadMarkdown(path string) (Content, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return content.NewMarkdown(path, src), nil
}
