# cloud-cmdb

A multi-cloud CMDB CLI tool for querying and inventorying cloud infrastructure resources across Oracle Cloud (OCI), Amazon Web Services (AWS), Microsoft Azure, and Google Cloud Platform (GCP).

## Features

- Unified CLI interface: `cloud-cmdb <provider> <command>`
- Multiple output formats: pretty tables, plain text, CSV, PDF
- Flexible column selection per command (`--columns`)
- Name filtering (`--contains`) and status filtering (`--status`)
- Cross-platform builds: Linux and Windows (amd64)

## Prerequisites

### Go

Go 1.21 or later is required to build from source.

### Oracle Cloud (OCI)

Configure the OCI CLI config file (typically `~/.oci/config`) with a valid profile:

```ini
[DEFAULT]
user=ocid1.user.oc1..xxx
fingerprint=xx:xx:xx:...
tenancy=ocid1.tenancy.oc1..xxx
region=us-ashburn-1
key_file=~/.oci/oci_api_key.pem
```

### AWS

Credentials are resolved in this order:
1. `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` environment variables
2. `~/.aws/credentials` file (profile via `--profile` flag)
3. IAM instance profile / task role (when running on EC2/ECS/EKS)

### Azure

Authentication uses the Default Azure Credential chain:
1. `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_SECRET` environment variables
2. `az login` (Azure CLI)
3. Managed Identity (when running on Azure)

The subscription ID is read from `--subscription` flag or `AZURE_SUBSCRIPTION_ID` environment variable.

### Google Cloud (GCP)

Authentication uses Application Default Credentials (ADC):
1. `GOOGLE_APPLICATION_CREDENTIALS` environment variable (path to a service account JSON)
2. `gcloud auth application-default login`
3. GCE metadata service (when running on GCP)

The project ID is read from `--project` flag or `GOOGLE_CLOUD_PROJECT` environment variable.

## Installation

### Build from source

```bash
# Native build (current platform)
make build

# Linux amd64
make build-linux

# Windows amd64
make build-windows

# Both Linux and Windows
make build-all
```

Binaries are written to `./dist/`.

### Install to $GOPATH/bin

```bash
make install
```

## Usage

```
cloud-cmdb [provider] [command] [flags]
```

### OCI (Oracle Cloud Infrastructure)

```bash
# List compute instances
cloud-cmdb oci list-instances

# List instances in a specific compartment
cloud-cmdb oci list-instances --compartment-id ocid1.compartment.oc1..xxx

# List all compartments
cloud-cmdb oci list-compartments

# List VCNs
cloud-cmdb oci list-vcns

# List subnets
cloud-cmdb oci list-subnets

# List OKE clusters
cloud-cmdb oci list-okes

# List object storage buckets
cloud-cmdb oci list-buckets --namespace my-namespace

# List private IPs
cloud-cmdb oci list-private-ips

# List servers (compute + bare metal inventory)
cloud-cmdb oci list-servers

# Use a custom OCI config file and profile
cloud-cmdb oci --config-file ~/.oci/config --profile PROD list-instances

# Export to PDF
cloud-cmdb oci list-instances --pdf --pdf-out oci-instances.pdf
```

### AWS

```bash
# List EC2 instances (uses default AWS credential chain)
cloud-cmdb aws list-instances

# Specific region and profile
cloud-cmdb aws list-instances --region us-east-1 --profile production

# Filter by name
cloud-cmdb aws list-instances --contains=web-,api-

# Filter by state
cloud-cmdb aws list-instances --status=running

# Custom columns
cloud-cmdb aws list-instances --columns="name,instance-id,type,state,private-ip,public-ip"

# Export to CSV
cloud-cmdb aws list-instances --csv > ec2-instances.csv

# Export to PDF
cloud-cmdb aws list-instances --pdf --pdf-out aws-inventory.pdf
```

### Azure

```bash
# List VMs across the subscription (requires AZURE_SUBSCRIPTION_ID or --subscription)
cloud-cmdb azure list-instances

# Explicit subscription ID
cloud-cmdb azure list-instances --subscription 00000000-0000-0000-0000-000000000000

# Filter by resource group
cloud-cmdb azure list-instances --resource-group my-rg

# Filter by name and export to CSV
cloud-cmdb azure list-instances --contains=web- --csv > azure-vms.csv

# PDF report
cloud-cmdb azure list-instances --pdf --pdf-out azure-inventory.pdf
```

### GCP

```bash
# List Compute Engine instances (uses GOOGLE_CLOUD_PROJECT)
cloud-cmdb gcp list-instances

# Explicit project ID
cloud-cmdb gcp list-instances --project my-gcp-project

# Scope to a zone (faster for large projects)
cloud-cmdb gcp list-instances --zone us-central1-a

# Filter by name
cloud-cmdb gcp list-instances --contains=web-,worker-

# Filter by status
cloud-cmdb gcp list-instances --status=RUNNING

# Export to CSV
cloud-cmdb gcp list-instances --csv > gce-instances.csv

# PDF report
cloud-cmdb gcp list-instances --pdf --pdf-out gce-inventory.pdf
```

## Output Formats

All `list-*` commands support the following output flags:

| Flag | Description |
|------|-------------|
| *(default)* | Pretty table with borders and row count footer |
| `--text` | Tab-separated plain text (script-friendly) |
| `--csv` | CSV with header row (pipe to a file for spreadsheets) |
| `--pdf` | PDF report (use `--pdf-out <filename>` for a custom name) |

## Command Reference

| Provider | Command | Description |
|----------|---------|-------------|
| `oci` | `list-instances` | Compute instances |
| `oci` | `list-compartments` | Compartment hierarchy |
| `oci` | `list-vcns` | Virtual Cloud Networks |
| `oci` | `list-subnets` | Subnets within VCNs |
| `oci` | `list-okes` | OKE Kubernetes clusters |
| `oci` | `list-buckets` | Object Storage buckets |
| `oci` | `list-private-ips` | Private IP addresses |
| `oci` | `list-servers` | Server inventory (compute + bare metal) |
| `aws` | `list-instances` | EC2 instances |
| `azure` | `list-instances` | Virtual Machines |
| `gcp` | `list-instances` | Compute Engine VM instances |

## Project Structure

```
cloud-cmdb/
├── main.go
├── Makefile
├── go.mod
├── cmd/
│   ├── root.go             # Root cobra command
│   ├── oci.go              # OCI provider group registration
│   ├── aws.go              # AWS provider group registration
│   ├── azure.go            # Azure provider group registration
│   ├── gcp.go              # GCP provider group registration
│   ├── oci/                # OCI subcommands (package oci)
│   │   ├── flags.go        # Shared flags and PDF helper
│   │   ├── listbuckets.go
│   │   ├── listcompartments.go
│   │   ├── listinstances.go
│   │   ├── listokes.go
│   │   ├── listprivateips.go
│   │   ├── listservers.go
│   │   ├── listsubnets.go
│   │   └── listvcns.go
│   ├── aws/                # AWS subcommands (package aws)
│   │   ├── flags.go
│   │   └── listinstances.go
│   ├── azure/              # Azure subcommands (package azure)
│   │   ├── flags.go
│   │   └── listinstances.go
│   └── gcp/                # GCP subcommands (package gcp)
│       ├── flags.go
│       └── listinstances.go
└── internal/
    ├── oci/                # OCI SDK wrappers
    ├── aws/                # AWS SDK wrappers
    ├── azure/              # Azure SDK wrappers
    ├── gcp/                # GCP SDK wrappers
    └── pdf/                # PDF generation (maroto)
```

## Build Targets

```
make build                - Native build (current platform)
make build-linux          - Linux x86_64
make build-windows        - Windows x86_64 (.exe)
make build-all            - Linux + Windows (recommended for releases)
make build-all-upx        - Same + UPX compression (smaller binaries)
make clean                - Remove dist/ directory
make fmt                  - Format code (go fmt ./...)
make vet                  - Run go vet ./...
make test                 - Run tests
make install              - Install via go install
```

UPX compression is optional. Install [UPX](https://upx.github.io/) to produce significantly smaller binaries. Builds continue normally if UPX is not found.
