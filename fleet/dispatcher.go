package main

import (
	"encoding/json"
	"fmt"
	"log"
	"sync"

	"vda5050/common"
	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

// releaseWindow is how many additional nodes the dispatcher releases
// each time it extends the base - the resource-constrained-robot
// reason for splitting an order at all (spec §6.1.2).
const releaseWindow = 3

// Dispatcher holds a full route - potentially far longer than any
// single base should be - and drip-feeds it to one robot in windows,
// extending the base whenever that robot's State reports
// newBaseRequest. nodes[i] and edges[i] are a fixed pairing: edges[i]
// connects nodes[i] to nodes[i+1], same invariant order_state.go and
// sim.Simulator already rely on. sequenceIds must already be assigned
// on nodes/edges before construction (even for nodes, odd for edges)
// since a node's sequenceId never changes once decided, released or
// not.
type Dispatcher struct {
	mu           sync.Mutex
	manufacturer string
	serialNumber string
	orderId      string
	nextUpdateId int
	nodes        []models.Node
	edges        []models.Edge
	// releasedIdx is the index into nodes of the last released node
	// (the decision point); -1 before Start.
	releasedIdx int
}

func NewDispatcher(manufacturer, serialNumber, orderId string, nodes []models.Node, edges []models.Edge) *Dispatcher {
	return &Dispatcher{
		manufacturer: manufacturer,
		serialNumber: serialNumber,
		orderId:      orderId,
		nodes:        nodes,
		edges:        edges,
		releasedIdx:  -1,
	}
}

// Start releases the first window as a brand-new order.
func (d *Dispatcher) Start(client mqtt.Client, headerGen *common.HeaderGenerator) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	end := d.clampEnd(releaseWindow - 1)
	d.release(0, end)
	order := d.buildOrder(0, end, 0)
	d.releasedIdx = end
	d.nextUpdateId = 1

	log.Printf("dispatcher %s: released nodes[0..%d]", d.orderId, end)
	return d.publish(client, headerGen, order)
}

// ExtendIfNeeded reacts to a newBaseRequest for this dispatcher's own
// orderId: if there's more route left, it stitches an update onto the
// current decision point and releases the next window.
func (d *Dispatcher) ExtendIfNeeded(client mqtt.Client, headerGen *common.HeaderGenerator, orderId string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	if orderId != d.orderId {
		return nil
	}
	if d.releasedIdx >= len(d.nodes)-1 {
		log.Printf("dispatcher %s: route fully released, nothing to extend", d.orderId)
		return nil
	}

	start := d.releasedIdx // the current decision point - resent identically
	end := d.clampEnd(d.releasedIdx + releaseWindow)
	d.release(start+1, end)

	order := d.buildOrder(start, end, d.nextUpdateId)
	d.releasedIdx = end
	d.nextUpdateId++

	log.Printf("dispatcher %s: extended base, released nodes[%d..%d]", d.orderId, start+1, end)
	return d.publish(client, headerGen, order)
}

func (d *Dispatcher) clampEnd(end int) int {
	if end > len(d.nodes)-1 {
		return len(d.nodes) - 1
	}
	return end
}

// release marks nodes[from..to] and the edges connecting them (down
// to the node just before from, so the edge leading into the new
// window is released too) as base.
func (d *Dispatcher) release(from, to int) {
	for i := from; i <= to; i++ {
		d.nodes[i].Released = true
	}
	edgeFrom := from - 1
	if edgeFrom < 0 {
		edgeFrom = 0
	}
	for i := edgeFrom; i < to; i++ {
		d.edges[i].Released = true
	}
}

// buildOrder returns the message to send: only nodes[start..end] and
// the edges between them - never anything before start, since that's
// either already-executed base (mustn't be resent) or, on the first
// call, simply the start of the order.
func (d *Dispatcher) buildOrder(start, end, updateId int) models.Order {
	nodes := make([]models.Node, end-start+1)
	copy(nodes, d.nodes[start:end+1])

	var edges []models.Edge
	if end > start {
		edges = make([]models.Edge, end-start)
		copy(edges, d.edges[start:end])
	}

	return models.Order{
		OrderId:       d.orderId,
		OrderUpdateId: updateId,
		Nodes:         nodes,
		Edges:         edges,
	}
}

// demoRoute builds a straight-line route of n nodes (all unreleased),
// long enough to need several dispatcher extensions at the default
// releaseWindow.
func demoRoute(n int) ([]models.Node, []models.Edge) {
	nodes := make([]models.Node, n)
	for i := 0; i < n; i++ {
		nodes[i] = models.Node{
			NodeId:       fmt.Sprintf("r_%d", i),
			SequenceId:   2 * i,
			Released:     false,
			NodePosition: demoNodePosition(float64(i)*2, 0),
		}
	}

	edges := make([]models.Edge, n-1)
	for i := 0; i < n-1; i++ {
		edges[i] = models.Edge{
			EdgeId:       fmt.Sprintf("re_%d", i),
			SequenceId:   2*i + 1,
			StartNodeId:  nodes[i].NodeId,
			EndNodeId:    nodes[i+1].NodeId,
			Released:     false,
			MaximumSpeed: 1.0,
		}
	}

	return nodes, edges
}

func (d *Dispatcher) publish(client mqtt.Client, headerGen *common.HeaderGenerator, order models.Order) error {
	topic, err := common.TopicFor(d.manufacturer, d.serialNumber, common.Order)
	if err != nil {
		return err
	}
	header := headerGen.Generate(string(common.Order), d.manufacturer, d.serialNumber)
	order.Header = *header

	payload, err := json.Marshal(order)
	if err != nil {
		return err
	}

	qos := common.QOS[common.Order]
	token := client.Publish(topic, byte(qos), false, payload)
	token.Wait()
	return token.Error()
}
