package reel

import (
	"fmt"
	"image"
	_ "image/gif"  // register GIF decoding
	_ "image/jpeg" // register JPEG decoding
	_ "image/png"  // register PNG decoding
	"os"
	"path/filepath"
	"strings"

	"github.com/narcilee7/reel/pkg/reel/content"
)

// Load reads a file and dispatches on its extension to the matching loader.
func Load(path string) (Content, error) {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".png", ".jpg", ".jpeg", ".gif":
		return LoadImage(path)
	case ".md", ".markdown", ".mdown":
		return LoadMarkdown(path)
	}
	return nil, fmt.Errorf("reel: unsupported file type %q", path)
}

// LoadImage decodes an image file into an Image content value.
func LoadImage(path string) (Content, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("reel: decode image %s: %w", path, err)
	}
	return &content.Image{Image: img, Alt: filepath.Base(path)}, nil
}

// LoadMarkdown reads a Markdown file into a Markdown content value.
func LoadMarkdown(path string) (Content, error) {
	src, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return content.NewMarkdown(path, src), nil
}
