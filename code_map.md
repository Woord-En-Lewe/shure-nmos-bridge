# Code Map

## Module: cmd/gateway
- **summary**: Main application entry point that initializes and runs the Shure-NMOS gateway
- **when_to_use**: Use this module to start the gateway application with command-line configuration
- **public_types**: 
  - None (main package)
- **public_functions**: 
  - main() - Entry point that parses flags, creates gateway, and manages lifecycle

## Module: cmd/dummy_node
- **summary**: A test utility that simulates an NMOS node with gain and fader controls
- **when_to_use**: Use this module to test NMOS control and event handling without requiring physical Shure hardware
- **public_types**: 
  - None (main package)
- **public_functions**: 
  - main() - Entry point that initializes a dummy NMOS node with simulated controls and events

## Module: internal/module
- **summary**: Contains the core gateway logic implementing the MIM (Module-Infrastructure-Module) pattern. This is where the business logic for translating between Shure Axient and NMOS protocols resides.
- **when_to_use**: Use this module when you need to understand or modify the core gateway behavior and protocol translation logic
- **public_types**: 
  - Gateway - Interface defining Start and Stop methods for the gateway
- **public_functions**: 
  - NewGateway(shureAddr, nmosAddr string) Gateway - Factory function to create a new Gateway instance

## Module: internal/infrastructure
- **summary**: Contains infrastructure implementations for Shure Axient control protocol, NMOS IS-04/IS-05, and message passing. This layer handles all external system interactions. The Shure controller implements actual TCP/IP communication with Shure Axient devices using the TPCI command protocol. Includes builder pattern for commands, domain-specific types for parsing responses, and robust mDNS discovery using the zeroconf library for automatic device detection. The NMOS controller now includes automatic registry discovery via mDNS and node self-registration per IS-04 specification.
- **when_to_use**: Use this module when you need to understand or modify how the gateway interacts with external systems (Shure devices, NMOS registry/event systems)
- **public_types**: 
  - ShureController - Interface for Shure Axient communication
  - NMOSController - Interface for NMOS IS-04/IS-05 communication (added UpdateResource and BroadcastEvent methods, IS-07 websocket support for real-time events, automatic registry discovery and node registration)
  - MessageBus - Interface for internal message passing (defined in message_bus.go)
  - Message - Structure for messages passed between components (added Source field for origin tracking)
  - MessageType - Type definition for message categorization (defined in message_bus.go)
  - ShureCommandBuilder - Builder pattern for creating Shure commands
  - Gain, Mute, Frequency, Channel, DeviceID - Domain-specific types for Shure parameters
  - DeviceStatus - Domain type for parsed Shure device status
  - ShureDiscoverer - Robust mDNS-based discovery for Shure Axient devices using zeroconf
  - DiscoveredDevice - Representation of a discovered Shure device
- **public_functions**: 
  - NewShureController(addr string) ShureController - Factory for Shure controller (now implements real TCP/IP communication)
  - NewNMOSController(addr string) NMOSController - Factory for NMOS controller (now automatically discovers registries and registers the node)
  - NewInMemoryMessageBus() MessageBus - Factory for in-memory message bus
  - NewShureCommand(command string) *ShureCommandBuilder - Factory for command builder
  - ParseDeviceStatus(response string) (*DeviceStatus, error) - Parse Shure responses into domain types
  - NewShureDiscoverer() *ShureDiscoverer - Factory for mDNS discovery of Shure devices