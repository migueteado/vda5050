package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"sync"

	"vda5050/common"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/gorilla/websocket"
)

// DropGate lets the control panel arm a one-shot drop of the next
// order/update the viewer is asked to forward, to demo the spec's
// "unreliable channel" resilience (§6.1.2) without touching fleet
// control's own publisher.
type DropGate struct {
	mu    sync.Mutex
	armed bool
}

// Toggle flips the armed state and returns the new value.
func (g *DropGate) Toggle() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	g.armed = !g.armed
	return g.armed
}

// Consume reports whether a drop is armed and disarms it if so.
func (g *DropGate) Consume() bool {
	g.mu.Lock()
	defer g.mu.Unlock()
	if !g.armed {
		return false
	}
	g.armed = false
	return true
}

var upgrader = websocket.Upgrader{
	// Browser and server run on the same host during dev, so no
	// origin check is needed yet.
	CheckOrigin: func(r *http.Request) bool { return true },
}

func onMessage(hub *common.Hub) mqtt.MessageHandler {
	return func(client mqtt.Client, msg mqtt.Message) {
		fmt.Printf("[%s] %s\n", msg.Topic(), msg.Payload())
		hub.Broadcast(msg.Topic(), msg.Payload())
	}
}

func onConnect(hub *common.Hub) mqtt.OnConnectHandler {
	return func(client mqtt.Client) {
		for _, t := range []common.TopicName{common.Connection, common.Visualization, common.State, common.Order} {
			topic, err := common.WildcardFor(string(t))
			if err != nil {
				log.Fatal(err)
			}

			qos := common.QOS[t]
			token := client.Subscribe(topic, byte(qos), onMessage(hub))
			token.Wait()
			if err := token.Error(); err != nil {
				log.Println(err)
				return
			}
		}

		// Not a spec topic - a dev-only channel the robot uses to
		// surface AcceptOrder rejections to the viewer, so the
		// "send bad update" experiment shows the error next to the
		// robot instead of only scrolling past in the robot's log.
		debugTopic := fmt.Sprintf("%s/%s/+/+/debug/rejection", common.INTERFACE_NAME, common.MAJOR_VERSION)
		token := client.Subscribe(debugTopic, 0, onMessage(hub))
		token.Wait()
		if err := token.Error(); err != nil {
			log.Println(err)
		}
	}
}

// orderHandler accepts a raw VDA 5050 order JSON body and publishes it
// on behalf of the robot named in the URL path, e.g.
// POST /order/KIT/0001
// If dropGate has an armed drop, the publish is skipped instead, to
// simulate the update being lost on an unreliable wireless link.
func orderHandler(client mqtt.Client, dropGate *DropGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}

		manufacturer := r.PathValue("manufacturer")
		serialNumber := r.PathValue("serialNumber")

		topic, err := common.TopicFor(manufacturer, serialNumber, common.Order)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		body, err := io.ReadAll(r.Body)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}

		w.Header().Set("Content-Type", "application/json")

		if dropGate.Consume() {
			w.WriteHeader(http.StatusAccepted)
			json.NewEncoder(w).Encode(map[string]bool{"dropped": true})
			return
		}

		qos := common.QOS[common.Order]
		token := client.Publish(topic, byte(qos), false, body)
		token.Wait()
		if err := token.Error(); err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		w.WriteHeader(http.StatusAccepted)
		json.NewEncoder(w).Encode(map[string]bool{"dropped": false})
	}
}

// dropToggleHandler flips the drop gate's armed state and reports it.
func dropToggleHandler(dropGate *DropGate) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		armed := dropGate.Toggle()
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]bool{"armed": armed})
	}
}

func wsHandler(hub *common.Hub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			log.Println(err)
			return
		}
		defer conn.Close()

		hub.Register(conn)
		defer hub.Unregister(conn)

		log.Println("browser connected")

		// Keep the connection open. Reading discards frames from the
		// browser for now; it also detects when the browser disconnects.
		for {
			if _, _, err := conn.ReadMessage(); err != nil {
				log.Println("browser disconnected:", err)
				return
			}
		}
	}
}

func main() {
	hub := common.NewHub()

	opts := mqtt.NewClientOptions()
	opts.AddBroker("tcp://localhost:1883")
	opts.SetClientID("viewer")
	opts.SetOnConnectHandler(onConnect(hub))

	client := mqtt.NewClient(opts)
	token := client.Connect()
	token.Wait()
	if err := token.Error(); err != nil {
		log.Fatal(err)
	}

	dropGate := &DropGate{}

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.Dir("viewer/static")))
	mux.HandleFunc("/ws", wsHandler(hub))
	mux.HandleFunc("POST /order/{manufacturer}/{serialNumber}", orderHandler(client, dropGate))
	mux.HandleFunc("POST /drop-toggle", dropToggleHandler(dropGate))

	log.Println("viewer listening on :8080")
	if err := http.ListenAndServe(":8080", mux); err != nil {
		log.Fatal(err)
	}
}
