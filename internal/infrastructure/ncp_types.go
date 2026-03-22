package infrastructure

import (
	"encoding/json"

	"github.com/Woord-En-Lewe/shure-nmos-bridge/internal/nca"
)

// NCP Type Aliases for MS-05 compatibility
// These aliases allow the IS-12 wire protocol to use MS-05 defined types
type (
	NCPPropertyID = nca.PropertyID
	NCPMethodID   = nca.MethodID
	NCPEventID    = nca.EventID
)

// NCP Message Types
const (
	NCPMessageTypeCommand              = 0
	NCPMessageTypeResponse             = 1
	NCPMessageTypeNotification         = 2
	NCPMessageTypeSubscription         = 3
	NCPMessageTypeSubscriptionResponse = 4
	NCPMessageTypeError                = 5
)

// NCP Method IDs (Core NcObject)
var (
	NCMethodGet                = NCPMethodID{Level: 1, Index: 1}
	NCMethodSet                = NCPMethodID{Level: 1, Index: 2}
	NCMethodGetSequenceItem    = NCPMethodID{Level: 1, Index: 3}
	NCMethodSetSequenceItem    = NCPMethodID{Level: 1, Index: 4}
	NCMethodAddSequenceItem    = NCPMethodID{Level: 1, Index: 5}
	NCMethodRemoveSequenceItem = NCPMethodID{Level: 1, Index: 6}
	NCMethodGetSequenceLength  = NCPMethodID{Level: 1, Index: 7}
)

// NCP Property IDs (Core NcObject)
var (
	NCPropertyClassID     = NCPPropertyID{Level: 1, Index: 1}
	NCPropertyOID         = NCPPropertyID{Level: 1, Index: 2}
	NCPropertyConstantOID = NCPPropertyID{Level: 1, Index: 3}
	NCPropertyOwner       = NCPPropertyID{Level: 1, Index: 4}
	NCPropertyRole        = NCPPropertyID{Level: 1, Index: 5}
	NCPropertyUserLabel   = NCPPropertyID{Level: 1, Index: 6}
)

// NCPMessage represents the top-level IS-12 message structure
type NCPMessage struct {
	MessageType      int                  `json:"messageType"`
	Commands         []NCPCommand         `json:"commands,omitempty"`
	CommandResponses []NCPCommandResponse `json:"responses,omitempty"`
	Notifications    []NCPNotification    `json:"notifications,omitempty"`
	Subscriptions    []int                `json:"subscriptions,omitempty"`
	Errors           []NCPError           `json:"errors,omitempty"`
}

// NCPCommand represents a single command in an NCP message
type NCPCommand struct {
	Handle    int             `json:"handle"`
	OID       int             `json:"oid"`
	MethodID  NCPMethodID     `json:"methodId"`
	Arguments json.RawMessage `json:"arguments,omitempty"`
}

// NCPCommandResponse represents a response to a command
type NCPCommandResponse struct {
	Handle int             `json:"handle"`
	Result NCPMethodResult `json:"result"`
}

// NCPError represents an error message
type NCPError struct {
	SubscriptionID int    `json:"subscriptionId,omitempty"`
	MessageType    int    `json:"messageType"`
	Status         int    `json:"status"`
	ErrorMessage   string `json:"errorMessage"`
}

// NCPMethodResult contains the status and value of a method execution
type NCPMethodResult struct {
	Status int         `json:"status"`
	Value  interface{} `json:"value,omitempty"`
}

// NCPNotification represents an event notification (e.g., PropertyChanged)
type NCPNotification struct {
	OID       int                      `json:"oid"`
	EventID   NCPEventID               `json:"eventId"`
	EventData PropertyChangedEventData `json:"eventData"`
}

// PropertyChangedEventData is the payload for a PropertyChanged notification
// Uses nca.PropertyChangedData as the base per MS-05-01
type PropertyChangedEventData = nca.PropertyChangedData

// NcBlockMemberDescriptor provides metadata about an object in a block
type NcBlockMemberDescriptor struct {
	Description string `json:"description,omitempty"`
	Role        string `json:"role"`
	OID         int    `json:"oid"`
	ConstantOID bool   `json:"constantOid"`
	ClassID     []int  `json:"classId"`
	UserLabel   string `json:"userLabel,omitempty"`
	Owner       *int   `json:"owner"`
}

// NcClassDescriptor describes a control class
type NcClassDescriptor struct {
	Description string                 `json:"description,omitempty"`
	ClassID     []int                  `json:"classId"`
	Name        string                 `json:"name"`
	FixedRole   *string                `json:"fixedRole,omitempty"`
	Properties  []NcPropertyDescriptor `json:"properties"`
	Methods     []NcMethodDescriptor   `json:"methods"`
	Events      []NcEventDescriptor    `json:"events"`
}

// NcPropertyDescriptor describes a class property
type NcPropertyDescriptor struct {
	Description  string        `json:"description,omitempty"`
	ID           NCPPropertyID `json:"id"`
	Name         string        `json:"name"`
	TypeName     string        `json:"typeName"`
	IsReadOnly   bool          `json:"isReadOnly"`
	IsNullable   bool          `json:"isNullable"`
	IsSequence   bool          `json:"isSequence"`
	IsDeprecated bool          `json:"isDeprecated"`
}

// NcMethodDescriptor describes a class method
type NcMethodDescriptor struct {
	Description    string                  `json:"description,omitempty"`
	ID             NCPMethodID             `json:"id"`
	Name           string                  `json:"name"`
	ResultDatatype string                  `json:"resultDatatype"`
	Parameters     []NcParameterDescriptor `json:"parameters"`
	IsDeprecated   bool                    `json:"isDeprecated"`
}

// NcParameterDescriptor describes a method parameter
type NcParameterDescriptor struct {
	Description string `json:"description,omitempty"`
	Name        string `json:"name"`
	TypeName    string `json:"typeName"`
	IsNullable  bool   `json:"isNullable"`
	IsSequence  bool   `json:"isSequence"`
}

// NcEventDescriptor describes a class event
type NcEventDescriptor struct {
	Description   string     `json:"description,omitempty"`
	ID            NCPEventID `json:"id"`
	Name          string     `json:"name"`
	EventDatatype string     `json:"eventDatatype"`
	IsDeprecated  bool       `json:"isDeprecated"`
}

// NcObject interface defines the core requirements for an NMOS Control object
type NcObject interface {
	GetOID() int
	GetClassID() []int
	GetRole() string
	GetProperty(id NCPPropertyID) (interface{}, error)
	SetProperty(id NCPPropertyID, value interface{}) error
	InvokeMethod(id NCPMethodID, args json.RawMessage) (interface{}, error)
	GetDescriptor() NcBlockMemberDescriptor
	SetNotifyCallback(func(oid int, eventID NCPEventID, eventData PropertyChangedEventData))
}
