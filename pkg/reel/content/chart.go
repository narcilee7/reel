package content

import (
	"errors"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// ChartType selects the chart drawing style.
type ChartType int

const (
	ChartBar ChartType = iota
	ChartLine
)

// Series is one named data series; Values align one-to-one with the chart's
// Labels.
type Series struct {
	Name   string
	Values []float64
}

// ChartSpec is a declarative, data-only description of a chart.
type ChartSpec struct {
	Type   ChartType
	Title  string
	Labels []string // x axis categories
	Series []Series
}

// Chart renders a declarative ChartSpec as a raster image fragment, plus a
// one-line text legend for multi-series charts.
type Chart struct {
	Spec ChartSpec
}

func (c *Chart) Kind() protocol.ContentKind { return protocol.ContentKindChart }

// defaultChartCells is the render area in cells when the Context cannot
// provide one.
const (
	defaultChartCols = 60
	defaultChartRows = 25
)

// ToIR rasterizes the spec to the render area (MaxCells × CellSize pixels)
// and returns it as an ImageFragment followed by an optional legend
// TextFragment.
func (c *Chart) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	cols, rows := ctx.MaxCells.Width, ctx.MaxCells.Height
	if cols <= 0 {
		cols = defaultChartCols
	}
	if rows <= 0 {
		rows = defaultChartRows
	}
	cw, ch := ctx.CellSize.Width, ctx.CellSize.Height
	if cw <= 0 {
		cw = 10
	}
	if ch <= 0 {
		ch = 20
	}

	img, legend, err := rasterize(c.Spec, cols*cw, rows*ch)
	if err != nil {
		return nil, err
	}
	fragments := []protocol.Fragment{&protocol.ImageFragment{Image: img, Alt: c.Spec.Title}}
	if len(legend) > 0 {
		fragments = append(fragments, legend...)
	}
	return &protocol.IntermediateRep{Fragments: fragments}, nil
}

var errChartNoSeries = errors.New("content: chart needs at least one series")
