package geometry

import "math"

type Position struct {
	X     float64 `json:"x"`
	Y     float64 `json:"y"`
	Theta float64 `json:"theta"`
}

type AllowedDeviationXY struct {
	A     float64 `json:"a"`
	B     float64 `json:"b"`
	Theta float64 `json:"theta"`
}

type NodePosition struct {
	Position
	MapId                 string             `json:"mapId"`
	AllowedDeviationXY    AllowedDeviationXY `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64            `json:"allowedDeviationTheta"`
}

// insideDeviationEllipse implements the §6.6.2 test: transform the
// point into the ellipse's frame (rotated by theta, centred on the
// node), then check it against the semi-major/semi-minor axes. a=b=0
// means "as precise as technically possible" — fall back to a tiny
// epsilon rather than a degenerate zero-area ellipse.
func InsideDeviationEllipse(p Position, node NodePosition) bool {
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
