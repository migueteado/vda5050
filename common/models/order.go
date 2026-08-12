package models

import (
	"encoding/json"

	vdaerrors "vda5050/common/errors"
	"vda5050/common/geometry"

	mqtt "github.com/eclipse/paho.mqtt.golang"
)

type OrientationType string

const (
	ORIENTATION_TANGENTIAL OrientationType = "TANGENTIAL"
	ORIENTATION_GLOBAL     OrientationType = "GLOBAL"
)

type Node struct {
	NodeId         string                `json:"nodeId"`
	SequenceId     int                   `json:"sequenceId"`
	NodeDescriptor string                `json:"nodeDescriptor"`
	Released       bool                  `json:"released"`
	NodePosition   geometry.NodePosition `json:"nodePosition"`
	Actions        []Action              `json:"actions"`
}

type Trajectory struct {
	Degree        int        `json:"degree"`
	KnotVector    []struct{} `json:"knotVector"`
	ControlPoints []struct{} `json:"controlPoints"`
}

type Edge struct {
	EdgeId                   string          `json:"edgeId"`
	SequenceId               int             `json:"sequenceId"`
	StartNodeId              string          `json:"startNodeId"`
	EndNodeId                string          `json:"endNodeId"`
	Released                 bool            `json:"released"`
	MaximumSpeed             float64         `json:"maximumSpeed"`
	MaximumMobileRobotHeight float64         `json:"maximumMobileRobotHeight"`
	Orientation              float64         `json:"orientation"`
	OrientationType          OrientationType `json:"orientationType"`
	RotationAllowed          bool            `json:"rotationAllowed"`
	Trajectory               Trajectory      `json:"trajectory"`
	Corridor                 struct{}        `json:"corridor"`
	Actions                  []Action        `json:"actions"`
}

type Order struct {
	Header
	OrderId          string `json:"orderId"`
	OrderUpdateId    int    `json:"orderUpdateId"`
	OrderDescription string `json:"orderDescription"`
	Nodes            []Node `json:"nodes"`
	Edges            []Edge `json:"edges"`
}

func Unmarshal(msg mqtt.Message) (Order, *vdaerrors.VDAError) {
	var order Order
	if err := json.Unmarshal(msg.Payload(), &order); err != nil {
		return Order{}, vdaerrors.New(vdaerrors.ErrorTypeValidationFailure, err.Error())
	}
	return order, nil
}
