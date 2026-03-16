package infrastructure

import (
	"context"
)

// ShureController defines the interface for Shure Axient control protocol communication
type ShureController interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	SendCommand(command interface{}) error
	ReceiveEvents() <-chan interface{}
}

// NMOSController defines the interface for NMOS IS-04/IS-05 communication
type NMOSController interface {
	Start(ctx context.Context) error
	Stop(ctx context.Context) error
	RegisterNode(node interface{}) error
	RegisterResource(resourceType string, resource interface{}) error
	UpdateResource(resourceType string, id string, updateFn func(interface{}) interface{}) error
	SetControls(deviceID string, controls []map[string]interface{})
	GetControls(deviceID string) []map[string]interface{}
	RegisterNCPObject(oid int, obj NcObject)
	GetNCPObject(oid int) NcObject
	GetNodes() ([]interface{}, error)
	SubscribeToEvents() <-chan interface{}
	GetNodeID() string
	BroadcastEvent(source string, eventType string, data interface{})
	OnControlChange(callback func(deviceID, controlID string, value interface{}))
}
