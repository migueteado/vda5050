package validations

import (
	"fmt"
	"sort"

	vdaerrors "vda5050/common/errors"
	"vda5050/common/models"
)

type Sequence struct {
	SequenceId int
	IsNode     bool
	Node       *models.Node
	Edge       *models.Edge
}

func ValidateOrder(order models.Order) *vdaerrors.VDAError {
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
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidOrder,
				fmt.Sprintf("sequenceId at index %d is not strictly increasing", i),
			)
		}

		// Nodes get even sequenceIds, edges get odd
		if seq.IsNode && seq.SequenceId%2 != 0 {
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidOrder,
				fmt.Sprintf("node at index %d must have an even sequenceId", i),
			)
		}
		if !seq.IsNode && seq.SequenceId%2 == 0 {
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidOrder,
				fmt.Sprintf("edge at index %d must have an odd sequenceId", i),
			)
		}

		// Once an unreleased element is seen, nothing released may follow
		released := seq.IsNode && seq.Node.Released || !seq.IsNode && seq.Edge.Released
		if released && seenUnreleased {
			return vdaerrors.New(
				vdaerrors.ErrorTypeInvalidOrder,
				fmt.Sprintf("element at index %d is released after an unreleased element", i),
			)
		}
		if !released {
			seenUnreleased = true
		}

		if i == 0 && !seq.IsNode {
			return vdaerrors.New(vdaerrors.ErrorTypeInvalidOrder, "order must start in a node")
		}
		if i == len(sequence)-1 && !seq.IsNode {
			return vdaerrors.New(vdaerrors.ErrorTypeInvalidOrder, "order must end in a node")
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
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("node at index %d must have an edge on the left", i),
				)
			}
			if nextSeq != nil && nextSeq.IsNode {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("node at index %d must have an edge on the right", i),
				)
			}
		} else {
			// Edges must have nodes on both sides and the nodes must equal
			// their startNode and endNode
			if prevSeq == nil || !prevSeq.IsNode {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("edge at index %d must have a node on the left", i),
				)
			} else if prevSeq.Node.NodeId != seq.Edge.StartNodeId {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("edge at index %d has incorrect start node", i),
				)
			}
			if nextSeq == nil || !nextSeq.IsNode {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("edge at index %d must have a node on the right", i),
				)
			} else if nextSeq.Node.NodeId != seq.Edge.EndNodeId {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("edge at index %d has incorrect end node", i),
				)
			}

			if seq.Edge.Released && (!prevSeq.Node.Released || !nextSeq.Node.Released) {
				return vdaerrors.New(
					vdaerrors.ErrorTypeInvalidOrder,
					fmt.Sprintf("edge at index %d is released but its nodes are not", i),
				)
			}
		}
	}

	return nil
}
