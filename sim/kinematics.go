package sim

import (
	"math"
	"time"

	"vda5050/common/models"
)

const tickRate = 100 * time.Millisecond // 10 Hz

type Position struct {
	X     float64
	Y     float64
	Theta float64
}

// Simulator drives an already-validated Order in a straight line from
// node to node, at each edge's maximumSpeed, ticking at 10 Hz. It uses
// the destination node's allowedDeviationXY ellipse to decide when it
// has "arrived" rather than waiting for an exact coordinate match.
type Simulator struct {
	order    models.Order
	edgeIdx  int
	position Position
}

func NewSimulator(order models.Order) *Simulator {
	start := order.Nodes[0].NodePosition
	return &Simulator{
		order:    order,
		position: Position{X: start.X, Y: start.Y, Theta: start.Theta},
	}
}

// Run blocks until every edge has been traversed, calling onTick once
// per 10 Hz tick with the current position and, on arrival, the nodeId
// just reached (empty string otherwise).
func (s *Simulator) Run(onTick func(pos Position, arrivedNodeId string)) {
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	for s.edgeIdx < len(s.order.Edges) {
		<-ticker.C
		arrivedNodeId := s.step()
		onTick(s.position, arrivedNodeId)
	}
}

// step advances the simulation by one tick and returns the nodeId just
// arrived at, or "" if still travelling.
func (s *Simulator) step() string {
	edge := s.order.Edges[s.edgeIdx]
	target := s.order.Nodes[s.edgeIdx+1]

	dx := target.NodePosition.X - s.position.X
	dy := target.NodePosition.Y - s.position.Y
	dist := math.Hypot(dx, dy)

	if insideDeviationEllipse(s.position, target.NodePosition) {
		s.position = Position{
			X:     target.NodePosition.X,
			Y:     target.NodePosition.Y,
			Theta: target.NodePosition.Theta,
		}
		s.edgeIdx++
		return target.NodeId
	}

	step := edge.MaximumSpeed * tickRate.Seconds()
	if step >= dist {
		s.position.X, s.position.Y = target.NodePosition.X, target.NodePosition.Y
	} else {
		s.position.X += dx / dist * step
		s.position.Y += dy / dist * step
	}
	s.position.Theta = math.Atan2(dy, dx)

	return ""
}

// insideDeviationEllipse implements the §6.6.2 test: transform the
// point into the ellipse's frame (rotated by theta, centred on the
// node), then check it against the semi-major/semi-minor axes. a=b=0
// means "as precise as technically possible" — fall back to a tiny
// epsilon rather than a degenerate zero-area ellipse.
func insideDeviationEllipse(p Position, node models.NodePosition) bool {
	dev := node.AllowedDeviationXY
	if dev.A == 0 && dev.B == 0 {
		const epsilon = 0.01
		return math.Hypot(p.X-node.X, p.Y-node.Y) <= epsilon
	}

	dx := p.X - node.X
	dy := p.Y - node.Y

	cos, sin := math.Cos(dev.Theta), math.Sin(dev.Theta)
	rx := dx*cos + dy*sin
	ry := -dx*sin + dy*cos

	return (rx*rx)/(dev.A*dev.A)+(ry*ry)/(dev.B*dev.B) <= 1
}
