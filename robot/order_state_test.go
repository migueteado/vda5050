package main

import (
	"testing"

	vdaerrors "vda5050/common/errors"
	"vda5050/common/geometry"
	"vda5050/common/models"
)

func node(id string, seq int, released bool, x, y float64) models.Node {
	return models.Node{
		NodeId:       id,
		SequenceId:   seq,
		Released:     released,
		NodePosition: geometry.NodePosition{Position: geometry.Position{X: x, Y: y}},
	}
}

func edge(id string, seq int, start, end string, released bool) models.Edge {
	return models.Edge{
		EdgeId:      id,
		SequenceId:  seq,
		StartNodeId: start,
		EndNodeId:   end,
		Released:    released,
	}
}

func pos(x, y float64) geometry.Position {
	return geometry.Position{X: x, Y: y}
}

func TestAcceptOrder(t *testing.T) {
	tests := []struct {
		name         string
		current      OrderState
		incoming     models.Order
		robotPos     geometry.Position
		wantAccepted bool
		wantErrType  vdaerrors.ErrorType
		check        func(t *testing.T, s *OrderState)
	}{
		{
			name:    "check 3: new order rejected while another order is active",
			current: OrderState{OrderId: "prev", DecisionSequenceId: 2, LastNodeSequenceId: 0},
			incoming: models.Order{
				OrderId: "new1", OrderUpdateId: 0,
				Nodes: []models.Node{node("n0", 0, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeOtherOrderActive,
		},
		{
			name:    "check 4: new order rejected when orderUpdateId != 0",
			current: OrderState{},
			incoming: models.Order{
				OrderId: "new1", OrderUpdateId: 1,
				Nodes: []models.Node{node("n0", 0, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeInvalidOrder,
		},
		{
			name:    "check 5: new order rejected when start node out of range",
			current: OrderState{},
			incoming: models.Order{
				OrderId: "new1", OrderUpdateId: 0,
				Nodes: []models.Node{node("n0", 0, true, 100, 100)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeStartNodeOutOfRange,
		},
		{
			name: "check 6: update rejected when outdated",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 5,
				DecisionNodeId: "g", DecisionSequenceId: 4,
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 3,
				Nodes: []models.Node{node("g", 4, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeOutdatedOrderUpdate,
		},
		{
			name: "check 7: update rejected after cancel",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 1, Cancelled: true,
				DecisionNodeId: "g", DecisionSequenceId: 4,
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 2,
				Nodes: []models.Node{node("g", 4, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeOrderUpdateFollowingCancel,
		},
		{
			name: "check 8: update rejected when orderUpdateId repeats",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 2,
				DecisionNodeId: "g", DecisionSequenceId: 4,
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 2,
				Nodes: []models.Node{node("g", 4, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeSameOrderUpdateId,
		},
		{
			name: "check 9/10: update rejected when first node id does not match decision node",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 1,
				DecisionNodeId: "g", DecisionSequenceId: 4,
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 2,
				Nodes: []models.Node{node("x", 4, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeInvalidOrder,
		},
		{
			name: "check 9/10: update rejected when first node sequenceId does not match decision point",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 1,
				DecisionNodeId: "g", DecisionSequenceId: 4,
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 2,
				Nodes: []models.Node{node("g", 6, true, 0, 0)},
			},
			robotPos:     pos(0, 0),
			wantAccepted: false,
			wantErrType:  vdaerrors.ErrorTypeInvalidOrder,
		},
		{
			name:    "accept: new order with a horizon",
			current: OrderState{},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 0,
				Nodes: []models.Node{
					node("n0", 0, true, 0, 0),
					node("n2", 2, true, 1, 0),
					node("n4", 4, false, 2, 0),
				},
				Edges: []models.Edge{
					edge("e1", 1, "n0", "n2", true),
					edge("e3", 3, "n2", "n4", false),
				},
			},
			robotPos:     pos(0, 0),
			wantAccepted: true,
			check: func(t *testing.T, s *OrderState) {
				if s.LastNodeId != "n0" || s.LastNodeSequenceId != 0 {
					t.Errorf("LastNode = %s/%d, want n0/0", s.LastNodeId, s.LastNodeSequenceId)
				}
				if s.DecisionNodeId != "n2" || s.DecisionSequenceId != 2 {
					t.Errorf("Decision = %s/%d, want n2/2", s.DecisionNodeId, s.DecisionSequenceId)
				}
				if s.IsIdle() {
					t.Error("robot should not be idle: it has a horizon")
				}
			},
		},
		{
			name: "accept: update stitches onto the in-flight edge and drops the resent decision node",
			current: OrderState{
				OrderId: "o1", OrderUpdateId: 0,
				LastNodeId: "d", LastNodeSequenceId: 2,
				DecisionNodeId: "g", DecisionSequenceId: 4,
				Nodes: []models.Node{
					node("f", 0, true, 0, 0),
					node("d", 2, true, 1, 0),
					node("g", 4, true, 2, 0),
				},
				Edges: []models.Edge{
					edge("e1", 1, "f", "d", true),
					edge("e3", 3, "d", "g", true),
				},
			},
			incoming: models.Order{
				OrderId: "o1", OrderUpdateId: 1,
				Nodes: []models.Node{
					node("g", 4, true, 2, 0),
					node("b", 6, true, 3, 0),
					node("h", 8, false, 4, 0),
				},
				Edges: []models.Edge{
					edge("e8", 5, "g", "b", true),
					edge("e9", 7, "b", "h", false),
				},
			},
			robotPos:     pos(1, 0),
			wantAccepted: true,
			check: func(t *testing.T, s *OrderState) {
				wantNodeIds := []string{"d", "g", "b", "h"}
				if len(s.Nodes) != len(wantNodeIds) {
					t.Fatalf("Nodes = %v, want ids %v", s.Nodes, wantNodeIds)
				}
				for i, id := range wantNodeIds {
					if s.Nodes[i].NodeId != id {
						t.Errorf("Nodes[%d].NodeId = %s, want %s", i, s.Nodes[i].NodeId, id)
					}
				}
				wantEdgeIds := []string{"e3", "e8", "e9"}
				if len(s.Edges) != len(wantEdgeIds) {
					t.Fatalf("Edges = %v, want ids %v", s.Edges, wantEdgeIds)
				}
				for i, id := range wantEdgeIds {
					if s.Edges[i].EdgeId != id {
						t.Errorf("Edges[%d].EdgeId = %s, want %s", i, s.Edges[i].EdgeId, id)
					}
				}
				if s.DecisionNodeId != "b" || s.DecisionSequenceId != 6 {
					t.Errorf("Decision = %s/%d, want b/6", s.DecisionNodeId, s.DecisionSequenceId)
				}
				// LastNode is only advanced by the traversal callback, not by accept()
				if s.LastNodeId != "d" || s.LastNodeSequenceId != 2 {
					t.Errorf("LastNode = %s/%d, want unchanged d/2", s.LastNodeId, s.LastNodeSequenceId)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			current := tt.current
			accepted, err := AcceptOrder(&current, tt.incoming, tt.robotPos)

			if accepted != tt.wantAccepted {
				t.Fatalf("accepted = %v, want %v (err: %v)", accepted, tt.wantAccepted, err)
			}

			if !tt.wantAccepted {
				if err == nil {
					t.Fatal("expected an error, got nil")
				}
				if err.ErrorType != tt.wantErrType {
					t.Errorf("ErrorType = %s, want %s", err.ErrorType, tt.wantErrType)
				}
				return
			}

			if err != nil {
				t.Fatalf("unexpected error on accept: %v", err)
			}
			if tt.check != nil {
				tt.check(t, &current)
			}
		})
	}
}
