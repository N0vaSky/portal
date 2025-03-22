# Fibratus Management Portal (Concept...)

A centralized management portal for existing Fibratus installations, providing enterprise-wide visibility, control, and orchestration of Fibratus agents.

## Portal Overview

The Fibratus Management Portal extends the capabilities of the existing Fibratus agent ecosystem by adding:

- **Centralized Management**: Monitor and manage all Fibratus agents across your enterprise
- **Enterprise-Wide Rules Management**: Centrally deploy and manage detection rules
- **Alert Aggregation**: Collect and analyze alerts from all endpoints in one place
- **Remote Network Isolation**: Instantly isolate compromised endpoints while maintaining management capability
- **Remote Remediation**: Execute commands on endpoints for incident response
- **Advanced Analytics**: Cross-endpoint correlation and threat hunting

## Fibratus Agent Integration

This portal is designed to integrate with existing Fibratus agents which already provide:

- Real-time ETW kernel event monitoring
- Behavior-driven YAML rule engine
- YARA memory scanning
- Event shipping to various sinks
- Forensics capabilities
- Windows Event Log integration
- Python-based extensibility via filaments

### Integration Architecture

The portal introduces an enterprise management layer that sits above the existing Fibratus infrastructure:

1. **Agent Communication Extension**:
   - Adds WebSocket capability to Fibratus for real-time command and control
   - Maintains backward compatibility with existing Fibratus functionality
   - Provides secure, authenticated communication channel

2. **Immediate Isolation Capability**:
   - Leverages Fibratus' existing system access to implement immediate network isolation
   - Uses Windows Filtering Platform (WFP) APIs for rapid firewall rule deployment
   - Maintains communication channel to management portal during isolation

3. **Centralized Rule Management**:
   - Distributes YAML rules to all managed Fibratus instances
   - Supports different rule sets for different endpoint groups
   - Provides version control and rule testing capabilities

4. **Enterprise-Wide Alerting**:
   - Aggregates alerts from all endpoints in real-time
   - Provides advanced filtering and correlation
   - Supports integration with SOC workflows and SIEM systems

## Required Agent Extensions

To integrate with the Management Portal, existing Fibratus agents require the following extensions:

### Communication Module

A new Fibratus module that enables:

1. **Real-time Command Channel**:
   - WebSocket connection to the portal
   - Secure TLS with certificate validation
   - API key-based authentication
   - Immediate command reception and execution

2. **Status Reporting**:
   - Regular heartbeats to portal
   - System information collection
   - Agent configuration reporting
   - Rule deployment status feedback

### Network Isolation Module

An extension that enables:

1. **Immediate Isolation Capability**:
   - Windows Firewall rule implementation
   - IPsec policy configuration
   - Selective connectivity to management portal
   - Verification of isolation effectiveness

2. **Isolation Management**:
   - Status reporting of isolation state
   - Rule persistence across reboots
   - Safe de-isolation procedures
   - Isolation logging and verification

### Remote Command Module

A new module that processes commands from the portal:

1. **Command Processing**:
   - Secure command validation and execution
   - Privilege management for system operations
   - Result reporting with detailed status
   - Command auditing and logging

2. **Remediation Actions**:
   - File system operations
   - Process management
   - Registry operations
   - System configuration changes

## Portal-Agent Communication

The communication between the portal and agents follows this model:

### Real-time Command Channel

For immediate actions (isolation, critical commands):

```json
{
  "command": "isolate-host",
  "priority": "critical",
  "id": "cmd-12345",
  "details": {
    "reason": "Ransomware detection",
    "allow_portal_communication": true,
    "allow_dns": true,
    "portal_ips": ["192.168.1.10"],
    "portal_ports": [443, 8443]
  }
}
```

Agent acknowledgment:

```json
{
  "command_id": "cmd-12345",
  "status": "received",
  "timestamp": "2025-03-22T15:30:45.123Z"
}
```

Execution confirmation:

```json
{
  "command_id": "cmd-12345",
  "status": "completed",
  "success": true,
  "message": "Host isolated successfully",
  "details": {
    "firewall_rules_added": 4,
    "execution_time_ms": 237,
    "isolation_level": "full"
  },
  "timestamp": "2025-03-22T15:30:45.360Z"
}
```

### Rule Distribution Channel

The portal distributes rules to agents:

```json
{
  "operation": "update_rules",
  "rules": [
    {
      "name": "credential_dumping_vaultcmd",
      "content": "rule credential_dumping_vaultcmd {\n  meta:\n    author = \"Fibratus Team\"\n    ...",
      "version": 2,
      "checksum": "sha256:8a7b3ab6..."
    }
  ]
}
```

### Alert Collection

Agents forward alerts to the portal:

```json
{
  "alert": {
    "rule_name": "credential_dumping_vaultcmd",
    "severity": "high",
    "process": {
      "name": "vaultcmd.exe",
      "pid": 4567,
      "command_line": "VaultCmd.exe /listcreds:\"Windows Credentials\" /all"
    },
    "timestamp": "2025-03-22T14:25:32.561Z",
    "details": {
      "technique_id": "T1003.005",
      "tactic": "credential-access"
    }
  }
}
```

## Integration Implementation

To integrate with the portal, the following components need to be added to Fibratus:

1. **Agent Extension Package**:
   - WebSocket client implementation
   - Command processing module
   - Network isolation capabilities
   - Remote command execution framework

2. **Configuration Updates**:
   - Portal connection parameters
   - API key storage and management
   - WebSocket settings
   - Command authorization controls

3. **Installation Updates**:
   - Portal registration during agent installation
   - API key provisioning
   - Initial configuration setup
   - Testing connectivity to portal

## Deployment Architecture

The complete Fibratus Management solution consists of:

1. **Management Portal Server**:
   - Central web application for management
   - API endpoints for agent communication
   - Database for configuration and alert storage
   - WebSocket server for real-time communication

2. **Extended Fibratus Agents**:
   - Standard Fibratus functionality
   - Portal integration extensions
   - WebSocket client for real-time communication
   - Isolation and remediation capabilities

## Tech Stack

- **Portal Backend**: Go
- **Portal Frontend**: HTML, CSS, JavaScript
- **Database**: PostgreSQL
- **Web Server**: Nginx
- **Agent Extensions**: C/C++ and Python (compatible with existing Fibratus)
- **Real-time Communication**: WebSockets with TLS

## Getting Started

Please refer to [DEPLOYMENT.md](DEPLOYMENT.md) for detailed instructions on deploying the Fibratus Portal.

### Quick Start

For development:

```bash
# Install dependencies
make dev-deps

# Run development server
make dev
```

For production:

```bash
# Build the binary
make build

# Package for Debian-based systems
make package
```

## Project Structure

```
fibratus-portal/
├── cmd/                    # Command-line applications
│   └── server/             # Main server executable
├── internal/               # Internal packages
│   ├── api/                # API endpoints
│   ├── models/             # Database models
│   ├── services/           # Business logic
│   └── ...
├── web/                    # Frontend assets
│   ├── static/             # CSS, JS, etc.
│   └── templates/          # HTML templates
├── migrations/             # Database migrations
├── scripts/                # Installation scripts
├── debian/                 # Debian packaging files
└── ...
```

## Documentation

- [Deployment Guide](DEPLOYMENT.md): Instructions for deploying the portal
- [API Documentation](docs/api.md): API reference
- [User Guide](docs/user-guide.md): End-user documentation
- [Integration Guide](docs/integration.md): How to extend Fibratus agents for portal integration

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
