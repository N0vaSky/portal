# Fibratus Management Portal

A centralized management system for Fibratus nodes, providing node monitoring, rule management, alert collection, network isolation, and remote remediation capabilities.

## Features

- **Centralized Node Management**: Real-time view of all Fibratus nodes with status indicators
- **Network Isolation**: Remotely isolate compromised nodes
- **Rule Management**: Centralized rule repository with version control
- **Alert Collection**: Centralized alert dashboard and management
- **Remote Remediation**: Execute commands on nodes for incident response
- **Log Collection**: Collect and analyze logs from nodes
- **Process Investigation**: Query and visualize process information
- **Configuration Management**: Distribute configurations to nodes

## Tech Stack

- **Backend**: Go
- **Frontend**: HTML, CSS, JavaScript
- **Database**: PostgreSQL
- **Web Server**: Nginx

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