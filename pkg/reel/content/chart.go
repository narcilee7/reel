package content

import (
	"errors"

	"github.com/narcilee7/reel/pkg/reel/protocol"
)

// Chart is a placeholder for chart rendering.
type Chart struct {
	Title  string
	Series []float64
}

var errChartNotImplemented = errors.New("content: chart rendering not implemented")

func (c *Chart) Kind() protocol.ContentKind { return protocol.ContentKindChart }

// ToIR is reserved; chart rendering is not implemented yet.
func (c *Chart) ToIR(ctx protocol.Context) (*protocol.IntermediateRep, error) {
	return nil, errChartNotImplemented
}
