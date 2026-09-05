package protocol

import (
	"bytes"
	"image"
	"image/color"
	"strings"
	"testing"
	"time"

	"github.com/narcilee7/reel/pkg/reel/detector"
	"github.com/narcilee7/reel/pkg/reel/layout"
)

func solidFrame(w, h int, c color.RGBA) *image.RGBA {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	return img
}

func testAnimationFragment(source []byte) *AnimationFragment {
	f0 := solidFrame(2, 2, color.RGBA{255, 0, 0, 255})
	return &AnimationFragment{
		Image:      f0,
		Alt:        "anim",
		Rect:       layout.CellRect{Width: 1, Height: 1},
		Frames:     []image.Image{f0, solidFrame(2, 2, color.RGBA{0, 255, 0, 255}), solidFrame(2, 2, color.RGBA{0, 0, 255, 255})},
		Delay:      []time.Duration{50 * time.Millisecond, 60 * time.Millisecond, 70 * time.Millisecond},
		Source:     source,
		SourceMIME: "image/gif",
	}
}

func TestStaticOf(t *testing.T) {
	img := &ImageFragment{Image: solidFrame(1, 1, color.RGBA{}), Alt: "a", Rect: layout.CellRect{Width: 3, Height: 2}}
	if got, ok := StaticOf(img); !ok || got != img {
		t.Errorf("StaticOf(ImageFragment) = %v, %v; want same pointer", got, ok)
	}
	anim := testAnimationFragment(nil)
	got, ok := StaticOf(anim)
	if !ok {
		t.Fatal("StaticOf(AnimationFragment) not ok")
	}
	if got.Image != anim.Image || got.Alt != anim.Alt || got.Rect != anim.Rect {
		t.Errorf("StaticOf view = %+v, want first frame/alt/rect", got)
	}
	if _, ok := StaticOf(&TextFragment{Text: "x"}); ok {
		t.Error("StaticOf(TextFragment) should not be ok")
	}
}

func TestKittyAnimationSequence(t *testing.T) {
	k := &kittyProtocol{}
	k.EnableAnimation(true)
	anim := testAnimationFragment(nil)
	ir := &IntermediateRep{Fragments: []Fragment{anim}}
	opts := &RenderOptions{PlacementBase: 5, CellSize: detector.Size{Width: 10, Height: 20}}

	var buf bytes.Buffer
	if err := k.Write(&buf, ir, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()

	// Root frame carries a=T and the placement id.
	if !strings.Contains(out, "\x1b_Ga=T,f=32,s=2,v=2,c=1,r=1,i=5,m=0;") {
		t.Errorf("missing root frame control: %.60q", out)
	}
	// Two appended frames, each a=f with the shared id and z in ms.
	if got := strings.Count(out, "a=f,i=5,z="); got != 2 {
		t.Errorf("a=f frame count = %d, want 2", got)
	}
	if !strings.Contains(out, "z=60") || !strings.Contains(out, "z=70") {
		t.Errorf("frame gaps missing: %q", out)
	}
	// Autonomous looping start.
	if !strings.Contains(out, "\x1b_Ga=a,i=5,s=3\x1b\\") {
		t.Errorf("missing animation start: %q", out)
	}
	// Single placement id for the whole animation.
	if strings.Contains(out, "i=6") {
		t.Errorf("animation must occupy a single id: %q", out)
	}
}

func TestKittyAnimationDisabledDegradesToFirstFrame(t *testing.T) {
	k := &kittyProtocol{} // animation off
	anim := testAnimationFragment(nil)
	opts := &RenderOptions{PlacementBase: 1, CellSize: detector.Size{Width: 10, Height: 20}}
	var buf bytes.Buffer
	if err := k.Write(&buf, &IntermediateRep{Fragments: []Fragment{anim}}, opts); err != nil {
		t.Fatal(err)
	}
	out := buf.String()
	if strings.Contains(out, "a=f") || strings.Contains(out, "a=a") {
		t.Errorf("disabled animation must not emit frames: %q", out)
	}
	if !strings.Contains(out, "f=32,s=2,v=2,c=1,r=1,i=1,m=0;") {
		t.Errorf("disabled animation must render first frame: %.60q", out)
	}
}

func TestIterm2AnimationBranches(t *testing.T) {
	p := &iterm2Protocol{}
	anim := testAnimationFragment([]byte("GIF89a-fake-bytes"))
	opts := &RenderOptions{PlacementBase: 1, CellSize: detector.Size{Width: 10, Height: 20}}

	// With raw GIF bytes: the exact bytes are embedded.
	var buf bytes.Buffer
	if err := p.Write(&buf, &IntermediateRep{Fragments: []Fragment{anim}}, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "inline=1;width=1c;height=1c;") {
		t.Errorf("embedded dims wrong: %.80q", buf.String())
	}
	if !strings.Contains(buf.String(), "R0lGODlhLWZha2UtYnl0ZXM=") { // base64 of the raw fake GIF bytes
		t.Errorf("raw GIF bytes not embedded: %q", buf.String())
	}

	// Without raw bytes: first frame re-encoded as PNG.
	anim.Source = nil
	buf.Reset()
	if err := p.Write(&buf, &IntermediateRep{Fragments: []Fragment{anim}}, opts); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(buf.String(), "\x1b]1337;File=inline=1;") {
		t.Errorf("first-frame branch missing 1337 escape: %.60q", buf.String())
	}
}

// TestDegradationMatrix pins the §1.5 behavior for every protocol × fragment
// shape combination.
func TestDegradationMatrix(t *testing.T) {
	opts := &RenderOptions{PlacementBase: 1, CellSize: detector.Size{Width: 10, Height: 20}}
	static := &ImageFragment{Image: solidFrame(2, 2, color.RGBA{9, 9, 9, 255}), Alt: "s", Rect: layout.CellRect{Width: 1, Height: 1}}
	animRaw := testAnimationFragment([]byte("GIF89a-x"))
	animNoRaw := testAnimationFragment(nil)

	cases := []struct {
		name    string
		proto   Protocol
		frag    Fragment
		want    string
		wantNot string
	}{
		{"kitty-on/anim", func() Protocol { k := &kittyProtocol{}; k.EnableAnimation(true); return k }(), animNoRaw, "a=f", ""},
		{"kitty-off/anim", &kittyProtocol{}, animNoRaw, "f=32", "a=f"},
		{"kitty/static", &kittyProtocol{}, static, "f=32", "a=f"},
		{"iterm2/anim+raw", &iterm2Protocol{}, animRaw, "inline=1;width=1c", ""},
		{"iterm2/anim-raw", &iterm2Protocol{}, animNoRaw, "1337;File=inline=1", ""},
		{"iterm2/static", &iterm2Protocol{}, static, "1337;File=inline=1", ""},
		{"sixel/anim", &sixelProtocol{}, animNoRaw, "\x1bP0;1;0q", "a=f"},
		{"sixel/static", &sixelProtocol{}, static, "\x1bP0;1;0q", ""},
		{"ansiart/anim", &ansiArtProtocol{}, animNoRaw, "▀", "a=f"},
		{"ansiart/static", &ansiArtProtocol{}, static, "▀", ""},
		{"plain/anim", &plainProtocol{}, animNoRaw, "[animation: anim]", ""},
		{"plain/static", &plainProtocol{}, static, "[image: s]", ""},
	}
	for _, tc := range cases {
		var buf bytes.Buffer
		if err := tc.proto.Write(&buf, &IntermediateRep{Fragments: []Fragment{tc.frag}}, opts); err != nil {
			t.Fatalf("%s: %v", tc.name, err)
		}
		out := buf.String()
		if tc.want != "" && !strings.Contains(out, tc.want) {
			t.Errorf("%s: missing %q in %.80q", tc.name, tc.want, out)
		}
		if tc.wantNot != "" && strings.Contains(out, tc.wantNot) {
			t.Errorf("%s: must not contain %q in %.80q", tc.name, tc.wantNot, out)
		}
	}
}

func TestPlainAnimationPlaceholder(t *testing.T) {
	p := &plainProtocol{}
	anim := testAnimationFragment(nil)
	anim.Alt = ""
	var buf bytes.Buffer
	if err := p.Write(&buf, &IntermediateRep{Fragments: []Fragment{anim}}, nil); err != nil {
		t.Fatal(err)
	}
	if buf.String() != "[animation: animation]\n" {
		t.Errorf("placeholder = %q", buf.String())
	}
}
