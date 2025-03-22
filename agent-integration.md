# Fibratus Agent Integration Guide

This guide explains how to extend Fibratus agents to integrate with the Fibratus Management Portal.

## Overview

Fibratus agents already provide powerful capabilities for event monitoring, rule-based detection, and local alerting. Integration with the Management Portal adds:

- Centralized rule management
- Real-time command and control
- Immediate network isolation capability
- Enterprise-wide alert visibility
- Remote remediation capabilities

## Integration Requirements

To integrate Fibratus with the portal, you'll need to implement these extensions:

### 1. Portal Communication Module

Extend Fibratus with a communication module that implements:

#### HTTP API Integration

- Endpoint: `/agent/heartbeat`
  - Method: POST
  - Frequency: Every 30-60 seconds
  - Purpose: Report system status and maintain connection
  - Example request:
    ```json
    {
      "hostname": "workstation-001",
      "ip_address": "192.168.1.101",
      "os_version": "Windows 10 Pro 21H2",
      "fibratus_version": "2.3.0",
      "cpu": "Intel Core i7-10700 @ 2.90GHz",
      "memory": "32GB",
      "disk": "C: 500GB (120GB free)"
    }
    ```

- Endpoint: `/agent/commands`
  - Method: GET
  - Frequency: Every 10-15 seconds
  - Purpose: Poll for pending commands
  - Query params: `?hostname=workstation-001`

- Endpoint: `/agent/commands/{id}/result`
  - Method: POST
  - Purpose: Report command execution results
  - Example request:
    ```json
    {
      "success": true,
      "message": "Process terminated successfully",
      "details": {
        "execution_time_ms": 145,
        "process_id": 1234
      }
    }
    ```

- Endpoint: `/agent/alerts`
  - Method: POST
  - Purpose: Submit alerts to the portal
  - Example request:
    ```json
    {
      "rule_name": "credential_dumping_vaultcmd",
      "severity": "high",
      "title": "Credential Dumping Detected",
      "description": "Vault credentials enumeration using VaultCmd.exe",
      "process": {
        "name": "vaultcmd.exe",
        "pid": 4567,
        "path": "C:\\Windows\\System32\\vaultcmd.exe",
        "command_line": "VaultCmd.exe /listcreds:\"Windows Credentials\" /all"
      },
      "timestamp": "2025-03-22T14:25:32.561Z",
      "metadata": {
        "technique_id": "T1003.005",
        "tactic": "credential-access",
        "mitre_url": "https://attack.mitre.org/techniques/T1003/005/"
      }
    }
    ```

- Endpoint: `/agent/rules`
  - Method: GET
  - Frequency: Every 5-10 minutes
  - Purpose: Get latest rules assigned to this agent
  - Query params: `?hostname=workstation-001`

- Endpoint: `/agent/config`
  - Method: GET
  - Frequency: Every 15-30 minutes
  - Purpose: Get latest configuration for this agent
  - Query params: `?hostname=workstation-001`

#### WebSocket Integration for Real-time Communication

- Endpoint: `/agent/ws`
  - Query params: `?hostname=workstation-001`
  - Purpose: Establish WebSocket connection for real-time commands
  - Authentication: Same API key used for HTTP requests

- Message Types:
  - Command messages (from server to agent):
    ```json
    {
      "command_id": "cmd-12345",
      "command": "isolate-host",
      "priority": "critical",
      "details": {
        "reason": "Ransomware detection",
        "allow_portal_communication": true,
        "allow_dns": true,
        "portal_ips": ["192.168.1.10"],
        "portal_ports": [443, 8443]
      }
    }
    ```
  
  - Acknowledgment messages (from agent to server):
    ```json
    {
      "command_id": "cmd-12345",
      "status": "received",
      "timestamp": "2025-03-22T15:30:45.123Z"
    }
    ```
  
  - Result messages (from agent to server):
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

### 2. Network Isolation Module

Implement network isolation capability:

#### Isolation Implementation

Use Windows Firewall with Advanced Security (WFAS) or Windows Filtering Platform (WFP) to:

1. Block all inbound and outbound connections
2. Allow specific exceptions:
   - Connections to/from the Management Portal IP(s) and port(s)
   - DNS resolution (if specified in the command)

Example PowerShell implementation:

```powershell
# Block all outbound traffic
New-NetFirewallRule -DisplayName "Block All Outbound" -Direction Outbound -Action Block -Enabled True

# Allow connections to Management Portal
New-NetFirewallRule -DisplayName "Allow Management Portal" -Direction Outbound -Action Allow -RemoteAddress 192.168.1.10 -RemotePort 443 -Protocol TCP -Enabled True

# Allow DNS if specified
New-NetFirewallRule -DisplayName "Allow DNS" -Direction Outbound -Action Allow -RemotePort 53 -Protocol UDP -Enabled True
```

#### De-isolation Implementation

When receiving the unisolate command:

1. Remove all firewall rules created during isolation
2. Restore previous firewall configuration if applicable

### 3. Command Execution Module

Implement handlers for these commands:

#### Process Commands
- `kill-process`: Terminate a process by PID or name
- `get-process-details`: Get detailed information about a process
- `get-process-tree`: Get the process tree starting from a specific PID

#### File Commands
- `remove-file`: Delete a file
- `quarantine-file`: Move a file to a quarantine location
- `list-files`: List files in a directory
- `get-file-hash`: Calculate hash for a file

#### Registry Commands
- `get-registry-value`: Read a registry value
- `set-registry-value`: Set a registry value
- `revert-registry-key`: Revert a registry key to a previous state
- `export-registry-hive`: Export a registry hive

#### System Commands
- `reboot-node`: Restart the system
- `get-system-info`: Get detailed system information

#### Log Collection Commands
- `collect-windows-logs`: Collect and transmit Windows Event Logs
- `collect-fibratus-logs`: Collect and transmit Fibratus logs

## Integration Implementation

### For Python Filament-based Integration

You can implement the portal integration as a Fibratus filament:

```python
"""
Fibratus filament for Management Portal integration
"""
import json
import requests
import websocket
import threading
import time
import subprocess
import os
from fibratus.filament import Filament

class PortalIntegration(Filament):
    def init(self):
        self.config = self.kevent.get_config()
        self.api_key = self.config.get('portal_api_key', '')
        self.portal_url = self.config.get('portal_url', 'https://portal.example.com')
        self.hostname = os.environ['COMPUTERNAME']
        
        # Start WebSocket connection
        self.ws_thread = threading.Thread(target=self.ws_connect)
        self.ws_thread.daemon = True
        self.ws_thread.start()
        
        # Start heartbeat thread
        self.heartbeat_thread = threading.Thread(target=self.heartbeat_loop)
        self.heartbeat_thread.daemon = True
        self.heartbeat_thread.start()
        
        # Start command polling thread
        self.command_thread = threading.Thread(target=self.command_poll_loop)
        self.command_thread.daemon = True
        self.command_thread.start()

    def ws_connect(self):
        """Establish and maintain WebSocket connection"""
        headers = {"Authorization": f"ApiKey {self.api_key}"}
        ws_url = f"{self.portal_url.replace('https://', 'wss://')}/agent/ws?hostname={self.hostname}"
        
        def on_message(ws, message):
            try:
                cmd = json.loads(message)
                if 'command_id' in cmd and 'command' in cmd:
                    # Acknowledge receipt
                    self.ws.send(json.dumps({
                        "command_id": cmd["command_id"],
                        "status": "received",
                        "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime())
                    }))
                    
                    # Process command
                    threading.Thread(target=self.process_command, args=(cmd,)).start()
            except Exception as e:
                print(f"Error processing WebSocket message: {e}")
        
        def on_error(ws, error):
            print(f"WebSocket error: {error}")
        
        def on_close(ws, close_status_code, close_reason):
            print(f"WebSocket closed. Reconnecting in 10 seconds...")
            time.sleep(10)
            self.ws_connect()
        
        def on_open(ws):
            print("WebSocket connection established")
        
        # Create WebSocket connection
        self.ws = websocket.WebSocketApp(ws_url,
                                         on_message=on_message,
                                         on_error=on_error,
                                         on_close=on_close,
                                         on_open=on_open,
                                         header=headers)
        
        # Run WebSocket connection in a loop
        while True:
            try:
                self.ws.run_forever()
            except Exception as e:
                print(f"WebSocket error: {e}")
            time.sleep(10)

    def heartbeat_loop(self):
        """Send periodic heartbeats to the portal"""
        while True:
            try:
                self.send_heartbeat()
            except Exception as e:
                print(f"Heartbeat error: {e}")
            time.sleep(60)  # Send heartbeat every minute

    def command_poll_loop(self):
        """Poll for commands periodically"""
        while True:
            try:
                self.poll_commands()
            except Exception as e:
                print(f"Command poll error: {e}")
            time.sleep(15)  # Poll every 15 seconds

    def send_heartbeat(self):
        """Send a heartbeat to the portal"""
        url = f"{self.portal_url}/agent/heartbeat"
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"ApiKey {self.api_key}"
        }
        
        # Collect system information
        system_info = self.get_system_info()
        
        # Send heartbeat
        response = requests.post(url, headers=headers, json=system_info)
        if response.status_code != 200:
            print(f"Heartbeat failed: {response.status_code} {response.text}")

    def get_system_info(self):
        """Collect system information for heartbeat"""
        import platform
        import psutil
        import re
        
        # Get CPU info
        try:
            cpu_info = platform.processor()
        except:
            cpu_info = "Unknown"
        
        # Get memory info
        try:
            memory = psutil.virtual_memory()
            memory_info = f"{round(memory.total / (1024**3))}GB"
        except:
            memory_info = "Unknown"
        
        # Get disk info
        try:
            disk = psutil.disk_usage('/')
            disk_info = f"C: {round(disk.total / (1024**3))}GB ({round(disk.free / (1024**3))}GB free)"
        except:
            disk_info = "Unknown"
        
        return {
            "hostname": self.hostname,
            "ip_address": socket.gethostbyname(socket.gethostname()),
            "os_version": f"{platform.system()} {platform.release()}",
            "fibratus_version": "2.3.0",  # Update with actual version
            "cpu": cpu_info,
            "memory": memory_info,
            "disk": disk_info
        }

    def poll_commands(self):
        """Poll for pending commands"""
        url = f"{self.portal_url}/agent/commands?hostname={self.hostname}"
        headers = {"Authorization": f"ApiKey {self.api_key}"}
        
        response = requests.get(url, headers=headers)
        if response.status_code == 200:
            commands = response.json().get("commands", [])
            for cmd in commands:
                # Process each command in a new thread
                threading.Thread(target=self.process_command, args=(cmd,)).start()

    def process_command(self, cmd):
        """Process a command from the portal"""
        command_id = cmd.get("command_id") or cmd.get("id")
        command_type = cmd.get("command") or cmd.get("command_type")
        details = cmd.get("details") or cmd.get("command_details", {})
        
        print(f"Processing command: {command_id} of type {command_type}")
        
        result = {
            "success": False,
            "message": "Command not implemented",
            "details": {}
        }
        
        # Process different command types
        if command_type == "isolate-host":
            result = self.execute_isolate_host(details)
        elif command_type == "unisolate-host":
            result = self.execute_unisolate_host(details)
        elif command_type == "kill-process":
            result = self.execute_kill_process(details)
        elif command_type == "get-process-details":
            result = self.execute_get_process_details(details)
        # ... implement other command handlers ...
        
        # Send result
        self.send_command_result(command_id, result)

    def send_command_result(self, command_id, result):
        """Send command execution result to the portal"""
        # First try WebSocket if available
        if hasattr(self, 'ws') and self.ws.sock and self.ws.sock.connected:
            try:
                response = {
                    "command_id": command_id,
                    "status": "completed",
                    "success": result["success"],
                    "message": result["message"],
                    "details": result["details"],
                    "timestamp": time.strftime("%Y-%m-%dT%H:%M:%S.000Z", time.gmtime())
                }
                self.ws.send(json.dumps(response))
                return
            except:
                pass  # Fall back to HTTP
        
        # HTTP fallback
        url = f"{self.portal_url}/agent/commands/{command_id}/result"
        headers = {
            "Content-Type": "application/json",
            "Authorization": f"ApiKey {self.api_key}"
        }
        requests.post(url, headers=headers, json=result)

    def execute_isolate_host(self, details):
        """Implement host isolation"""
        try:
            # Allow portal communication
            portal_ips = details.get("portal_ips", [])
            portal_ports = details.get("portal_ports", [443])
            allow_dns = details.get("allow_dns", True)
            
            # Block all outbound traffic
            subprocess.run([
                "powershell", 
                "-Command", 
                "New-NetFirewallRule -DisplayName 'Fibratus Isolation - Block All' -Direction Outbound -Action Block -Enabled True"
            ], check=True)
            
            # Allow portal communication
            for ip in portal_ips:
                for port in portal_ports:
                    subprocess.run([
                        "powershell",
                        "-Command",
                        f"New-NetFirewallRule -DisplayName 'Fibratus Isolation - Allow Portal {ip}:{port}' -Direction Outbound -Action Allow -RemoteAddress {ip} -RemotePort {port} -Protocol TCP -Enabled True"
                    ], check=True)
            
            # Allow DNS if specified
            if allow_dns:
                subprocess.run([
                    "powershell",
                    "-Command",
                    "New-NetFirewallRule -DisplayName 'Fibratus Isolation - Allow DNS' -Direction Outbound -Action Allow -RemotePort 53 -Protocol UDP -Enabled True"
                ], check=True)
            
            return {
                "success": True,
                "message": "Host isolated successfully",
                "details": {
                    "portal_ips_allowed": portal_ips,
                    "dns_allowed": allow_dns
                }
            }
        except Exception as e:
            return {
                "success": False,
                "message": f"Failed to isolate host: {str(e)}",
                "details": {"error": str(e)}
            }

    # ... implement other command execution methods ...

    def on_close(self):
        """Cleanup when filament closes"""
        if hasattr(self, 'ws'):
            self.ws.close()
```

### For Native C/C++ Integration

For better performance and deeper system integration, you can implement these features in C/C++ directly within the Fibratus agent:

1. Create a new module in the Fibratus codebase for portal integration
2. Implement WebSocket client for real-time communication
3. Add HTTP client for polling operations
4. Integrate with the Fibratus rule engine for rule synchronization
5. Implement isolation capabilities using Windows APIs

## Configuration

Configure the portal integration with these settings in your Fibratus configuration:

```yaml
portal:
  enabled: true
  url: https://fibratus-portal.example.com
  api_key: your-api-key-here
  heartbeat_interval: 60
  command_poll_interval: 15
  websocket:
    enabled: true
    reconnect_interval: 10
  isolation:
    allow_dns: true
    dns_servers: []  # Empty for system default
```

## Testing Your Integration

1. Start by testing HTTP communication:
   - Verify heartbeats are being sent and received
   - Check that rules can be retrieved

2. Test WebSocket communication:
   - Verify connection establishment
   - Test command delivery and execution

3. Test isolation functionality:
   - Send isolation command
   - Verify network is properly isolated
   - Verify portal communication is maintained
   - Test de-isolation command

## Security Considerations

1. **API Key Security**:
   - Store API keys securely using Windows DPAPI
   - Rotate keys periodically

2. **Command Validation**:
   - Validate all commands before execution
   - Implement allowlisting for command parameters

3. **TLS Verification**:
   - Always validate TLS certificates
   - Consider certificate pinning for production

4. **Privilege Management**:
   - Run with least privilege when possible
   - Elevate only when necessary for specific commands

## Troubleshooting

Common issues and solutions:

1. **Connection Issues**:
   - Check firewall settings
   - Verify TLS certificates
   - Check API key validity

2. **Command Execution Failures**:
   - Check command syntax and parameters
   - Verify sufficient privileges
   - Check system requirements for commands

3. **Isolation Problems**:
   - Verify Windows Firewall service is running
   - Check for conflicting firewall rules
   - Ensure portal IPs are correctly specified

## Support

For integration support, contact the Fibratus Portal team.