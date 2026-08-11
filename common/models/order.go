package models

import (
	"fmt"
	"sort"

	"vda5050/common"
)

type OrientationType string

const (
	ORIENTATION_TANGENTIAL OrientationType = "TANGENTIAL"
	ORIENTATION_GLOBAL     OrientationType = "GLOBAL"
)

type Action struct{}

type NodePosition struct {
	X                  float64 `json:"x"`
	Y                  float64 `json:"y"`
	Theta              float64 `json:"theta"`
	MapId              string  `json:"mapId"`
	AllowedDeviationXY struct {
		A     float64 `json:"a"`
		B     float64 `json:"b"`
		Theta float64 `json:"theta"`
	} `json:"allowedDeviationXY"`
	AllowedDeviationTheta float64 `json:"allowedDeviationTheta"`
}

type Node struct {
	NodeId         string       `json:"nodeId"`
	SequenceId     int          `json:"sequenceId"`
	NodeDescriptor string       `json:"nodeDescriptor"`
	Released       bool         `json:"released"`
	NodePosition   NodePosition `json:"nodePosition"`
	Actions        []Action     `json:"actions"`
}

type Trajectory struct {
	Degree        int        `json:"degree"`
	KnotVector    []struct{} `json:"knotVector"`
	ControlPoints []struct{} `json:"controlPoints"`
}

type Edge struct {
	EdgeId                   string          `json:"edgeId"`
	SequenceId               int             `json:"sequenceId"`
	StartNodeId              string          `json:"startNodeId"`
	EndNodeId                string          `json:"endNodeId"`
	Released                 bool            `json:"released"`
	MaximumSpeed             float64         `json:"maximumSpeed"`
	MaximumMobileRobotHeight float64         `json:"maximumMobileRobotHeight"`
	Orientation              float64         `json:"orientation"`
	OrientationType          OrientationType `json:"orientationType"`
	RotationAllowed          bool            `json:"rotationAllowed"`
	Trajectory               Trajectory      `json:"trajectory"`
	Corridor                 struct{}        `json:"corridor"`
	Actions                  []Action        `json:"actions"`
}

type Order struct {
	common.Header
	OrderId          string `json:"orderId"`
	OrderUpdateId    int    `json:"orderUpdateId"`
	OrderDescription string `json:"orderDescription"`
	Nodes            []Node `json:"nodes"`
	Edges            []Edge `json:"edges"`
}

type Sequence struct {
	SequenceId int
	IsNode     bool
	Node       *Node
	Edge       *Edge
}

func Validate(order Order) error {
	var sequence []Sequence = []Sequence{}

	for _, node := range order.Nodes {
		sequence = append(sequence, Sequence{
			SequenceId: node.SequenceId,
			IsNode:     true,
			Node:       &node,
			Edge:       nil,
		})
	}

	for _, edge := range order.Edges {
		sequence = append(sequence, Sequence{
			SequenceId: edge.SequenceId,
			IsNode:     false,
			Node:       nil,
			Edge:       &edge,
		})
	}

	sort.Slice(sequence, func(i, j int) bool {
		return sequence[i].SequenceId < sequence[j].SequenceId
	})

	seenUnreleased := false

	for i, seq := range sequence {
		// Verify that sequenceId is strictly increasing with no gaps
		if i > 0 && seq.SequenceId != sequence[i-1].SequenceId+1 {
			return fmt.Errorf("sequenceId at index %d is not strictly increasing", i)
		}

		// Nodes get even sequenceIds, edges get odd
		if seq.IsNode && seq.SequenceId%2 != 0 {
			return fmt.Errorf("node at index %d must have an even sequenceId", i)
		}
		if !seq.IsNode && seq.SequenceId%2 == 0 {
			return fmt.Errorf("edge at index %d must have an odd sequenceId", i)
		}

		// Once an unreleased element is seen, nothing released may follow
		released := seq.IsNode && seq.Node.Released || !seq.IsNode && seq.Edge.Released
		if released && seenUnreleased {
			return fmt.Errorf("element at index %d is released after an unreleased element", i)
		}
		if !released {
			seenUnreleased = true
		}

		if i == 0 && !seq.IsNode {
			return fmt.Errorf("order must start in a node")
		}
		if i == len(sequence)-1 && !seq.IsNode {
			return fmt.Errorf("order must end in a node")
		}

		var prevSeq, nextSeq *Sequence
		if i > 0 {
			prevSeq = &sequence[i-1]
		}
		if i < len(sequence)-1 {
			nextSeq = &sequence[i+1]
		}

		if seq.IsNode {
			// Nodes must have edges on both sides (where a side exists)
			if prevSeq != nil && prevSeq.IsNode {
				return fmt.Errorf("node at index %d must have an edge on the left", i)
			}
			if nextSeq != nil && nextSeq.IsNode {
				return fmt.Errorf("node at index %d must have an edge on the right", i)
			}
		} else {
			// Edges must have nodes on both sides and the nodes must equal
			// their startNode and endNode
			if prevSeq == nil || !prevSeq.IsNode {
				return fmt.Errorf("edge at index %d must have a node on the left", i)
			} else if prevSeq.Node.NodeId != seq.Edge.StartNodeId {
				return fmt.Errorf("edge at index %d has incorrect start node", i)
			}
			if nextSeq == nil || !nextSeq.IsNode {
				return fmt.Errorf("edge at index %d must have a node on the right", i)
			} else if nextSeq.Node.NodeId != seq.Edge.EndNodeId {
				return fmt.Errorf("edge at index %d has incorrect end node", i)
			}

			if seq.Edge.Released && (!prevSeq.Node.Released || !nextSeq.Node.Released) {
				return fmt.Errorf("edge at index %d is released but its nodes are not", i)
			}
		}
	}

	return nil
}
