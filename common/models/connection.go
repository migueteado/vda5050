package models

import "vda5050/common"

type ConnectionState string

const (
	Online ConnectionState = "ONLINE"
	Offline ConnectionState = "OFFLINE"
	Hibernating ConnectionState = "HIBERNATING"
	ConnectionBroken ConnectionState = "CONNECTION_BROKEN"
)

type Connection struct {
	common.Header
	ConnectionState ConnectionState `json:"connectionState"`
}