package main 

import (
	mqtt "github.com/eclipse/paho.mqtt.golang"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"slices"
	"log"
	"vda5050/common"
	"vda5050/common/models"
)

const (
	Manufacturer = "KIT"
	SerialNumber = "0001"
)

func onMessage(client mqtt.Client, msg mqtt.Message) {
	fmt.Printf("[%s] %s\n", msg.Topic(), msg.Payload())
}

func onConnect(headerGen *common.HeaderGenerator) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		topic, err := common.TopicFor(Manufacturer, SerialNumber, common.Order)
		if err != nil {
			log.Println(err)
			return
		}
		qos := common.QOS[common.Order]
		token := client.Subscribe(topic, byte(qos), onMessage)
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
			Header: *header,
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
		Header: *header,
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
	header := headerGen.Generate(string(common.Connection), Manufacturer, SerialNumber)
	conn := models.Connection{
		Header: *header,
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
	opts.SetOnConnectHandler(onConnect(headerGen))

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}
	fmt.Println("connected")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM) 
	<- sigCh

	shutdown(client, headerGen)
}