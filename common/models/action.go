package models

type BlockingType string

const (
	BlockingTypeNone   BlockingType = "NONE"
	BlockingTypeSingle BlockingType = "SINGLE"
	BlockingTypeSoft   BlockingType = "SOFT"
	BlockingTypeHard   BlockingType = "HARD"
)

type ActionParameter struct {
	Key   string      `json:"key"`
	Value interface{} `json:"value"`
}

type Action struct {
	ActionType       string            `json:"actionType"`
	ActionId         string            `json:"actionId"`
	ActionDescriptor string            `json:"actionDescriptor"`
	BlockingType     BlockingType      `json:"blockingType"`
	ActionParameters []ActionParameter `json:"actionParameters"`
	Retriable        bool              `json:"retriable"`
}
