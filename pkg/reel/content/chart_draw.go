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

// Plot margins in pixels, sized for basicfont's 7x13 face. marginRight
// widens to marginAxis when a right-axis series exists, symmetric with the
// left axis labels.
const (
	marginTop    = 22
	marginLeft   = 56
	marginRight  = 10
	marginAxis   = 56
	marginBottom = 22
)

// tickPx is the target pixel spacing between y axis ticks.
const tickPx = 40

// seriesPalette colors the series consistently in the drawing and the
// legend. Colors cycle from the 7th series onward — six series is the
// readability ceiling for terminal charts; more series are a spec problem.
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

// niceStep returns the smallest "nice" step (1/2/5 × 10^n) >= rawStep.
func niceStep(rawStep float64) float64 {
	if rawStep <= 0 {
		return 1
	}
	mag := math.Pow(10, math.Floor(math.Log10(rawStep)))
	for _, m := range []float64{1, 2, 5, 10} {
		if s := m * mag; s >= rawStep {
			return s
		}
	}
	return 10 * mag
}

// axisRange computes the nice-ified y axis range for one axis: the raw
// extent padded for line charts, constrained to include the zero baseline
// for bar charts, then floored/ceiled to nice step multiples. Ticks start
// at ymin and advance by step.
func axisRange(spec ChartSpec, axis int, plotH int) (ymin, ymax, step float64) {
	lo, hi := math.Inf(1), math.Inf(-1)
	for _, s := range spec.Series {
		if s.YAxis != axis {
			continue
		}
		for _, v := range s.Values {
			lo = math.Min(lo, v)
			hi = math.Max(hi, v)
		}
	}
	if math.IsInf(lo, 0) {
		return 0, 1, 1
	}
	if spec.Type == ChartBar {
		lo = math.Min(lo, 0)
		hi = math.Max(hi, 0)
	} else {
		pad := (hi - lo) * 0.05
		lo -= pad
		hi += pad
	}
	if hi == lo {
		hi = lo + 1
	}
	targetTicks := float64(max(2, plotH/tickPx))
	step = niceStep((hi - lo) / targetTicks)
	ymin = math.Floor(lo/step) * step
	ymax = math.Ceil(hi/step) * step
	return ymin, ymax, step
}

// rasterize draws spec onto a width×height RGBA canvas with a transparent
// background and returns the image plus legend fragments (nil when the
// chart needs no legend).
func rasterize(spec ChartSpec, width, height int) (*image.RGBA, []protocol.Fragment, error) {
	if width < marginLeft+marginRight+20 || height < marginTop+marginBottom+20 {
		return nil, nil, fmt.Errorf("content: chart area %dx%d too small", width, height)
	}

	n := len(spec.Labels)
	if n == 0 {
		n = len(spec.Series[0].Values)
	}
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	x0, y0 := marginLeft, marginTop
	x1 := width - marginRight
	if hasRightAxis(spec) {
		x1 = width - marginAxis
	}
	y1 := height - marginBottom

	yminL, ymaxL, stepL := axisRange(spec, 0, y1-y0)
	yminR, ymaxR, stepR := axisRange(spec, 1, y1-y0)
	yscale := func(axis int) func(float64) int {
		ymin, ymax := yminL, ymaxL
		if axis == 1 {
			ymin, ymax = yminR, ymaxR
		}
		return func(v float64) int {
			return y1 - int((v-ymin)/(ymax-ymin)*float64(y1-y0))
		}
	}

	// Left grid lines and tick labels.
	for v := yminL; v <= ymaxL; v += stepL {
		y := yscale(0)(v)
		drawHLine(img, x0, x1, y, gridColor)
		drawText(img, fmt.Sprintf("%.4g", v), x0-6, y-6, axisColor, true)
	}
	// Right axis ticks (no grid lines), when used.
	if hasRightAxis(spec) {
		for v := yminR; v <= ymaxR; v += stepR {
			y := yscale(1)(v)
			drawText(img, fmt.Sprintf("%.4g", v), x1+6, y-6, axisColor, false)
		}
	}
	// Axes.
	drawVLine(img, x0, y0, y1, axisColor)
	drawHLine(img, x0, x1, y1, axisColor)

	groupW := float64(x1-x0) / float64(n)
	for i, s := range spec.Series {
		scale := yscale(s.YAxis)
		ymin := yminL
		if s.YAxis == 1 {
			ymin = yminR
		}
		switch spec.Type {
		case ChartBar:
			drawBars(img, s.Values, n, i, len(spec.Series), groupW, x0, y1, scale, ymin)
		case ChartLine:
			drawLineSeries(img, s.Values, n, i, groupW, x0, scale)
		case ChartScatter:
			drawScatterPoints(img, s.Values, n, i, groupW, x0, scale)
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

// hasRightAxis reports whether any series is scaled to the right axis.
func hasRightAxis(spec ChartSpec) bool {
	for _, s := range spec.Series {
		if s.YAxis == 1 {
			return true
		}
	}
	return false
}

// legendFragments builds the "■ name1  ■ name2" legend line whenever the
// chart has more than one series, or its single series is named.
func legendFragments(spec ChartSpec) []protocol.Fragment {
	if len(spec.Series) == 0 || (len(spec.Series) == 1 && spec.Series[0].Name == "") {
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

// drawBars renders one series of grouped bars. The zero baseline comes from
// yscale(0) (or the plot bottom when the axis minimum is above zero).
func drawBars(img *image.RGBA, values []float64, n, seriesIdx, numSeries int, groupW float64, x0, y1 int, yscale func(float64) int, ymin float64) {
	barW := int(groupW)/numSeries - 1
	if barW < 1 {
		barW = 1
	}
	c := seriesColor(seriesIdx)
	for i, v := range values {
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
		x := int(float64(x0) + groupW*float64(i) + groupW/2)
		y := yscale(v)
		if i > 0 {
			drawBresenham(img, px, py, x, y, c)
		}
		fillRect(img, x-1, y-1, x+2, y+2, c)
		px, py = x, y
	}
}

// drawScatterPoints renders each value as a 3×3 block; no lines, no
// interpolation.
func drawScatterPoints(img *image.RGBA, values []float64, n, seriesIdx int, groupW float64, x0 int, yscale func(float64) int) {
	c := seriesColor(seriesIdx)
	for i, v := range values {
		x := int(float64(x0) + groupW*float64(i) + groupW/2)
		y := yscale(v)
		fillRect(img, x-1, y-1, x+2, y+2, c)
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
