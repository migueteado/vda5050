package main

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"slices"
	"sync"
	"syscall"

	"vda5050/common"
	vdaerrors "vda5050/common/errors"
	"vda5050/common/geometry"
	"vda5050/common/models"
	"vda5050/common/validations"
	"vda5050/sim"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

const (
	Manufacturer = "KIT"
	SerialNumber = "0001"
)

// Robot holds the one order state and simulator that persist across
// MQTT messages for this robot.
type Robot struct {
	mu        sync.Mutex
	state     OrderState
	simulator *sim.Simulator
}

// position returns where the robot currently is, or the origin if it
// hasn't driven yet.
func (r *Robot) position() geometry.Position {
	r.mu.Lock()
	simulator := r.simulator
	r.mu.Unlock()
	if simulator == nil {
		return geometry.Position{}
	}
	return simulator.Position()
}

// sequenceIdFor looks up a node's sequenceId by nodeId in the robot's
// current known nodes. Must be called with r.mu held.
func (r *Robot) sequenceIdFor(nodeId string) int {
	for _, n := range r.state.Nodes {
		if n.NodeId == nodeId {
			return n.SequenceId
		}
	}
	return -1
}

func onMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("[%s] %s\n", msg.Topic(), msg.Payload())
}

// publishRejection surfaces an AcceptOrder rejection on a dev-only
// debug topic (not part of the spec's message set) so the viewer can
// render it next to the robot instead of it only reaching this log.
func publishRejection(client mqtt.Client, rejection *vdaerrors.VDAError) {
	topic := fmt.Sprintf(
		"%s/%s/%s/%s/debug/rejection",
		common.INTERFACE_NAME, common.MAJOR_VERSION, Manufacturer, SerialNumber,
	)
	payload, err := json.Marshal(rejection)
	if err != nil {
		log.Println(err)
		return
	}
	client.Publish(topic, 0, false, payload)
}

func onOrder(headerGen *common.HeaderGenerator, robot *Robot, statePublisher *StatePublisher) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		order, err := models.Unmarshal(msg)
		if err != nil {
			log.Println(err.Error())
			return
		}
		if err := validations.ValidateOrder(order); err != nil {
			log.Println(err.Error())
			return
		}

		robotPos := robot.position()

		robot.mu.Lock()
		accepted, acceptErr := AcceptOrder(&robot.state, order, robotPos)
		nodes, edges := robot.state.Nodes, robot.state.Edges
		robot.mu.Unlock()

		if !accepted {
			log.Printf("order rejected: %s", acceptErr.Error())
			publishRejection(client, acceptErr)
			return
		}
		statePublisher.MarkDirty()

		vizTopic, topicErr := common.TopicFor(Manufacturer, SerialNumber, common.Visualization)
		if topicErr != nil {
			log.Println(topicErr)
			return
		}
		vizQos := common.QOS[common.Visualization]

		onTick := func(pos geometry.Position, arrivedNodeId string) {
			fmt.Printf("driving: (%.2f, %.2f) theta=%.2f\n", pos.X, pos.Y, pos.Theta)
			if arrivedNodeId != "" {
				fmt.Printf("arrived at %s\n", arrivedNodeId)
				robot.mu.Lock()
				robot.state.LastNodeId = arrivedNodeId
				robot.state.LastNodeSequenceId = robot.sequenceIdFor(arrivedNodeId)
				robot.mu.Unlock()
				statePublisher.MarkDirty()
			}

			header := headerGen.Generate(string(common.Visualization), Manufacturer, SerialNumber)
			viz := models.Visualization{Header: *header}
			viz.AgvPosition.X = pos.X
			viz.AgvPosition.Y = pos.Y
			viz.AgvPosition.Theta = pos.Theta
			viz.AgvPosition.MapId = "floor1"

			payload, err := json.Marshal(viz)
			if err != nil {
				log.Println(err)
				return
			}
			client.Publish(vizTopic, byte(vizQos), false, payload)
		}

		robot.mu.Lock()
		simulator := robot.simulator
		robot.mu.Unlock()

		if simulator == nil {
			simulator = sim.NewSimulator(nodes, edges)
			robot.mu.Lock()
			robot.simulator = simulator
			robot.mu.Unlock()
			go simulator.Run(onTick)
		} else {
			simulator.Extend(nodes, edges)
		}
	}
}

func onConnect(headerGen *common.HeaderGenerator, robot *Robot, statePublisher *StatePublisher) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		topic, err := common.TopicFor(Manufacturer, SerialNumber, common.Order)
		if err != nil {
			log.Println(err)
			return
		}
		qos := common.QOS[common.Order]
		token := client.Subscribe(topic, byte(qos), onOrder(headerGen, robot, statePublisher))
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
			return
		}

		topic, err = common.TopicFor(Manufacturer, SerialNumber, common.InstantActions)
		if err != nil {
			log.Println(err)
			return
		}
		qos = common.QOS[common.InstantActions]
		token = client.Subscribe(topic, byte(qos), onMessage)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
			return
		}

		header := headerGen.Generate(string(common.Connection), Manufacturer, SerialNumber)
		conn := models.Connection{
			Header:          *header,
			ConnectionState: models.Online,
		}
		payload, err := json.Marshal(conn)
		if err != nil {
			log.Println(err)
			return
		}

		topic, err = common.TopicFor(Manufacturer, SerialNumber, common.Connection)
		if err != nil {
			log.Println(err)
			return
		}

		qos = common.QOS[common.Connection]
		retained := slices.Contains(common.RETAINED, common.Connection)

		token = client.Publish(topic, byte(qos), retained, payload)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
		}
	}
}

func shutdown(client mqtt.Client, headerGen *common.HeaderGenerator) {
	header := headerGen.Generate(string(common.Connection), Manufacturer, SerialNumber)
	conn := models.Connection{
		Header:          *header,
		ConnectionState: models.Offline,
	}
	payload, err := json.Marshal(conn)
	if err != nil {
		log.Println(err)
		return
	}

	topic, err := common.TopicFor(Manufacturer, SerialNumber, common.Connection)
	if err != nil {
		log.Println(err)
		return
	}

	qos := common.QOS[common.Connection]
	retained := slices.Contains(common.RETAINED, common.Connection)

	token := client.Publish(topic, byte(qos), retained, payload)
	token.Wait()
	if err := token.Error(); err != nil {
		log.Println(err)
	}

	client.Disconnect(250)
}

func main() {
	headerGen := common.NewHeaderGenerator()
	robot := &Robot{}
	statePublisher := &StatePublisher{}
	header := headerGen.Generate(string(common.Connection), Manufacturer, SerialNumber)
	conn := models.Connection{
		Header:          *header,
		ConnectionState: models.ConnectionBroken,
	}
	payload, err := json.Marshal(conn)
	if err != nil {
		log.Fatal(err)
	}

	topic, err := common.TopicFor(Manufacturer, SerialNumber, common.Connection)
	if err != nil {
		log.Fatal(err)
	}

	qos := common.QOS[common.Connection]
	retained := slices.Contains(common.RETAINED, common.Connection)

	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
	opts.SetClientID("robot")
	opts.SetWill(topic, string(payload), byte(qos), retained)
	opts.SetOnConnectHandler(onConnect(headerGen, robot, statePublisher))

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("connected")

	go statePublisher.Run(client, headerGen, robot)

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	shutdown(client, headerGen)
}
