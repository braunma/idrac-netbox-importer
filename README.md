# iDRAC NetBox Importer

A powerful Dell iDRAC hardware inventory collection tool that automatically gathers detailed server hardware information via the Redfish API and optionally syncs it to NetBox for infrastructure management and documentation.

## Features

- **Automated Hardware Discovery**: Scans Dell iDRAC servers via Redfish API
- **Comprehensive Inventory**: Collects CPU, memory, storage, and system information
- **NetBox Integration**: Automatically syncs hardware data to NetBox custom fields
- **IP Range Scanning**: Define server groups with IP ranges and CIDR notation for bulk scanning
- **Multi-Credential Support**: Different username/password combinations for different network segments
- **Parallel Scanning**: Configurable concurrency for fast multi-server inventory
- **Multiple Output Formats**: Console, JSON, CSV, and table formats
- **Connection Validation**: Test connectivity without running full scans
- **Flexible Configuration**: YAML config files with environment variable overrides
- **Docker Support**: Containerized deployment with multi-stage builds
- **Robust Error Handling**: Per-server error tracking without batch failure
- **Structured Logging**: JSON and console logging with configurable levels

## Table of Contents

- [Installation](#installation)
- [Quick Start](#quick-start)
- [Configuration](#configuration)
- [Usage](#usage)
- [NetBox Integration](#netbox-integration)
- [Output Formats](#output-formats)
- [Environment Variables](#environment-variables)
- [Docker Deployment](#docker-deployment)
- [Architecture](#architecture)
- [Development](#development)
- [Troubleshooting](#troubleshooting)

## Installation

### From Source

Requirements:
- Go 1.22 or higher
- Access to Dell iDRAC servers with Redfish API enabled

```bash
# Clone the repository from your GitLab
git clone <your-gitlab-url>/idrac-netbox-importer.git
cd idrac-netbox-importer

# Build the binary
make build

# Or install directly
go install ./cmd/idrac-inventory
```

### Using Docker

```bash
# Build the Docker image
make docker-build

# Or pull from registry (when published)
docker pull yourusername/idrac-inventory:latest
```

## Quick Start

### Single Server Scan

```bash
# Scan a single server and display results
./idrac-inventory -host 192.168.1.10 -user root -pass calvin

# Output as JSON
./idrac-inventory -host 192.168.1.10 -user root -pass calvin -output json
```

### Multi-Server Scan from Config File

```bash
# Create a config file (see config.yaml example)
cp config.yaml my-config.yaml

# Edit with your servers and credentials
vim my-config.yaml

# Run the scan
./idrac-inventory -config my-config.yaml

# With verbose output
./idrac-inventory -config my-config.yaml -verbose
```

### Validate Connections

```bash
# Test connectivity without collecting inventory
./idrac-inventory -config my-config.yaml -validate
```

### Sync to NetBox

```bash
# Set NetBox credentials
export NETBOX_URL="https://netbox.example.com"
export NETBOX_TOKEN="your-api-token-here"

# Scan and sync to NetBox
./idrac-inventory -config my-config.yaml -sync
```

## Configuration

### Config File Structure

Create a `config.yaml` file:

```yaml
# NetBox API Configuration (optional - required for -sync)
netbox:
  url: "${NETBOX_URL}"                    # NetBox URL
  token: "${NETBOX_TOKEN}"                # API token
  insecure_skip_verify: false             # Skip TLS verification
  timeout_seconds: 30                     # API timeout

# Default credentials for all servers
defaults:
  username: "${IDRAC_DEFAULT_USER}"       # iDRAC username
  password: "${IDRAC_DEFAULT_PASS}"       # iDRAC password
  timeout_seconds: 60                     # Connection timeout
  insecure_skip_verify: true              # Skip TLS verification (for self-signed certs)

# Concurrency settings
concurrency: 5                            # Max parallel scans (1-50)

# Logging configuration
logging:
  level: info                             # debug, info, warn, error
  format: console                         # console or json

# Retry configuration
retry:
  max_attempts: 3                         # Max retry attempts
  base_delay: 1s                          # Initial retry delay
  max_delay: 30s                          # Max retry delay

# HTTP client settings
http:
  max_idle_conns: 10                      # Max idle connections
  idle_conn_timeout_seconds: 30           # Idle connection timeout

# Servers to scan
servers:
  - host: idrac1.example.com
    name: "Production Server 1"           # Optional friendly name
    # Override defaults if needed:
    # username: custom-user
    # password: custom-pass
    # timeout_seconds: 120

  - host: 192.168.1.11
    name: "Production Server 2"

  - host: 192.168.1.12
    username: admin                       # Per-server credentials
    password: different-password
```

### IP Range Scanning

You can use `server_groups` to define multiple servers using IP ranges with specific credentials. This is especially useful when you have different credential sets for different network segments:

```yaml
# Define server groups with IP ranges
server_groups:
  # Production servers in Data Center 1
  - name: "DC1 Production"
    ip_ranges:
      - "10.10.10.1-10.10.10.25"        # Range of IPs
      - "10.10.10.58-10.10.10.80"       # Multiple ranges per group
    username: "${DC1_PROD_USER}"
    password: "${DC1_PROD_PASS}"
    timeout_seconds: 60

  # Development servers in Data Center 2
  - name: "DC2 Development"
    ip_ranges:
      - "192.168.1.1-192.168.1.50"
    username: "${DC2_DEV_USER}"
    password: "${DC2_DEV_PASS}"

  # Remote site using CIDR notation
  - name: "Remote Office"
    ip_ranges:
      - "172.16.0.0/24"                 # CIDR expands to 172.16.0.1-254
    username: "remote-admin"
    password: "${REMOTE_PASS}"
    timeout_seconds: 120

  # Group using default credentials
  - ip_ranges:
      - "10.20.30.1-10.20.30.10"
      - "10.20.30.100"                  # Single IP also works
    # Omit username/password to use defaults

# You can use both 'servers' and 'server_groups' together
servers:
  - host: 192.168.1.254
    name: "Special Server"
```

**Supported IP Range Formats:**
- Single IP: `"10.10.10.5"`
- IP Range: `"10.10.10.1-10.10.10.25"` (expands to all IPs from .1 to .25)
- CIDR Notation: `"192.168.1.0/24"` (expands to 192.168.1.1-254, excluding network/broadcast)
- Multiple ranges: Define multiple `ip_ranges` entries in a single group

**Notes:**
- `server_groups` are expanded into individual servers during config loading
- Each server group can have its own credentials, overriding the defaults
- Maximum 10,000 IPs per range (safety limit)
- You can mix `servers` and `server_groups` in the same configuration

### Environment Variable Substitution

Use `${VAR_NAME}` syntax in the config file to substitute environment variables:

```yaml
defaults:
  username: "${IDRAC_DEFAULT_USER}"
  password: "${IDRAC_DEFAULT_PASS}"
```

## Usage

### Command-Line Flags

```
Usage:
  idrac-inventory [options]

Options:
  -config string
        Path to configuration file (default "config.yaml")

  Single Server Mode:
  -host string
        Single host to scan (overrides config file)
  -user string
        Username for single host mode
  -pass string
        Password for single host mode

  Output Options:
  -output string
        Output format: console, json, table, csv (default "console")
  -verbose
        Show detailed output
  -no-color
        Disable colored output

  Actions:
  -sync
        Sync results to NetBox
  -validate
        Only validate connections, don't collect inventory

  Misc:
  -version
        Show version information
  -log-level string
        Log level: debug, info, warn, error (default "info")
```

### Examples

```bash
# Scan from config with detailed output
./idrac-inventory -config config.yaml -verbose

# Scan single server and export to JSON
./idrac-inventory -host 10.0.1.100 -user root -pass calvin -output json > inventory.json

# Scan and sync to NetBox
./idrac-inventory -config config.yaml -sync

# Validate all connections
./idrac-inventory -config config.yaml -validate

# Export to CSV for Excel
./idrac-inventory -config config.yaml -output csv > servers.csv

# Debug mode with JSON logging
./idrac-inventory -config config.yaml -log-level debug
```

## NetBox Integration

### Prerequisites

1. NetBox instance with API access
2. API token with write permissions to devices
3. Custom fields created in NetBox (see below)

### Required NetBox Custom Fields

Create these custom fields on the Device model in NetBox:

| Field Name | Type | Description |
|------------|------|-------------|
| `hw_cpu_count` | Integer | Number of CPUs |
| `hw_cpu_model` | Text | CPU model name |
| `hw_cpu_cores` | Integer | CPU cores per socket |
| `hw_ram_total_gb` | Integer | Total RAM in GB |
| `hw_ram_slots_total` | Integer | Total memory slots |
| `hw_ram_slots_used` | Integer | Used memory slots |
| `hw_ram_slots_free` | Integer | Free memory slots |
| `hw_memory_type` | Text | Memory type (e.g., DDR4, DDR5) |
| `hw_memory_speed_mhz` | Integer | Memory speed in MHz |
| `hw_memory_max_capacity_gb` | Integer | Maximum memory capacity (slots × largest module) |
| `hw_storage_summary` | Text | Storage grouped by capacity (e.g., "2x745GB, 16x14306GB") |
| `hw_storage_total_tb` | Text | Total storage in TB |
| `hw_bios_version` | Text | BIOS version |
| `hw_power_state` | Text | Power state (On/Off) |
| `hw_last_inventory` | Text | Last inventory timestamp |

### Custom Field Name Configuration

If your NetBox uses different field names, configure them via environment variables:

```bash
export NETBOX_FIELD_CPU_COUNT="custom_cpu_count"
export NETBOX_FIELD_CPU_MODEL="custom_cpu_model"
# ... etc
```

See [Environment Variables](#environment-variables) for the complete list.

### Device Matching

The tool matches servers to NetBox devices using:
1. **Service Tag** (asset_tag field) - Primary method
2. **Serial Number** (serial field) - Fallback method

Ensure your NetBox devices have either the service tag or serial number populated.

### Sync Workflow

```bash
# Set NetBox credentials
export NETBOX_URL="https://netbox.company.com"
export NETBOX_TOKEN="abc123..."

# Run scan with sync
./idrac-inventory -config config.yaml -sync
```

The tool will:
1. Test NetBox API connectivity
2. Scan all configured servers
3. Look up each device by service tag or serial
4. Update custom fields with hardware data
5. Report success/failure for each server

## Output Formats

### Console (Default)

Human-readable output with emojis and colors (disable with `-no-color`):

```
═══════════════════════════════════════════════════════════════════
🖥️  server1.example.com (PowerEdge R740)
═══════════════════════════════════════════════════════════════════

📋 System Information:
   Model:         Dell Inc. PowerEdge R740
   Service Tag:   ABC1234
   Serial:        CN123456789
   BIOS:          2.10.2
   Power State:   🟢 On

🔲 CPUs: 2 installed
   └─ Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz (18 Cores / 36 Threads)

💾 Memory: 384 GiB total
   └─ Slots: 12/24 used (12 free)

💿 Storage: 4 drive(s), 7.28 TB total
   └─ 2× SSD (1920 GB total)
   └─ 2× HDD (6400 GB total)
```

### JSON

Machine-readable JSON output:

```bash
./idrac-inventory -config config.yaml -output json
```

```json
{
  "servers": [
    {
      "host": "server1.example.com",
      "collected_at": "2025-12-21T10:30:00Z",
      "model": "PowerEdge R740",
      "manufacturer": "Dell Inc.",
      "service_tag": "ABC1234",
      "cpu_count": 2,
      "cpu_model": "Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz",
      "cpus": [
        {
          "socket": "CPU.Socket.1",
          "model": "Intel(R) Xeon(R) Gold 6140 CPU @ 2.30GHz",
          "cores": 18,
          "threads": 36,
          "max_speed_mhz": 2300
        }
      ],
      "total_memory_gib": 384,
      "drive_count": 4,
      "total_storage_tb": 7.28
    }
  ],
  "stats": {
    "total_servers": 1,
    "successful_count": 1,
    "failed_count": 0,
    "total_duration": 2500000000
  }
}
```

### Table

Tabular output for quick overview:

```bash
./idrac-inventory -config config.yaml -output table
```

```
HOST                    MODEL            SERVICE TAG  CPUs  RAM (GB)  RAM SLOTS  DRIVES  STATUS
----                    -----            -----------  ----  --------  ---------  ------  ------
server1.example.com     PowerEdge R740   ABC1234      2     384       12/24      4       OK
server2.example.com     PowerEdge R640   DEF5678      2     192       6/24       2       OK
```

### CSV

CSV export for spreadsheet analysis:

```bash
./idrac-inventory -config config.yaml -output csv > inventory.csv
```

## Environment Variables

### Application Settings

| Variable | Description | Default |
|----------|-------------|---------|
| `IDRAC_LOG_LEVEL` | Log level (debug, info, warn, error) | `info` |
| `IDRAC_LOG_FORMAT` | Log format (json, console) | `console` |
| `IDRAC_CONCURRENCY` | Max parallel scans (1-50) | `5` |

### iDRAC Connection

| Variable | Description | Default |
|----------|-------------|---------|
| `IDRAC_DEFAULT_USER` | Default iDRAC username | - |
| `IDRAC_DEFAULT_PASS` | Default iDRAC password | - |
| `IDRAC_DEFAULT_TIMEOUT` | Connection timeout (seconds) | `60` |
| `IDRAC_INSECURE_SKIP_VERIFY` | Skip TLS verification | `true` |

### NetBox Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `NETBOX_URL` | NetBox API URL | - |
| `NETBOX_TOKEN` | NetBox API token | - |
| `NETBOX_TIMEOUT` | API timeout (seconds) | `30` |
| `NETBOX_INSECURE_SKIP_VERIFY` | Skip TLS verification | `false` |

### NetBox Custom Field Names

| Variable | Description | Default |
|----------|-------------|---------|
| `NETBOX_FIELD_CPU_COUNT` | CPU count field name | `hw_cpu_count` |
| `NETBOX_FIELD_CPU_MODEL` | CPU model field name | `hw_cpu_model` |
| `NETBOX_FIELD_CPU_CORES` | CPU cores field name | `hw_cpu_cores` |
| `NETBOX_FIELD_RAM_TOTAL` | Total RAM field name | `hw_ram_total_gb` |
| `NETBOX_FIELD_RAM_SLOTS_TOTAL` | RAM slots total field name | `hw_ram_slots_total` |
| `NETBOX_FIELD_RAM_SLOTS_USED` | RAM slots used field name | `hw_ram_slots_used` |
| `NETBOX_FIELD_RAM_SLOTS_FREE` | RAM slots free field name | `hw_ram_slots_free` |
| `NETBOX_FIELD_RAM_TYPE` | Memory type field name | `hw_memory_type` |
| `NETBOX_FIELD_RAM_SPEED` | Memory speed field name | `hw_memory_speed_mhz` |
| `NETBOX_FIELD_RAM_MAX_CAPACITY` | Max memory capacity field name | `hw_memory_max_capacity_gb` |
| `NETBOX_FIELD_STORAGE_SUMMARY` | Storage summary field name | `hw_storage_summary` |
| `NETBOX_FIELD_STORAGE_TOTAL` | Total storage field name | `hw_storage_total_tb` |
| `NETBOX_FIELD_BIOS_VERSION` | BIOS version field name | `hw_bios_version` |
| `NETBOX_FIELD_POWER_STATE` | Power state field name | `hw_power_state` |
| `NETBOX_FIELD_LAST_INVENTORY` | Last inventory field name | `hw_last_inventory` |

### Retry Configuration

| Variable | Description | Default |
|----------|-------------|---------|
| `IDRAC_RETRY_MAX_ATTEMPTS` | Max retry attempts | `3` |
| `IDRAC_RETRY_BASE_DELAY` | Base retry delay | `1s` |
| `IDRAC_RETRY_MAX_DELAY` | Max retry delay | `30s` |

## Docker Deployment

### Build and Run

```bash
# Build the image
make docker-build

# Run with environment variables
docker run --rm \
  -e IDRAC_DEFAULT_USER=root \
  -e IDRAC_DEFAULT_PASS=calvin \
  -e NETBOX_URL=https://netbox.example.com \
  -e NETBOX_TOKEN=your-token \
  -v $(pwd)/config.yaml:/app/config.yaml:ro \
  idrac-inventory:latest -config /app/config.yaml -sync
```

### Docker Compose

```yaml
version: '3.8'

services:
  idrac-inventory:
    build: .
    environment:
      - IDRAC_DEFAULT_USER=root
      - IDRAC_DEFAULT_PASS=calvin
      - NETBOX_URL=https://netbox.example.com
      - NETBOX_TOKEN=${NETBOX_TOKEN}
      - IDRAC_LOG_LEVEL=info
    volumes:
      - ./config.yaml:/app/config.yaml:ro
    command: -config /app/config.yaml -sync
```

### Scheduled Scans with Cron

```bash
# Add to crontab
0 2 * * * docker run --rm -v /path/to/config.yaml:/app/config.yaml idrac-inventory -config /app/config.yaml -sync
```

## Architecture

### Project Structure

```
idrac-netbox-importer/
├── cmd/
│   └── idrac-inventory/      # CLI entry point
│       └── main.go
├── internal/
│   ├── config/               # Configuration management
│   ├── models/               # Data structures
│   ├── netbox/               # NetBox API client
│   ├── output/               # Output formatters
│   ├── redfish/              # Redfish API types
│   └── scanner/              # Hardware scanner (MISSING - see notes)
├── pkg/
│   ├── defaults/             # Default values and env vars
│   ├── errors/               # Custom error types
│   └── logging/              # Structured logging
├── tests/                    # Integration tests
├── config.yaml               # Example configuration
├── Dockerfile                # Multi-stage container build
├── Makefile                  # Build automation
└── README.md

```

### Data Flow

```
Config File/CLI → Scanner → Redfish API (iDRAC) → Data Models → Output Formatter → Console/File
                                                               ↓
                                                         NetBox Client → NetBox API
```

### Key Components

1. **Scanner**: Manages parallel server scanning with concurrency control
2. **Config**: YAML parsing with environment variable overrides
3. **Models**: Data structures for hardware inventory
4. **NetBox Client**: HTTP client for NetBox API integration
5. **Output Formatters**: Multiple output format implementations
6. **Logging**: Structured logging with zap

## Development

### Prerequisites

- Go 1.22+
- Make
- Docker (optional)

### Building

```bash
# Build binary
make build

# Run tests
make test

# Run linters
make lint

# Build Docker image
make docker-build

# Clean build artifacts
make clean
```

### Testing

```bash
# Run unit tests
go test ./...

# Run integration tests
go test ./tests/...

# Run with coverage
go test -cover ./...
```

### Code Organization

- `cmd/`: Application entry points
- `internal/`: Private application code
- `pkg/`: Public libraries (reusable packages)
- `tests/`: Integration and end-to-end tests

## Troubleshooting

### Connection Issues

**Problem**: `connection failed` or timeout errors

**Solutions**:
- Verify iDRAC is accessible: `ping <idrac-host>`
- Check iDRAC credentials are correct
- Ensure Redfish API is enabled on iDRAC
- Increase timeout: `-timeout 120` or in config
- Check firewall rules (iDRAC uses HTTPS port 443)

### Authentication Failures

**Problem**: `authentication failed` errors

**Solutions**:
- Verify username/password are correct
- Check iDRAC user has sufficient privileges
- Ensure account is not locked

### TLS Certificate Errors

**Problem**: `x509: certificate signed by unknown authority`

**Solutions**:
- Enable insecure mode in config:
  ```yaml
  defaults:
    insecure_skip_verify: true
  ```
- Or use environment variable:
  ```bash
  export IDRAC_INSECURE_SKIP_VERIFY=true
  ```

### NetBox Sync Failures

**Problem**: Device not found in NetBox

**Solutions**:
- Verify device exists in NetBox
- Check service tag matches asset_tag field in NetBox
- Or ensure serial number matches serial field
- Run with `-log-level debug` for detailed matching info

**Problem**: Custom field errors

**Solutions**:
- Ensure all required custom fields are created in NetBox
- Verify field names match (or configure via env vars)
- Check API token has write permissions

### Performance Issues

**Problem**: Scans are slow

**Solutions**:
- Increase concurrency in config (max 50):
  ```yaml
  concurrency: 10
  ```
- Check network latency to iDRAC servers
- Reduce timeout if servers are fast:
  ```yaml
  defaults:
    timeout_seconds: 30
  ```

## Known Issues

### Critical - Missing Scanner Implementation

The scanner package implementation (`internal/scanner/scanner.go`) is currently missing from the repository. This file is required for the application to compile and run.

**Status**: The scanner test file exists but the main implementation is missing.

**Impact**: The application will not build without this file.

### Code Quality Issues

See the separate refactoring report for details on:
- Duplicate imports in `formatter.go` and `main.go`
- Redundant code patterns that can be refactored
- Opportunities for code consolidation

## Contributing

Contributions are welcome! Please:

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Add tests for new functionality
5. Run tests and linters
6. Submit a pull request

## License

[Add your license here]

## Support

For issues and questions:
- GitLab Issues: Use your GitLab instance's issue tracker
- Documentation: See the docs/ directory and markdown files in this repository

## Acknowledgments

- Dell for the Redfish API documentation
- NetBox team for the excellent IPAM/DCIM platform
- Go community for the excellent tooling and libraries
