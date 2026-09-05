package content

import (
	"fmt"
	"image"
	"image/color"
	"math"

	"golang.org/x/image/font"
	"golang.org/x/image/font/basicfont"
	"golang.org/x/image/math/fixed"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Plot margins in pixels, sized for basicfont's 7x13 face.
const (
	marginTop    = 22
	marginLeft   = 56
	marginRight  = 10
	marginBottom = 22
)

// seriesPalette colors the series consistently in the drawing and the legend.
var seriesPalette = []color.RGBA{
	{80, 250, 123, 255},  // green
	{139, 233, 253, 255}, // cyan
	{255, 184, 108, 255}, // orange
	{255, 121, 198, 255}, // pink
	{189, 147, 249, 255}, // purple
	{241, 250, 140, 255}, // yellow
}

func seriesColor(i int) color.RGBA { return seriesPalette[i%len(seriesPalette)] }

func seriesSGR(i int) string {
	c := seriesColor(i)
	return fmt.Sprintf("38;2;%d;%d;%d", c.R, c.G, c.B)
}

var (
	axisColor = color.RGBA{98, 114, 164, 255}
	gridColor = color.RGBA{68, 71, 90, 255}
)

// rasterize draws spec onto a width×height RGBA canvas with a transparent
// background and returns the image plus legend fragments (nil for
// single-series charts).
func rasterize(spec ChartSpec, width, height int) (*image.RGBA, []protocol.Fragment, error) {
	if len(spec.Series) == 0 {
		return nil, nil, errChartNoSeries
	}
	if width < marginLeft+marginRight+20 || height < marginTop+marginBottom+20 {
		return nil, nil, fmt.Errorf("content: chart area %dx%d too small", width, height)
	}

	n := len(spec.Labels)
	if n == 0 {
		for _, s := range spec.Series {
			if len(s.Values) > n {
				n = len(s.Values)
			}
		}
	}
	if n == 0 {
		return nil, nil, fmt.Errorf("content: chart has no data points")
	}

	img := image.NewRGBA(image.Rect(0, 0, width, height))
	x0, y0 := marginLeft, marginTop
	x1, y1 := width-marginRight, height-marginBottom

	ymin, ymax := valueRange(spec)
	yscale := func(v float64) int {
		return y1 - int((v-ymin)/(ymax-ymin)*float64(y1-y0))
	}

	// Horizontal grid lines and y tick labels.
	for i := 0; i <= 4; i++ {
		v := ymin + (ymax-ymin)*float64(i)/4
		y := yscale(v)
		drawHLine(img, x0, x1, y, gridColor)
		drawText(img, fmt.Sprintf("%.4g", v), x0-6, y-6, axisColor, true)
	}
	// Axes.
	drawVLine(img, x0, y0, y1, axisColor)
	drawHLine(img, x0, x1, y1, axisColor)

	groupW := float64(x1-x0) / float64(n)
	for i, s := range spec.Series {
		switch spec.Type {
		case ChartBar:
			drawBars(img, s.Values[:min(len(s.Values), n)], n, i, len(spec.Series), groupW, x0, y1, yscale, ymin)
		case ChartLine:
			drawLineSeries(img, s.Values[:min(len(s.Values), n)], n, i, groupW, x0, yscale)
		}
	}

	// x labels, centered under each group; skipped when they would collide.
	for i, label := range spec.Labels {
		if len(label)*7 > int(groupW) {
			continue
		}
		cx := x0 + int(groupW*float64(i)+groupW/2)
		drawText(img, label, cx, y1+14, axisColor, true)
	}

	if spec.Title != "" {
		drawText(img, spec.Title, width/2, 14, color.White, true)
	}

	return img, legendFragments(spec), nil
}

// legendFragments builds the "■ name1  ■ name2" legend line for multi-series
// charts, one styled fragment per series.
func legendFragments(spec ChartSpec) []protocol.Fragment {
	if len(spec.Series) <= 1 {
		return nil
	}
	var out []protocol.Fragment
	for i, s := range spec.Series {
		if i > 0 {
			out = append(out, &protocol.TextFragment{Text: "  "})
		}
		out = append(out, &protocol.TextFragment{Text: "■ " + s.Name, Style: protocol.Style{FG: seriesSGR(i)}})
	}
	out = append(out, &protocol.TextFragment{Text: "\n"})
	return out
}

// valueRange computes the y axis range: bar charts include the zero baseline,
// line charts pad the data extent by 5%.
func valueRange(spec ChartSpec) (ymin, ymax float64) {
	if spec.Type == ChartBar {
		ymin = math.Inf(1)
		ymax = math.Inf(-1)
		for _, s := range spec.Series {
			for _, v := range s.Values {
				ymin = math.Min(ymin, math.Min(v, 0))
				ymax = math.Max(ymax, math.Max(v, 0))
			}
		}
		if math.IsInf(ymin, 0) {
			ymin, ymax = 0, 1
		}
	} else {
		ymin = math.Inf(1)
		ymax = math.Inf(-1)
		for _, s := range spec.Series {
			for _, v := range s.Values {
				ymin = math.Min(ymin, v)
				ymax = math.Max(ymax, v)
			}
		}
		if math.IsInf(ymin, 0) {
			ymin, ymax = 0, 1
		}
		pad := (ymax - ymin) * 0.05
		ymin -= pad
		ymax += pad
	}
	if ymax == ymin {
		ymax = ymin + 1
	}
	return ymin, ymax
}

// drawBars renders one series of grouped bars; the zero baseline comes from
// yscale(0) (or the plot bottom when ymin >= 0... handled by yscale).
func drawBars(img *image.RGBA, values []float64, n, seriesIdx, numSeries int, groupW float64, x0, y1 int, yscale func(float64) int, ymin float64) {
	barW := int(groupW)/numSeries - 1
	if barW < 1 {
		barW = 1
	}
	c := seriesColor(seriesIdx)
	for i, v := range values {
		if i >= n {
			break
		}
		gx := float64(x0) + groupW*float64(i)
		bx := int(gx) + seriesIdx*(barW+1)
		yt := yscale(v)
		yb := yscale(0)
		if ymin > 0 {
			yb = y1
		}
		if yt > yb {
			yt, yb = yb, yt
		}
		fillRect(img, bx, yt, bx+barW, yb, c)
	}
}

// drawLineSeries connects the series' points with Bresenham lines and marks
// each point.
func drawLineSeries(img *image.RGBA, values []float64, n, seriesIdx int, groupW float64, x0 int, yscale func(float64) int) {
	c := seriesColor(seriesIdx)
	px, py := 0, 0
	for i, v := range values {
		if i >= n {
			break
		}
		x := int(float64(x0) + groupW*float64(i) + groupW/2)
		y := yscale(v)
		if i > 0 {
			drawBresenham(img, px, py, x, y, c)
		}
		fillRect(img, x-1, y-1, x+2, y+2, c)
		px, py = x, y
	}
}

// drawText draws s with the 7x13 basic font; when center is true, x is the
// horizontal center of the text.
func drawText(img *image.RGBA, s string, x, y int, c color.Color, center bool) {
	if center {
		x -= len(s) * 7 / 2
	}
	if x < 0 {
		x = 0
	}
	d := &font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: basicfont.Face7x13,
		Dot:  fixed.P(x, y),
	}
	d.DrawString(s)
}

func fillRect(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	for y := max(y0, 0); y < min(y1, img.Bounds().Dy()); y++ {
		for x := max(x0, 0); x < min(x1, img.Bounds().Dx()); x++ {
			img.SetRGBA(x, y, c)
		}
	}
}

func drawHLine(img *image.RGBA, x0, x1, y int, c color.RGBA) {
	for x := x0; x < x1; x++ {
		img.SetRGBA(x, y, c)
	}
}

func drawVLine(img *image.RGBA, x, y0, y1 int, c color.RGBA) {
	for y := y0; y < y1; y++ {
		img.SetRGBA(x, y, c)
	}
}

// drawBresenham draws a 1px line between two points.
func drawBresenham(img *image.RGBA, x0, y0, x1, y1 int, c color.RGBA) {
	dx := abs(x1 - x0)
	dy := -abs(y1 - y0)
	sx, sy := 1, 1
	if x0 > x1 {
		sx = -1
	}
	if y0 > y1 {
		sy = -1
	}
	err := dx + dy
	for {
		if image.Pt(x0, y0).In(img.Bounds()) {
			img.SetRGBA(x0, y0, c)
		}
		if x0 == x1 && y0 == y1 {
			return
		}
		e2 := 2 * err
		if e2 >= dy {
			err += dy
			x0 += sx
		}
		if e2 <= dx {
			err += dx
			y0 += sy
		}
	}
}

func abs(v int) int {
	if v < 0 {
		return -v
	}
	return v
}
