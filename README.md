# Shure-NMOS Bridge

A high-performance gateway that bridges Shure Axient Digital wireless systems to the NMOS (Networked Media Open Specifications) ecosystem.

## Summary
This application discovers Shure Axient devices via mDNS and translates their proprietary TPCI control protocol into standard NMOS IS-04 and IS-07 resources. It allows broadcast controllers to discover, monitor, and manage Shure wireless systems using open interoperability standards.

## Capabilities
- Compatibility with any Shure device that can be monitored or controlled over the network via Wireless Workbench (WWB).
- Automatic discovery of Shure Axient and QLX-D receivers via mDNS.
- Translation of Shure device parameters into NMOS IS-04 Node API resources.
- Dynamic control assignment based on specific device model capabilities.
- Real-time status monitoring (battery levels, audio peaks, and RF quality) via NMOS IS-07 Event & Tally websockets.
- Support for both standard REP and periodic SAMPLE metering messages.
- Unique control URL generation per discovered hardware unit.
- Multi-channel mapping for high-density receivers like the AD4Q.
