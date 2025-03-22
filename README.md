# Fibratus Management Portal (Concept...)

A centralized management system for Fibratus nodes, providing Windows endpoint monitoring, rule management, alert collection, network isolation, and remote remediation capabilities.

## Features

- **Centralized Windows Node Management**: Real-time view of all Windows endpoints with status indicators
- **Immediate Network Isolation**: Instantly isolate compromised Windows systems with no delay
- **Rule Management**: Centralized rule repository with version control
- **Alert Collection**: Centralized alert dashboard and management
- **Remote Remediation**: Execute commands on Windows nodes for incident response
- **Windows Event Log Collection**: Collect and analyze Windows event logs
- **Process Investigation**: Query and visualize Windows process information
- **Configuration Management**: Distribute configurations to Windows nodes

## Portal-Agent Communication Architecture

The Fibratus Management Portal employs a hybrid communication model designed specifically for Windows environments, with special emphasis on immediate response to critical security commands.

### Dual-Channel Communication Model

The portal and Windows agents use a **dual-channel communication system**:

1. **Real-time Command Channel** (for immediate actions):
   - Persistent WebSocket connection maintained by agents
   - Enables immediate command delivery from portal to agents
   - Priority channel for isolation and critical security commands
   - Zero-delay response to critical incidents

2. **Standard HTTP Channel** (for routine operations):
   - Used for regular agent operations (heartbeats, data collection)
   - Periodic polling for non-critical commands
   - Data transmission for logs, alerts, and status updates

3. **Agent Authentication**:
   - Both channels secured with TLS and API key authentication
   - WebSocket connections maintain persistent authentication
   - HTTP requests include API key in Authorization header

### Isolation Command Workflow

When an incident responder initiates isolation through the portal UI:

1. **Immediate Dispatch**:
   - Portal instantly sends isolation command through WebSocket channel
   - Command marked as highest priority
   - Delivery confirmation required from agent

2. **Agent Response**:
   - Agent receives command immediately via WebSocket
   - Executes isolation procedures with no polling delay
   - Responds with acknowledgment within milliseconds
   - Implements isolation before any potential malware can react

3. **Execution Confirmation**:
   - Agent implements Windows Firewall isolation immediately
   - Sends confirmation of successful isolation
   - Portal updates UI to show "Isolated" status in real-time

4. **Failsafe Mechanism**:
   - If WebSocket channel fails, command is also queued in HTTP channel
   - Agent will receive command on next HTTP poll (backup mechanism)
   - Portal alerts admins if immediate isolation cannot be confirmed

### Real-Time Status Monitoring

The WebSocket channel provides these additional benefits:

1. **Instant Status Updates**:
   - Agents report critical status changes in real-time
   - Portal displays accurate node status without polling delays

2. **Bi-directional Communication**:
   - Portal can query agents for immediate information
   - Agents can push critical alerts without waiting for next poll cycle

3. **Connection Health Monitoring**:
   - WebSocket heartbeats verify connection is alive
   - Automatic reconnection with exponential backoff
   - Portal tracks agent connectivity status in real-time

## Windows Agent Requirements

Windows agents must implement these key capabilities to support immediate isolation:

### Communication Implementation

1. **WebSocket Client**:
   - Maintain persistent WebSocket connection to portal
   - Implement automatic reconnection if connection drops
   - Handle TLS certificate validation
   - Process commands received through WebSocket immediately

2. **Windows Service Operation**:
   - Run as a Windows service with automatic startup
   - Run with SYSTEM privileges for full system access
   - Implement service recovery options for reliability
   - Handle Windows updates and reboots while maintaining connection

3. **Command Prioritization**:
   - Process isolation commands with highest priority
   - Interrupt any non-critical operations to handle isolation commands
   - Implement command queueing with priority levels

### Network Isolation Capabilities

1. **Instant Windows Firewall Control**:
   - Immediate implementation of Windows Firewall rules
   - Use Windows Filtering Platform (WFP) APIs for instant rule application
   - Support for complex filtering scenarios
   - Maintain detailed logging of firewall changes

2. **IPsec Implementation**:
   - Configure IPsec policies through Windows API
   - Create secure tunnels back to management server
   - Implement connection security rules

3. **Selective Connectivity During Isolation**:
   - Maintain WebSocket connection to portal during isolation
   - Allow DNS resolution if specified
   - Block all other inbound/outbound traffic
   - Support for custom allow-list of emergency connections

### Windows System Integration

1. **Windows Event Log Integration**:
   - Subscribe to Windows event logs
   - Forward security events in real-time via WebSocket
   - Support batch collection via HTTP channel

2. **ETW (Event Tracing for Windows) Integration**:
   - Consume ETW events from security providers
   - Process and analyze ETW data
   - Generate alerts based on suspicious patterns

3. **Registry and File System Monitoring**:
   - Real-time monitoring of critical registry keys
   - File system activity monitoring
   - Instant alerting on suspicious changes

## Immediate Isolation Implementation Details

The isolation command delivered via WebSocket has this structure:

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

Agent acknowledgment is immediate:

```json
{
  "command_id": "cmd-12345",
  "status": "received",
  "timestamp": "2025-03-22T15:30:45.123Z"
}
```

Followed quickly by execution confirmation:

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

## Standard Command Implementation

Agents must also implement handlers for these Windows-specific command categories:

### File Commands

```json
{
  "command": "remove-file",
  "id": "cmd-12346",
  "details": {
    "path": "C:\\malware\\suspicious.exe",
    "force": true
  }
}
```

### Windows Registry Commands

```json
{
  "command": "revert-registry-key",
  "id": "cmd-12347",
  "details": {
    "key_path": "HKLM\\SOFTWARE\\Microsoft\\Windows\\CurrentVersion\\Run",
    "backup_file": "run_keys_backup.reg"
  }
}
```

### Windows Process Commands

```json
{
  "command": "kill-process",
  "id": "cmd-12348",
  "details": {
    "pid": 1234,
    "force": true
  }
}
```

### Windows Event Log Collection Commands

```json
{
  "command": "collect-windows-logs",
  "id": "cmd-12349",
  "details": {
    "log_type": "Security",
    "start_time": "2025-03-20T00:00:00Z",
    "end_time": "2025-03-22T23:59:59Z",
    "event_ids": [4624, 4625],
    "max_events": 1000
  }
}
```

## Windows Agent Implementation Guidance

1. **Real-Time Response Architecture**:
   - Design agent with non-blocking async architecture
   - Use Windows I/O completion ports for efficiency
   - Implement command queue with priority processing
   - Ensure WebSocket channel has highest priority

2. **Windows Security Considerations**:
   - Run with SYSTEM privileges for immediate firewall control
   - Use Windows DPAPI for secure storage of credentials
   - Verify command authenticity before execution
   - Implement secure logging of all critical actions

3. **Isolation Reliability**:
   - Test isolation under various network conditions
   - Verify isolation persists after system reboots
   - Ensure agent can still communicate with portal post-isolation
   - Implement periodic verification that isolation is still effective

4. **Performance Optimization**:
   - Minimize WebSocket overhead
   - Optimize Windows Firewall rule application
   - Balance between real-time monitoring and system performance
   - Implement intelligent throttling during user activity

## Tech Stack

- **Portal Backend**: Go
- **Portal Frontend**: HTML, CSS, JavaScript
- **Database**: PostgreSQL
- **Web Server**: Nginx
- **Agent Platform**: Windows (agent implementation not included in this repository)
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

## Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

## License

This project is licensed under the MIT License - see the LICENSE file for details.
