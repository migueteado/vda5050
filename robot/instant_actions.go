package main

import (
	"encoding/json"
	"log"
	"time"

	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// actionTickRate is how often the action queue is advanced - same
// cadence as the simulator's drive ticks, since the drive gate reads
// DrivingAllowed() every tick too.
const actionTickRate = 100 * time.Millisecond

// runActionEngine advances robot's action queue on a tick, marking the
// state dirty only when something about the reported action states
// actually changed - the same coalescing idea as StatePublisher, just
// applied to actionStates instead of the whole message.
func runActionEngine(robot *Robot, statePublisher *StatePublisher) {
	ticker := time.NewTicker(actionTickRate)
	defer ticker.Stop()

	var last []models.ActionState
	for range ticker.C {
		robot.actions.Advance()
		states := robot.actions.ActionStates()
		if !equalActionStates(states, last) {
			statePublisher.MarkDirty()
			last = states
		}
	}
}

func equalActionStates(a, b []models.ActionState) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i].ActionId != b[i].ActionId || a[i].ActionStatus != b[i].ActionStatus {
			return false
		}
	}
	return true
}

// onInstantActions handles the mandatory minimum (cancelOrder,
// startPause, stopPause) plus retry/skipRetry, so item 6's RETRIABLE
// demo has a way back to FINISHED. Unsupported types are rejected per
// spec §6.2.1: INVALID_INSTANT_ACTION, WARNING, actionId as reference.
func onInstantActions(robot *Robot, statePublisher *StatePublisher) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		var incoming models.InstantActions
		if err := json.Unmarshal(msg.Payload(), &incoming); err != nil {
			log.Println("instant action unmarshal:", err)
			return
		}

		for _, action := range incoming.Actions {
			applyInstantAction(robot, action)
		}
		statePublisher.MarkDirty()
	}
}

// keepNodesUpTo drops every node past lastSeq - used by cancelOrder to
// clear the horizon/future base, so nodeStates reports empty (spec
// §6.1.3).
func keepNodesUpTo(nodes []models.Node, lastSeq int) []models.Node {
	kept := make([]models.Node, 0, len(nodes))
	for _, n := range nodes {
		if n.SequenceId <= lastSeq {
			kept = append(kept, n)
		}
	}
	return kept
}

func keepEdgesUpTo(edges []models.Edge, lastSeq int) []models.Edge {
	kept := make([]models.Edge, 0, len(edges))
	for _, e := range edges {
		if e.SequenceId <= lastSeq {
			kept = append(kept, e)
		}
	}
	return kept
}

// stringParam looks up a string-valued actionParameter by key (spec
// Table 4: retry/skipRetry carry their target as an "actionId"
// parameter, not as the instant action's own actionId).
func stringParam(action models.Action, key string) (string, bool) {
	for _, p := range action.ActionParameters {
		if p.Key == key {
			if s, ok := p.Value.(string); ok {
				return s, true
			}
		}
	}
	return "", false
}

func applyInstantAction(robot *Robot, action models.Action) {
	robot.mu.Lock()
	defer robot.mu.Unlock()

	state := models.ActionState{
		ActionId:     action.ActionId,
		ActionType:   action.ActionType,
		ActionStatus: models.ActionStatusFinished,
	}

	switch action.ActionType {
	case "cancelOrder":
		robot.state.Cancelled = true
		robot.actions.CancelAll()
		robot.state.Nodes = keepNodesUpTo(robot.state.Nodes, robot.state.LastNodeSequenceId)
		robot.state.Edges = keepEdgesUpTo(robot.state.Edges, robot.state.LastNodeSequenceId)
		if robot.simulator != nil {
			robot.simulator.Extend(robot.state.Nodes, robot.state.Edges)
		}

	case "startPause":
		robot.state.Paused = true

	case "stopPause":
		robot.state.Paused = false

	case "retry":
		targetId, ok := stringParam(action, "actionId")
		if !ok {
			state.ActionStatus = models.ActionStatusFailed
			state.ActionResult = "retry requires an actionId parameter"
			break
		}
		if err := robot.actions.Retry(targetId); err != nil {
			state.ActionStatus = models.ActionStatusFailed
			state.ActionResult = err.Error()
		}

	case "skipRetry":
		targetId, ok := stringParam(action, "actionId")
		if !ok {
			state.ActionStatus = models.ActionStatusFailed
			state.ActionResult = "skipRetry requires an actionId parameter"
			break
		}
		if err := robot.actions.SkipRetry(targetId); err != nil {
			state.ActionStatus = models.ActionStatusFailed
			state.ActionResult = err.Error()
		}

	default:
		state.ActionStatus = models.ActionStatusFailed
		state.ActionResult = "unsupported instant action: " + action.ActionType
	}

	robot.instantActionStates = append(robot.instantActionStates, state)
}
