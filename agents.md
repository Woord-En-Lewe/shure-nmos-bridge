# NMOS-Shure Gateway Agents

This document tracks the agents and sub-agents involved in the NMOS-Shure Gateway project and their specific roles in the development and maintenance lifecycle.

## Core Agents

### Gemini CLI (Main Orchestrator)
- **Role**: Primary developer and system designer.
- **Responsibilities**:
  - Translating user requirements into architectural decisions.
  - Managing sub-agent delegation.
  - Ensuring technical integrity and adherence to NMOS and Shure Axient protocols.
  - Final validation of all changes.

## Specialized Sub-Agents

### Codebase Investigator
- **Role**: Architectural analysis and dependency mapping.
- **Responsibilities**:
  - Identifying the root cause of duplicate control URLs.
  - Mapping the relationship between Shure controllers and NMOS resources.
  - Ensuring consistent implementation of the MIM (Module-Infrastructure-Module) architecture.

### Generalist
- **Role**: Batch processing and complex investigations.
- **Responsibilities**:
  - Implementing the multi-file protocol expansion for `SAMPLE ALL` support.
  - Managing the addition of the `gorilla/websocket` dependency.
  - Refactoring the NMOS controller to support IS-07 websocket broadcasts.

## System Configuration

### Memory & Context
- The agents prioritize the `code_map.md` for context loading and architectural alignment.
- Global engineering standards are enforced to ensure provably correct and modular software.
