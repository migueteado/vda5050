package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vda5050/common"
	"vda5050/common/models"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func onMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("[%s] %s\n", msg.Topic(), msg.Payload())
}

func onConnect(headerGen *common.HeaderGenerator, experiment string, baseRequests *BaseRequestWatcher) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		topic, err := common.WildcardFor(string(common.State))
		if err != nil {
			log.Fatal(err)
		}

		qos := common.QOS[common.State]
		token := client.Subscribe(topic, byte(qos), baseRequests.OnState)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
			return
		}

		topic, err = common.WildcardFor(string(common.Connection))
		if err != nil {
			log.Fatal(err)
		}

		qos = common.QOS[common.Connection]
		token = client.Subscribe(topic, byte(qos), onMessage)
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
			return
		}

		switch experiment {
		case "drop":
			go RunDropExperiment(client, headerGen)
		case "failure":
			go RunFailureExperiment(client, headerGen)
		case "dispatch":
			nodes, edges := demoRoute(10)
			dispatcher := NewDispatcher(demoManufacturer, demoSerialNumber, "demo-route-1", nodes, edges)
			baseRequests.OnRequest = func(state models.State) {
				if err := dispatcher.ExtendIfNeeded(client, headerGen, state.OrderId); err != nil {
					log.Println("dispatcher extend:", err)
				}
			}
			if err := dispatcher.Start(client, headerGen); err != nil {
				log.Println("dispatcher start:", err)
			}
		}
	}
}

func main() {
	experiment := flag.String("experiment", "none", "demo experiment to run: none, drop, failure, dispatch")
	flag.Parse()

	headerGen := common.NewHeaderGenerator()
	baseRequests := NewBaseRequestWatcher()

	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
	opts.SetClientID("fleet")
	opts.SetOnConnectHandler(onConnect(headerGen, *experiment, baseRequests))

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)
	<-sigCh

	client.Disconnect(250)
}
