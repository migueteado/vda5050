package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"

	"vda5050/common"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

func onMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("[%s] %s\n", msg.Topic(), msg.Payload())
}

func onConnect() mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		topic, err := common.WildcardFor(string(common.State))
		if err != nil {
			log.Fatal(err)
		}

		qos := common.QOS[common.State]
		token := client.Subscribe(topic, byte(qos), onMessage)
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
	}
}

func main() {
	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
	opts.SetClientID("fleet")
	opts.SetOnConnectHandler(onConnect())

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
