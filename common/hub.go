package common

import (
	"sync"

	"github.com/gorilla/websocket"
)

type Hub struct {
	mu    sync.Mutex
	conns []*websocket.Conn
	// latest holds the most recent message seen per MQTT topic, so a
	// browser tab that opens after a retained message was delivered
	// still gets the current state on connect.
	latest map[string][]byte
}

func NewHub() *Hub {
	return &Hub{conns: make([]*websocket.Conn, 0), latest: make(map[string][]byte)}
}

// Register adds conn to the hub and immediately replays the latest
// known message for every topic, so the new tab starts caught up.
func (hub *Hub) Register(conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.conns = append(hub.conns, conn)
	for _, msg := range hub.latest {
		conn.WriteMessage(websocket.TextMessage, msg)
	}
}

func (hub *Hub) Unregister(conn *websocket.Conn) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for i, c := range hub.conns {
		if c == conn {
			hub.conns = append(hub.conns[:i], hub.conns[i+1:]...)
			return
		}
	}
}

func (hub *Hub) Broadcast(topic string, msg []byte) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	hub.latest[topic] = msg
	var dead []*websocket.Conn
	for _, conn := range hub.conns {
		if err := conn.WriteMessage(websocket.TextMessage, msg); err != nil {
			dead = append(dead, conn)
		}
	}
	for _, conn := range dead {
		for i, c := range hub.conns {
			if c == conn {
				hub.conns = append(hub.conns[:i], hub.conns[i+1:]...)
				break
			}
		}
	}
}
