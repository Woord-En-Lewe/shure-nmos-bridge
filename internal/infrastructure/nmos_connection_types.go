package infrastructure



// Connection Activation Modes
const (
	ActivationModeImmediate          = "activate_immediate"
	ActivationModeScheduledAbsolute  = "activate_scheduled_absolute"
	ActivationModeScheduledRelative  = "activate_scheduled_relative"
	ActivationModeNull               = ""
)

// ConnectionActivation represents the activation settings for IS-05
type ConnectionActivation struct {
	Mode          string  `json:"mode"`
	RequestedTime *string `json:"requested_time"`
	ActivationTime *string `json:"activation_time,omitempty"`
}

// TransportFile represents the transport file resource (SDP, etc.)
type TransportFile struct {
	Data string `json:"data"`
	Type string `json:"type"`
}

// ConnectionStaged represents the staged parameters for a Sender or Receiver
type ConnectionStaged struct {
	SenderID        *string                  `json:"sender_id,omitempty"`
	ReceiverID      *string                  `json:"receiver_id,omitempty"`
	MasterEnable    bool                     `json:"master_enable"`
	Activation      ConnectionActivation     `json:"activation"`
	TransportFile   *TransportFile           `json:"transport_file,omitempty"`
	TransportParams []map[string]interface{} `json:"transport_params"`
}

// ConnectionActive represents the active parameters for a Sender or Receiver
type ConnectionActive struct {
	SenderID        *string                  `json:"sender_id,omitempty"`
	ReceiverID      *string                  `json:"receiver_id,omitempty"`
	MasterEnable    bool                     `json:"master_enable"`
	Activation      ConnectionActivation     `json:"activation"`
	TransportParams []map[string]interface{} `json:"transport_params"`
}

// ConnectionConstraints represents the constraints for a Sender or Receiver
type ConnectionConstraints []interface{}
