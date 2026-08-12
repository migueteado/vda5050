package main

import (
	"encoding/json"
	"log"
	"math/rand"
	"time"

	"vda5050/common"
	"vda5050/common/geometry"
	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	demoManufacturer = "KIT"
	demoSerialNumber = "0001"
)

func demoNodePosition(x, y float64) geometry.NodePosition {
	return geometry.NodePosition{
		Position: geometry.Position{X: x, Y: y, Theta: 0},
		MapId:    "floor1",
		AllowedDeviationXY: geometry.AllowedDeviationXY{
			A: 0.1, B: 0.1, Theta: 0,
		},
		AllowedDeviationTheta: 0.1,
	}
}

// baseOrder releases n_A, n_B, n_C, holding n_D and n_E as horizon.
// The decision point is n_C (sequenceId 4).
func baseOrder() models.Order {
	return models.Order{
		OrderId:       "demo-order-1",
		OrderUpdateId: 0,
		Nodes: []models.Node{
			{NodeId: "n_A", SequenceId: 0, Released: true, NodePosition: demoNodePosition(0, 0)},
			{NodeId: "n_B", SequenceId: 2, Released: true, NodePosition: demoNodePosition(2, 0)},
			{NodeId: "n_C", SequenceId: 4, Released: true, NodePosition: demoNodePosition(4, 1)},
			{NodeId: "n_D", SequenceId: 6, Released: false, NodePosition: demoNodePosition(6, 1)},
			{NodeId: "n_E", SequenceId: 8, Released: false, NodePosition: demoNodePosition(8, 0)},
		},
		Edges: []models.Edge{
			{EdgeId: "e_1", SequenceId: 1, StartNodeId: "n_A", EndNodeId: "n_B", Released: true, MaximumSpeed: 1.0},
			{EdgeId: "e_2", SequenceId: 3, StartNodeId: "n_B", EndNodeId: "n_C", Released: true, MaximumSpeed: 1.0},
			{EdgeId: "e_3", SequenceId: 5, StartNodeId: "n_C", EndNodeId: "n_D", Released: false, MaximumSpeed: 1.0},
			{EdgeId: "e_4", SequenceId: 7, StartNodeId: "n_D", EndNodeId: "n_E", Released: false, MaximumSpeed: 1.0},
		},
	}
}

// goodUpdate stitches onto n_C (the base order's decision point),
// releasing n_D and its leading edge. n_E stays horizon; the new
// decision point becomes n_D.
func goodUpdate() models.Order {
	return models.Order{
		OrderId:       "demo-order-1",
		OrderUpdateId: 1,
		Nodes: []models.Node{
			{NodeId: "n_C", SequenceId: 4, Released: true, NodePosition: demoNodePosition(4, 1)},
			{NodeId: "n_D", SequenceId: 6, Released: true, NodePosition: demoNodePosition(6, 1)},
			{NodeId: "n_E", SequenceId: 8, Released: false, NodePosition: demoNodePosition(8, 0)},
		},
		Edges: []models.Edge{
			{EdgeId: "e_3", SequenceId: 5, StartNodeId: "n_C", EndNodeId: "n_D", Released: true, MaximumSpeed: 1.0},
			{EdgeId: "e_4", SequenceId: 7, StartNodeId: "n_D", EndNodeId: "n_E", Released: false, MaximumSpeed: 1.0},
		},
	}
}

// badUpdate stitches onto n_B instead of the real decision point
// (n_C), so AcceptOrder must reject it via checks 9/10.
func badUpdate() models.Order {
	return models.Order{
		OrderId:       "demo-order-1",
		OrderUpdateId: 1,
		Nodes: []models.Node{
			{NodeId: "n_B", SequenceId: 2, Released: true, NodePosition: demoNodePosition(2, 0)},
			{NodeId: "n_D", SequenceId: 6, Released: true, NodePosition: demoNodePosition(6, 1)},
		},
		Edges: []models.Edge{
			{EdgeId: "e_x", SequenceId: 3, StartNodeId: "n_B", EndNodeId: "n_D", Released: true, MaximumSpeed: 1.0},
		},
	}
}

func publishOrder(client mqtt.Client, headerGen *common.HeaderGenerator, order models.Order) error {
	topic, err := common.TopicFor(demoManufacturer, demoSerialNumber, common.Order)
	if err != nil {
		return err
	}
	header := headerGen.Generate(string(common.Order), demoManufacturer, demoSerialNumber)
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

// dropRate is the chance an update is swallowed instead of published,
// simulating an unreliable wireless link (spec §6.1.2).
const dropRate = 0.3

// RunDropExperiment publishes the base order, then after a short delay
// publishes the update — unless the random drop fires, in which case
// the update is silently skipped and the robot should be left stopped
// at its decision point, unharmed, until a real update arrives.
func RunDropExperiment(client mqtt.Client, headerGen *common.HeaderGenerator) {
	if err := publishOrder(client, headerGen, baseOrder()); err != nil {
		log.Println("publish base order:", err)
		return
	}
	log.Println("published base order (decision point: n_C)")

	time.Sleep(3 * time.Second)

	if rand.Float64() < dropRate {
		log.Println("update DROPPED in transit (simulated) - robot should stop at n_C and wait")
		return
	}

	if err := publishOrder(client, headerGen, goodUpdate()); err != nil {
		log.Println("publish update:", err)
		return
	}
	log.Println("published update (new decision point: n_D)")
}

// RunFailureExperiment publishes the base order, then an update whose
// first node is not the current decision point. AcceptOrder must
// reject it (checks 9/10) and the robot's state must be untouched.
func RunFailureExperiment(client mqtt.Client, headerGen *common.HeaderGenerator) {
	if err := publishOrder(client, headerGen, baseOrder()); err != nil {
		log.Println("publish base order:", err)
		return
	}
	log.Println("published base order (decision point: n_C)")

	time.Sleep(3 * time.Second)

	if err := publishOrder(client, headerGen, badUpdate()); err != nil {
		log.Println("publish bad update:", err)
		return
	}
	log.Println("published bad update stitched onto n_B instead of n_C - expect rejection")
}
