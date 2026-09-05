package encoder

import (
	"bytes"
	"errors"
	"fmt"
	"image"
	"image/color"
	"sort"
)

// sixelBandHeight is the number of vertical pixels one sixel character encodes.
const sixelBandHeight = 6

// colorKey quantizes an RGB color to a 15-bit histogram key.
func colorKey(c color.RGBA) uint32 {
	return uint32(c.R>>3)<<10 | uint32(c.G>>3)<<5 | uint32(c.B>>3)
}

// sixelPaletteEntry is one color of the sixel palette.
type sixelPaletteEntry struct {
	key   uint32
	c     color.RGBA
	count int
}

// EncodeSixel encodes img as a sixel DCS payload body (without the DCS/ST
// wrappers), quantizing to at most maxColors palette entries. Callers are
// expected to downsample to the display size beforehand; sixel display size
// equals pixel size.
func EncodeSixel(img image.Image, maxColors int) ([]byte, error) {
	b := img.Bounds()
	if b.Empty() {
		return nil, errors.New("encoder: cannot encode empty image")
	}
	if maxColors <= 0 || maxColors > 256 {
		maxColors = 256
	}

	// Collect a color histogram and pick the most frequent maxColors entries.
	counts := map[uint32]int{}
	repr := map[uint32]color.RGBA{}
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.RGBAModel.Convert(img.At(x, y)).(color.RGBA)
			k := colorKey(c)
			counts[k]++
			if _, ok := repr[k]; !ok {
				repr[k] = c
			}
		}
	}
	entries := make([]sixelPaletteEntry, 0, len(counts))
	for k, n := range counts {
		entries = append(entries, sixelPaletteEntry{key: k, c: repr[k], count: n})
	}
	sort.Slice(entries, func(i, j int) bool {
		if entries[i].count != entries[j].count {
			return entries[i].count > entries[j].count
		}
		return entries[i].key < entries[j].key
	})
	if len(entries) > maxColors {
		entries = entries[:maxColors]
	}
	index := map[uint32]uint8{}
	for i, e := range entries {
		index[e.key] = uint8(i + 1) // color numbers start at 1
	}

	// Resolve every pixel to a palette number up front.
	w, h := b.Dx(), b.Dy()
	plan := make([][]uint8, h)
	for y := 0; y < h; y++ {
		row := make([]uint8, w)
		for x := 0; x < w; x++ {
			row[x] = index[colorKey(color.RGBAModel.Convert(img.At(b.Min.X+x, b.Min.Y+y)).(color.RGBA))]
		}
		plan[y] = row
	}

	var buf bytes.Buffer
	for i, e := range entries {
		fmt.Fprintf(&buf, "#%d;2;%d;%d;%d", i+1,
			(int(e.c.R)*100+127)/255, (int(e.c.G)*100+127)/255, (int(e.c.B)*100+127)/255)
	}

	for band := 0; band < h; band += sixelBandHeight {
		used := map[uint8]bool{}
		for y := band; y < band+sixelBandHeight && y < h; y++ {
			for x := 0; x < w; x++ {
				used[plan[y][x]] = true
			}
		}
		colors := make([]int, 0, len(used))
		for c := range used {
			colors = append(colors, int(c))
		}
		sort.Ints(colors)

		for _, cn := range colors {
			buf.WriteString(fmt.Sprintf("#%d", cn))
			emitBandRow(&buf, plan, band, w, uint8(cn))
		}
		if band+sixelBandHeight >= h {
			buf.WriteByte('$')
		} else {
			buf.WriteByte('-')
		}
	}
	return buf.Bytes(), nil
}

// emitBandRow writes the sixel data for one color across one 6-pixel band,
// folding runs of three or more identical characters into !Pn<ch> RLE.
func emitBandRow(buf *bytes.Buffer, plan [][]uint8, band, w int, cn uint8) {
	flush := func(runChar byte, runLen int) {
		for runLen >= 3 {
			n := runLen
			if n > 255 {
				n = 255
			}
			fmt.Fprintf(buf, "!%d%c", n, runChar)
			runLen -= n
		}
		for i := 0; i < runLen; i++ {
			buf.WriteByte(runChar)
		}
	}

	var cur byte
	var runLen int
	for x := 0; x < w; x++ {
		var mask byte
		for i := 0; i < sixelBandHeight; i++ {
			y := band + i
			if y < len(plan) && plan[y][x] == cn {
				mask |= 1 << i
			}
		}
		ch := byte(63 + mask)
		if runLen > 0 && ch == cur {
			runLen++
			continue
		}
		flush(cur, runLen)
		cur, runLen = ch, 1
	}
	flush(cur, runLen)
}
