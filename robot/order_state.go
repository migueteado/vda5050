package main

import (
	"fmt"
	"slices"
	"sort"
	"strconv"

	vdaerrors "vda5050/common/errors"
	"vda5050/common/geometry"
	"vda5050/common/models"
	"vda5050/common/validations"
)

type OrderState struct {
	OrderId            string
	OrderUpdateId      int
	Cancelled          bool
	Paused             bool   // startPause/stopPause - linked state field (spec §6.2.3)
	LastNodeId         string // spec: lastNodeId - last node id traversed by the robot
	LastNodeSequenceId int    // spec: lastNodeSequenceId - last node sequence id traversed by the robot
	DecisionNodeId     string // last released node
	DecisionSequenceId int
	Nodes              []models.Node // accepted base+horizon, for stitching
	Edges              []models.Edge
}

func (s *OrderState) HasHorizon() bool {
	for _, n := range s.Nodes {
		if !n.Released {
			return true
		}
	}
	return false
}

func (s *OrderState) IsIdle() bool {
	if s.OrderId == "" {
		return true
	}
	return !s.HasHorizon() && s.LastNodeSequenceId == s.DecisionSequenceId
}

func startNodeInRange(incoming models.Order, robotPos geometry.Position) bool {
	first := incoming.Nodes[0]
	return geometry.InsideDeviationEllipse(robotPos, first.NodePosition)
}

func decisionPoint(nodes []models.Node, edges []models.Edge) (string, int) {
	var nodeId string = ""
	var sequenceId int = -1
	var sequence []validations.Sequence = []validations.Sequence{}

	for _, node := range nodes {
		sequence = append(sequence, validations.Sequence{
			SequenceId: node.SequenceId,
			IsNode:     true,
			Node:       &node,
			Edge:       nil,
		})
	}

	for _, edge := range edges {
		sequence = append(sequence, validations.Sequence{
			SequenceId: edge.SequenceId,
			IsNode:     false,
			Node:       nil,
			Edge:       &edge,
		})
	}

	sort.Slice(sequence, func(i, j int) bool {
		return sequence[i].SequenceId < sequence[j].SequenceId
	})

	for _, seq := range sequence {
		if seq.IsNode && seq.Node.Released {
			nodeId = seq.Node.NodeId
			sequenceId = seq.SequenceId
		}

		// Break the loop early when reached unreleased nodes or edges
		if seq.IsNode && !seq.Node.Released || !seq.IsNode && !seq.Edge.Released {
			break
		}
	}
	return nodeId, sequenceId
}

func stitch(
	s *OrderState,
	incoming models.Order,
) ([]models.Node, []models.Edge) {
	// Keep nodes that are released and have a sequenceId equal or higher to the
	// lastNodeSequenceId
	// and are not the decision node (as that is present again in upcoming)
	filteredNodes := slices.DeleteFunc(s.Nodes, func(n models.Node) bool {
		return !n.Released || n.SequenceId < s.LastNodeSequenceId ||
			n.SequenceId == s.DecisionSequenceId
	})

	// Keep edges that are released and have a sequenceId equal or higher to the lastNodeSequenceId
	filteredEdges := slices.DeleteFunc(s.Edges, func(e models.Edge) bool {
		return !e.Released || e.SequenceId < s.LastNodeSequenceId
	})

	stitchedNodes := append(filteredNodes, incoming.Nodes...)
	stitchedEdges := append(filteredEdges, incoming.Edges...)
	return stitchedNodes, stitchedEdges
}

func accept(s *OrderState, incoming models.Order, isNew bool) {
	decisionNodeId, decisionSeqId := decisionPoint(incoming.Nodes, incoming.Edges)
	if !isNew {
		// update requires stitching
		updatedNodes, updatedEdges := stitch(s, incoming)
		s.Nodes = updatedNodes
		s.Edges = updatedEdges
	} else {
		// new requires replacing
		s.OrderId = incoming.OrderId
		// robot is already standing on the first node (check 5
		// confirmed it's in range) - that counts as reached
		s.LastNodeId = incoming.Nodes[0].NodeId
		s.LastNodeSequenceId = incoming.Nodes[0].SequenceId
		s.Cancelled = false
		s.Nodes = incoming.Nodes
		s.Edges = incoming.Edges
	}

	s.OrderUpdateId = incoming.OrderUpdateId
	s.DecisionNodeId = decisionNodeId
	s.DecisionSequenceId = decisionSeqId
}

func AcceptOrder(
	current *OrderState,
	incoming models.Order,
	robotPos geometry.Position,
) (bool, *vdaerrors.VDAError) {
	// Check 2: Is it a new order or an update
	isNew := incoming.OrderId != current.OrderId

	// Check 3 (new only): Is current empty/idle and has no horizon?
	if isNew && !current.IsIdle() {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeOtherOrderActive,
			fmt.Sprintf(
				"Order rejected: there is already an active order %s: %d",
				current.OrderId,
				current.OrderUpdateId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: current.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(current.OrderUpdateId),
			},
		)
	}

	// Check 4 (new only): Is OrderUpdateId == 0?
	if isNew && incoming.OrderUpdateId != 0 {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeInvalidOrder,
			fmt.Sprintf(
				"Order rejected: orderUpdateId for new order must be 0, received %d",
				incoming.OrderUpdateId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: incoming.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(incoming.OrderUpdateId),
			},
		)
	}

	// Check 5 (new only): Is incoming first node within allowedDeviationXY/Theta of robotPos?
	if isNew && !startNodeInRange(incoming, robotPos) {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeStartNodeOutOfRange,
			"Order rejected: first node is not within allowed deviation of robot position",
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: incoming.OrderId},
		)
	}
	// Check 6 (update only): Is incoming OrderUpdateId <= current OrderUpdateId?
	if !isNew && incoming.OrderUpdateId < current.OrderUpdateId {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeOutdatedOrderUpdate,
			fmt.Sprintf(
				"Order update rejected: order %s is currently at update %d, but received update %d",
				current.OrderId, current.OrderUpdateId, incoming.OrderUpdateId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: current.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(current.OrderUpdateId),
			},
		)
	}

	// Check 7 (update only): Is current Cancelled?
	if !isNew && current.Cancelled {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeOrderUpdateFollowingCancel,
			fmt.Sprintf(
				"Order update rejected: order %s was cancelled and can no longer be updated",
				current.OrderId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: current.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(incoming.OrderUpdateId),
			},
		)
	}

	// Check 8 (update only): Is incoming OrderUpdateId == current OrderUpdateId?
	if !isNew && incoming.OrderUpdateId == current.OrderUpdateId {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeSameOrderUpdateId,
			fmt.Sprintf(
				"Order update rejected: order %s is currently at update %d, but received update %d",
				current.OrderId, current.OrderUpdateId, incoming.OrderUpdateId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: current.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(current.OrderUpdateId),
			},
		)
	}

	// Check 9 and 10 (update only): Is incoming first node should equal current Decision Node
	if !isNew && (incoming.Nodes[0].NodeId != current.DecisionNodeId ||
		incoming.Nodes[0].SequenceId != current.DecisionSequenceId) {
		return false, vdaerrors.New(
			vdaerrors.ErrorTypeInvalidOrder,
			fmt.Sprintf(
				"Order update rejected: order %s has invalid first node",
				current.OrderId,
			),
			vdaerrors.ErrorReference{ReferenceKey: "orderId", ReferenceValue: current.OrderId},
			vdaerrors.ErrorReference{
				ReferenceKey:   "orderUpdateId",
				ReferenceValue: strconv.Itoa(current.OrderUpdateId),
			},
			vdaerrors.ErrorReference{
				ReferenceKey:   "currentNodeId",
				ReferenceValue: current.DecisionNodeId,
			},
			vdaerrors.ErrorReference{
				ReferenceKey:   "incomingNodeId",
				ReferenceValue: incoming.Nodes[0].NodeId,
			},
		)
	}

	// Check 11: Accept, append the new base/horizon nodes/edges to current, replacing the
	// old horizon, and advance DecisionNodeId/DecisionSequenceId to the new decision point
	accept(current, incoming, isNew)

	// Accepted
	return true, nil
}
