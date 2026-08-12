package models

type ConnectionState string

const (
	Online           ConnectionState = "ONLINE"
	Offline          ConnectionState = "OFFLINE"
	Hibernating      ConnectionState = "HIBERNATING"
	ConnectionBroken ConnectionState = "CONNECTION_BROKEN"
)

type Connection struct {
	Header
	ConnectionState ConnectionState `json:"connectionState"`
}
