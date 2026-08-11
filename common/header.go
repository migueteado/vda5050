package common

import (
	"sync"
	"time"
)

const VERSION string = "3.0.0"

type Header struct {
	HeaderId     uint32 `json:"headerId"`
	Timestamp    string `json:"timestamp"`
	Version      string `json:"version"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serialNumber"`
}

type HeaderGenerator struct {
	mu sync.Mutex
	id map[string]uint32
}

func NewHeaderGenerator() *HeaderGenerator {
	return &HeaderGenerator{id: make(map[string]uint32)}
}

func (hg *HeaderGenerator) Next(topic string) uint32 {
	hg.mu.Lock()
	defer hg.mu.Unlock()
	hg.id[topic]++
	return hg.id[topic]
}

func (hg *HeaderGenerator) Generate(topic, manufacturer, serialNumber string) *Header {
	return &Header{
		HeaderId:     hg.Next(topic),
		Timestamp:    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Version:      VERSION,
		Manufacturer: manufacturer,
		SerialNumber: serialNumber,
	}
}
