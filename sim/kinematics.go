package sim

import (
	"math"
	"sync"
	"time"

	"vda5050/common/geometry"
	"vda5050/common/models"
)

const tickRate = 100 * time.Millisecond // 10 Hz

// Simulator drives a stitched nodes/edges chain in a straight line
// from node to node, at each edge's maximumSpeed, ticking at 10 Hz. It
// uses the destination node's allowedDeviationXY ellipse to decide
// when it has "arrived" rather than waiting for an exact coordinate
// match.
//
// It only drives through released edges. When it reaches an
// unreleased edge (the decision point) it blocks until Extend
// supplies a newer nodes/edges chain — mirroring the spec's rule that
// the robot stops and waits at its decision point until fleet control
// extends the base.
type Simulator struct {
	mu        sync.Mutex
	nodes     []models.Node
	edges     []models.Edge
	edgeIdx   int
	position  geometry.Position
	wake      chan struct{}
	driveGate func() bool
}

func NewSimulator(nodes []models.Node, edges []models.Edge) *Simulator {
	start := nodes[0].NodePosition
	return &Simulator{
		nodes:    nodes,
		edges:    edges,
		position: geometry.Position{X: start.X, Y: start.Y, Theta: start.Theta},
		wake:     make(chan struct{}, 1),
	}
}

// Extend replaces the simulator's known nodes/edges with a freshly
// stitched chain (an accepted order update) and wakes the driving loop
// if it was blocked waiting at a decision point.
func (s *Simulator) Extend(nodes []models.Node, edges []models.Edge) {
	s.mu.Lock()
	s.nodes = nodes
	s.edges = edges
	s.mu.Unlock()

	select {
	case s.wake <- struct{}{}:
	default:
	}
}

// SetDriveGate installs a check that must return true for the
// simulator to actually move on a tick - e.g. the action queue's
// DrivingAllowed() or a startPause/stopPause flag. A nil gate (the
// default) never holds driving back.
func (s *Simulator) SetDriveGate(gate func() bool) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.driveGate = gate
}

// Position returns the simulator's current position.
func (s *Simulator) Position() geometry.Position {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.position
}

// canDrive reports whether there is a released edge left to traverse.
func (s *Simulator) canDrive() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.edgeIdx < len(s.edges) && s.edges[s.edgeIdx].Released
}

// Driving reports whether the robot is actively moving right now, as
// opposed to stopped at a decision point waiting for more base.
func (s *Simulator) Driving() bool {
	return s.canDrive()
}

// Run blocks forever, calling onTick once per 10 Hz tick while driving
// and pausing at each decision point until Extend is called.
func (s *Simulator) Run(onTick func(pos geometry.Position, arrivedNodeId string)) {
	ticker := time.NewTicker(tickRate)
	defer ticker.Stop()

	for {
		if !s.canDrive() {
			<-s.wake
			continue
		}
		<-ticker.C

		s.mu.Lock()
		gate := s.driveGate
		s.mu.Unlock()
		if gate != nil && !gate() {
			continue // held by an action queue or pause; re-check next tick
		}

		arrivedNodeId := s.step()
		onTick(s.Position(), arrivedNodeId)
	}
}

// step advances the simulation by one tick and returns the nodeId just
// arrived at, or "" if still travelling.
func (s *Simulator) step() string {
	s.mu.Lock()
	defer s.mu.Unlock()

	edge := s.edges[s.edgeIdx]
	target := s.nodes[s.edgeIdx+1]

	dx := target.NodePosition.X - s.position.X
	dy := target.NodePosition.Y - s.position.Y
	dist := math.Hypot(dx, dy)

	if geometry.InsideDeviationEllipse(s.position, target.NodePosition) {
		s.position = geometry.Position{
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
