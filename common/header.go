package common

import (
	"sync"
	"time"
	"vda5050/common/models"
)

const VERSION string = "3.0.0"

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

func (hg *HeaderGenerator) Generate(topic, manufacturer, serialNumber string) *models.Header {
	return &models.Header{
		HeaderId:     hg.Next(topic),
		Timestamp:    time.Now().UTC().Format("2006-01-02T15:04:05.000Z"),
		Version:      VERSION,
		Manufacturer: manufacturer,
		SerialNumber: serialNumber,
	}
}
