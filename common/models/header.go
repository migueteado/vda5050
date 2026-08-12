package models

type Header struct {
	HeaderId     uint32 `json:"headerId"`
	Timestamp    string `json:"timestamp"`
	Version      string `json:"version"`
	Manufacturer string `json:"manufacturer"`
	SerialNumber string `json:"serialNumber"`
}
