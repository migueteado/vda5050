package main

import (
	"encoding/json"
	"log"
	"sync"

	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// BaseRequestWatcher reacts to newBaseRequest edges (false -> true)
// across every robot's State stream - the spec's one piece of upward
// flow control (§6.6.3): the robot signals it's about to run out of
// released road, and fleet control is the one that must act on it.
type BaseRequestWatcher struct {
	mu      sync.Mutex
	pending map[string]bool

	// OnRequest, if set, is called after logging, on every false ->
	// true edge. Wire a Dispatcher's ExtendIfNeeded here to actually
	// react instead of just observing.
	OnRequest func(state models.State)
}

func NewBaseRequestWatcher() *BaseRequestWatcher {
	return &BaseRequestWatcher{pending: make(map[string]bool)}
}

// OnState is an MQTT handler: subscribe it to the State wildcard topic.
func (w *BaseRequestWatcher) OnState(client mqtt.Client, msg mqtt.Message) {
	var state models.State
	if err := json.Unmarshal(msg.Payload(), &state); err != nil {
		log.Println("state unmarshal:", err)
		return
	}

	robotId := state.Manufacturer + "/" + state.SerialNumber

	w.mu.Lock()
	wasPending := w.pending[robotId]
	w.pending[robotId] = state.NewBaseRequest
	w.mu.Unlock()

	if state.NewBaseRequest && !wasPending {
		log.Printf(
			"newBaseRequest from %s: orderId=%s lastNodeId=%s - extend the base",
			robotId, state.OrderId, state.LastNodeId,
		)
		if w.OnRequest != nil {
			w.OnRequest(state)
		}
	}
}
