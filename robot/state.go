package main

import (
	"encoding/json"
	"log"
	"sync"
	"time"

	"vda5050/common"
	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// statePublishInterval stands in for the factsheet's
// protocolLimits.timing.minimumStateInterval (Session 7) - once that
// exists it replaces this constant.
const statePublishInterval = 100 * time.Millisecond

// stateHeartbeat is the spec's "at least every 30s, even if nothing
// changed" floor (§6.6).
const stateHeartbeat = 30 * time.Second

// StatePublisher coalesces state changes: MarkDirty records that
// something reportable happened, and the ticker loop publishes once
// per tick if dirty, or unconditionally once the heartbeat elapses.
// Driving over a node changing lastNodeId, nodeStates and actionStates
// all in the same tick still produces exactly one state message.
type StatePublisher struct {
	mu          sync.Mutex
	dirty       bool
	lastPublish time.Time
}

// MarkDirty records that a reportable field changed since the last
// publish.
func (p *StatePublisher) MarkDirty() {
	p.mu.Lock()
	p.dirty = true
	p.mu.Unlock()
}

// due reports whether a publish is owed right now, and if so resets
// the dirty flag and publish clock as if it already happened.
func (p *StatePublisher) due() bool {
	p.mu.Lock()
	defer p.mu.Unlock()

	if !p.dirty && time.Since(p.lastPublish) < stateHeartbeat {
		return false
	}
	p.dirty = false
	p.lastPublish = time.Now()
	return true
}

// Run blocks forever, publishing a State message for robot whenever
// due() says one is owed.
func (p *StatePublisher) Run(client mqtt.Client, headerGen *common.HeaderGenerator, robot *Robot) {
	ticker := time.NewTicker(statePublishInterval)
	defer ticker.Stop()

	for range ticker.C {
		if !p.due() {
			continue
		}

		state := buildState(headerGen, robot)
		payload, err := json.Marshal(state)
		if err != nil {
			log.Println(err)
			continue
		}

		topic, err := common.TopicFor(Manufacturer, SerialNumber, common.State)
		if err != nil {
			log.Println(err)
			continue
		}
		client.Publish(topic, byte(common.QOS[common.State]), false, payload)
	}
}

// buildState assembles the current State message from the robot's
// order state and simulator. nodeStates/edgeStates are recomputed
// fresh each time from OrderState - filtering out anything at or
// behind LastNodeSequenceId - so they shrink automatically as the
// robot progresses, with no separate array to maintain (spec §6.6.1:
// the first/current node is never included, only what remains).
func buildState(headerGen *common.HeaderGenerator, robot *Robot) models.State {
	robot.mu.Lock()
	defer robot.mu.Unlock()

	var nodeStates []models.NodeState
	for _, n := range robot.state.Nodes {
		if n.SequenceId <= robot.state.LastNodeSequenceId {
			continue
		}
		nodeStates = append(nodeStates, models.NodeState{
			NodeId:         n.NodeId,
			SequenceId:     n.SequenceId,
			NodeDescriptor: n.NodeDescriptor,
			Released:       n.Released,
			NodePosition:   n.NodePosition,
		})
	}

	var edgeStates []models.EdgeState
	for _, e := range robot.state.Edges {
		if e.SequenceId <= robot.state.LastNodeSequenceId {
			continue
		}
		edgeStates = append(edgeStates, models.EdgeState{
			EdgeId:     e.EdgeId,
			SequenceId: e.SequenceId,
			Released:   e.Released,
			Trajectory: e.Trajectory,
		})
	}

	var driving bool
	if robot.simulator != nil {
		driving = robot.simulator.Driving()
	}

	// newBaseRequest: fewer than 2 released-but-undriven nodes left in
	// the base means fleet control must extend it soon, or the robot
	// will run out of road and have to slow down (spec §6.6.3).
	remainingBaseNodes := 0
	for _, n := range nodeStates {
		if n.Released {
			remainingBaseNodes++
		}
	}
	newBaseRequest := remainingBaseNodes < 2

	header := headerGen.Generate(string(common.State), Manufacturer, SerialNumber)

	return models.State{
		Header:             *header,
		OrderId:            robot.state.OrderId,
		OrderUpdateId:      robot.state.OrderUpdateId,
		LastNodeId:         robot.state.LastNodeId,
		LastNodeSequenceId: robot.state.LastNodeSequenceId,
		NodeStates:         nodeStates,
		EdgeStates:         edgeStates,
		Driving:            driving,
		NewBaseRequest:     newBaseRequest,
		OperatingMode:      models.OperatingModeAutomatic,
		// No battery model yet - permanently-powered stand-in per
		// spec §7.8 ("For permanently powered mobile robots, this
		// field shall be 100").
		PowerSupply: models.PowerSupply{StateOfCharge: 100, Charging: false},
		SafetyState: models.SafetyState{ActiveEmergencyStop: models.EmergencyStopNone},
	}
}
