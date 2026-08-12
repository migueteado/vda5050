package models

import (
	vdaerrors "vda5050/common/errors"
	"vda5050/common/geometry"
)

type OperatingMode string

const (
	OperatingModeStartup       OperatingMode = "STARTUP"
	OperatingModeAutomatic     OperatingMode = "AUTOMATIC"
	OperatingModeSemiautomatic OperatingMode = "SEMIAUTOMATIC"
	OperatingModeIntervened    OperatingMode = "INTERVENED"
	OperatingModeManual        OperatingMode = "MANUAL"
	OperatingModeService       OperatingMode = "SERVICE"
	OperatingModeTeachIn       OperatingMode = "TEACH_IN"
)

type ActionStatus string

const (
	ActionStatusWaiting      ActionStatus = "WAITING"
	ActionStatusInitializing ActionStatus = "INITIALIZING"
	ActionStatusRunning      ActionStatus = "RUNNING"
	ActionStatusPaused       ActionStatus = "PAUSED"
	ActionStatusRetriable    ActionStatus = "RETRIABLE"
	ActionStatusFinished     ActionStatus = "FINISHED"
	ActionStatusFailed       ActionStatus = "FAILED"
)

type EmergencyStop string

const (
	EmergencyStopManual EmergencyStop = "MANUAL"
	EmergencyStopRemote EmergencyStop = "REMOTE"
	EmergencyStopNone   EmergencyStop = "NONE"
)

type InfoLevel string

const (
	InfoLevelDebug InfoLevel = "DEBUG"
	InfoLevelInfo  InfoLevel = "INFO"
)

type NodeState struct {
	NodeId         string                `json:"nodeId"`
	SequenceId     int                   `json:"sequenceId"`
	NodeDescriptor string                `json:"nodeDescriptor"`
	Released       bool                  `json:"released"`
	NodePosition   geometry.NodePosition `json:"nodePosition"`
}

type EdgeState struct {
	EdgeId         string     `json:"edgeId"`
	SequenceId     int        `json:"sequenceId"`
	EdgeDescriptor string     `json:"edgeDescriptor"`
	Released       bool       `json:"released"`
	Trajectory     Trajectory `json:"trajectory"`
}

type ActionState struct {
	ActionId         string       `json:"actionId"`
	ActionType       string       `json:"actionType"`
	ActionDescriptor string       `json:"actionDescriptor"`
	ActionStatus     ActionStatus `json:"actionStatus"`
	ActionResult     string       `json:"actionResult"`
}

type PowerSupply struct {
	StateOfCharge  float64 `json:"stateOfCharge"`
	BatteryVoltage float64 `json:"batteryVoltage"`
	BatteryCurrent float64 `json:"batteryCurrent"`
	BatteryHealth  int8    `json:"batteryHealth"`
	Charging       bool    `json:"charging"`
	Range          uint32  `json:"range"`
}

type SafetyState struct {
	ActiveEmergencyStop EmergencyStop `json:"activeEmergencyStop"`
	FieldViolation      bool          `json:"fieldViolation"`
}

type MobileRobotPosition struct {
	geometry.Position
	MapId             string  `json:"mapId"`
	Localized         bool    `json:"localized"`
	LocalizationScore float64 `json:"localizationScore"`
	DeviationRange    float64 `json:"deviationRange"`
}

type Velocity struct {
	Vx    float64 `json:"vx"`
	Vy    float64 `json:"vy"`
	Omega float64 `json:"omega"`
}

type Info struct {
	InfoType       string    `json:"infoType"`
	InfoDescriptor string    `json:"infoDescriptor"`
	InfoLevel      InfoLevel `json:"infoLevel"`
}

type State struct {
	Header
	OrderId               string                `json:"orderId"`
	OrderUpdateId         int                   `json:"orderUpdateId"`
	LastNodeId            string                `json:"lastNodeId"`
	LastNodeSequenceId    int                   `json:"lastNodeSequenceId"`
	NodeStates            []NodeState           `json:"nodeStates"`
	EdgeStates            []EdgeState           `json:"edgeStates"`
	MobileRobotPosition   *MobileRobotPosition  `json:"mobileRobotPosition"`
	Velocity              Velocity              `json:"velocity"`
	Driving               bool                  `json:"driving"`
	Paused                bool                  `json:"paused"`
	NewBaseRequest        bool                  `json:"newBaseRequest"`
	DistanceSinceLastNode float64               `json:"distanceSinceLastNode"`
	ActionStates          []ActionState         `json:"actionStates"`
	InstantActionStates   []ActionState         `json:"instantActionStates"`
	ZoneActionStates      []ActionState         `json:"zoneActionStates"`
	PowerSupply           PowerSupply           `json:"powerSupply"`
	OperatingMode         OperatingMode         `json:"operatingMode"`
	Errors                []*vdaerrors.VDAError `json:"errors"`
	Information           []Info                `json:"information"`
	SafetyState           SafetyState           `json:"safetyState"`
}
