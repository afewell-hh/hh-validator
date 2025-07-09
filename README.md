# Validator Service

A web service and CLI tool for validating Hedgehog Open Network Fabric (ONF) configuration files using the `hhfab` utility.

## Overview

The Validator service provides a simple way to validate ONF configuration files without requiring users to install and configure the `hhfab` utility locally. It supports two validation use cases:

1. **UC1**: Validate wiring diagram only (generates default fab.yaml)
2. **UC2**: Validate both wiring diagram and custom fab.yaml

## Quick Start

### Using Docker (Recommended)

```bash
# Build and run the service
make docker-run

# Service will be available at http://localhost:8080
```

### Using the CLI

```bash
# Build CLI tool
make build-cli

# Validate wiring diagram only
./cmd/validator -w path/to/wiring.yaml

# Validate both wiring and fab files
./cmd/validator -w path/to/wiring.yaml -f path/to/fab.yaml

# Use custom server URL
./cmd/validator -w wiring.yaml -s http://remote-server:8080

# Verbose output
./cmd/validator -w wiring.yaml -v
```

## Installation

### Prerequisites

- Go 1.21 or later
- Docker (for containerized deployment)

### Building from Source

```bash
# Clone the repository
git clone <repository-url>
cd validator

# Install dependencies
make deps

# Build all components
make build

# Run tests
make test
```

## API Usage

### Validate Files

```bash
POST /validate
Content-Type: multipart/form-data

# Required:
wiring: <wiring-diagram-file>

# Optional:
fab: <fabricator-config-file>
```

**Example with curl:**

```bash
# UC1: Wiring only
curl -X POST http://localhost:8080/validate \
  -F "wiring=@wiring.yaml"

# UC2: Wiring + Fab
curl -X POST http://localhost:8080/validate \
  -F "wiring=@wiring.yaml" \
  -F "fab=@fab.yaml"
```

### Health Check

```bash
GET /health
```

### Service Info

```bash
GET /
```

## Response Format

```json
{
  "success": true,
  "message": "Fabricator config and wiring are valid",
  "output": "06:37:39 INF Hedgehog Fabricator version=v0.40.0...",
  "use_case": "uc1"
}
```

## Configuration

### Environment Variables

- `PORT`: Server port (default: 8080)
- `GIN_MODE`: Gin mode (default: release)

### CLI Options

- `-w, --wiring`: Path to wiring diagram file (required)
- `-f, --fab`: Path to fabricator config file (optional)
- `-s, --server`: Server URL (default: http://localhost:8080)
- `-v, --verbose`: Enable verbose output
- `-t, --timeout`: Request timeout in seconds (default: 30)

## Development

### Project Structure

```
validator/
├── cmd/                    # CLI client
├── server/                 # Web service
├── tests/                  # Test files
├── docs/project/           # Project documentation
├── scripts/                # Build and deployment scripts
├── Dockerfile              # Container definition
├── Makefile               # Build automation
└── README.md              # This file
```

### Development Workflow

1. Make changes to code
2. Run tests: `make test`
3. Build binaries: `make build`
4. Test Docker build: `make docker-build`
5. Commit with conventional commit format

### Testing

```bash
# Run unit tests
make test

# Test CLI functionality
make test-cli

# Test server locally
make run-server
```

## Deployment

### Docker Deployment

```bash
# Build image
make docker-build

# Run container
sudo docker run -p 8080:8080 validator:1.0.0
```

### Binary Deployment

```bash
# Build server binary
make build-server

# Run server
./server/validator-server
```

## Examples

### Example Wiring Diagram

```yaml
apiVersion: wiring.githedgehog.com/v1beta1
kind: VLANNamespace
metadata:
  name: default
spec:
  ranges:
  - from: 1000
    to: 2999
---
apiVersion: wiring.githedgehog.com/v1beta1
kind: Switch
metadata:
  name: spine-1
spec:
  role: spine
```

### Example Fabricator Config

```yaml
apiVersion: fabricator.githedgehog.com/v1beta1
kind: Fabricator
metadata:
  name: default
  namespace: fab
spec:
  config:
    fabric:
      mode: spine-leaf
```

## Troubleshooting

### Common Issues

1. **"hhfab utility not available"**
   - Ensure Docker image is built correctly
   - Check that hhfab is installed in container

2. **"Request timeout"**
   - Increase timeout with `-t` flag
   - Check server logs for processing delays

3. **"File too large"**
   - Files must be under 10MB each
   - Check file size and content

### Getting Help

- Check the [API documentation](docs/project/API_SPEC.md)
- Review [development guidelines](docs/project/DEVELOPMENT.md)
- Check [architecture documentation](docs/project/ARCHITECTURE.md)

## License

This project is licensed under the MIT License.