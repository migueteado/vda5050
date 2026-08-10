package common

import (
	"fmt"
	"strings"
)

const INTERFACE_NAME string = "vda5050"
const MAJOR_VERSION = "v3"

type TopicName string

const (
	Order TopicName = "order" // fleet -> robot, mandatory
	InstantActions TopicName = "instantActions" // fleet -> robot, mandatory
	State TopicName = "state" // robot -> fleet, mandatory
	Connection TopicName = "connection" // robot -> fleet, mandatory
	Factsheet TopicName = "factsheet" // robot -> fleet, mandatory
	Visualization TopicName = "visualization" // robot -> UI, optional
	ZoneSet TopicName = "zoneSet" // fleet -> robot, optional
	Responses TopicName = "responses" // fleet -> robot, optional
)

var ALL_TOPICS = []TopicName{Order, InstantActions, State, Connection, Factsheet, Visualization, ZoneSet, Responses}

// Published by fleet control, subscribed by the robot
var FLEET_TO_ROBOT = []TopicName{Order, InstantActions, ZoneSet, Responses}

// Published by the robot, subscribed by fleet control / visualization
var ROBOT_TO_FLEET = []TopicName{State, Connection, Factsheet, Visualization}

// Quality of service. 0 for everything except 'connection', which is QoS 1 
//because a death notice is the one message that nothing later supersedes
var QOS = map[TopicName]int{
	Order: 0,
	InstantActions: 0,
	State: 0,
	Connection: 1,
	Factsheet: 0,
	Visualization: 0,
	ZoneSet: 0,
	Responses: 0,
}

// Retained so a fleet manager that starts later immediately learns the 
// current connection state of every robot
var RETAINED = []TopicName{Connection}


// --- VALIDATION ------------------------------------------------------

var FORBIDDEN_CHARS = []string{"/", "+", "#", "$"}
var ALLOWED_CHARS = []string{
	"abcdefghijklmnopqrstuvwxyz",
	"ABCDEFGHIJKLMNOPQRSTUVWXYZ",
	"0123456789",
	"_.:-",
}

type TopicError struct {
	Topic string
	Reason string
}

func (e *TopicError) Error() string {
	return fmt.Sprintf("Invalid topic '%q': %s", e.Topic, e.Reason)
}

type FieldError struct {
	Field string
	Reason string
}

func (e *FieldError) Error() string {
	return fmt.Sprintf("Invalid field '%q': %s", e.Field, e.Reason)
}

func ValidateSegment(field, value string) error {
	if value == "" {
		return &FieldError{Field: field, Reason: "empty value"}
	}
	for _, c := range FORBIDDEN_CHARS {
		if strings.Contains(value, c) {
			return &FieldError{Field: field, Reason: fmt.Sprintf("contains forbidden character '%s'", c)}
		}
	}
	return nil
}

func ValidateSerial(serial string) error {
	err :=  ValidateSegment("serial", serial)
	if err != nil {
		return err
	}
	allowed := strings.Join(ALLOWED_CHARS, "")
	for _, c := range serial {
		if !strings.Contains(allowed, string(c)) {
			return &FieldError{Field: "serial", Reason: fmt.Sprintf("contains invalid character '%c'", c)}
		}
	}
	return nil
}

func ValidateTopicName(topic string) error {
	for _, t := range ALL_TOPICS {
		if TopicName(topic) == t {
			return nil
		}
	}
	return &TopicError{Topic: topic, Reason: "unknown topic"}
}

// --- CONSTRUCTION ------------------------------------------------------

func TopicFor(manufacturer string, serial string, topic TopicName) (string, error) {
	err := ValidateSegment("manufacturer", manufacturer)
	if err != nil {
		return "", err
	}
	err = ValidateSerial(serial)
	if err != nil {
		return "", err
	}
	err = ValidateTopicName(string(topic))
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/%s/%s/%s", INTERFACE_NAME, MAJOR_VERSION, manufacturer, serial, topic), nil
}


func WildcardFor(topic string) (string, error) {
	err := ValidateTopicName(topic)
	if err != nil {
		return "", err
	}
	return fmt.Sprintf("%s/%s/+/+/%s", INTERFACE_NAME, MAJOR_VERSION, topic), nil
}
