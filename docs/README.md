# cloud-cmdb — Technical Documentation

Complete reference for installing, authenticating, and operating the `cloud-cmdb` CLI.

**Quick navigation:** [Prerequisites](#prerequisites) · [Installation](#installation) · [Usage](#usage) · [Output formats](#output-formats) · [Command reference](#command-reference) · [Project structure](#project-structure) · [Build targets](#build-targets) · [Architecture](#architecture) · [SDK resources](SDK_RESOURCES.md)

---

## Overview

`cloud-cmdb` is a multi-cloud Configuration Management Database (CMDB) CLI written in Go. It queries cloud provider APIs in real time and returns infrastructure inventories in tabular, CSV, plain text, or PDF formats.

```
cloud-cmdb <provider> <subcommand> [flags]
```

| Namespace | Provider | SDK |
|-----------|----------|-----|
| `oci` | Oracle Cloud Infrastructure | [`oci-go-sdk/v65`](SDK_RESOURCES.md#oci) |
| `aws` | Amazon Web Services | [`aws-sdk-go-v2`](SDK_RESOURCES.md#aws) |
| `azure` | Microsoft Azure | [`azure-sdk-for-go`](SDK_RESOURCES.md#azure) |
| `gcp` | Google Cloud Platform | [`cloud.google.com/go/compute`](SDK_RESOURCES.md#gcp) |

---

## Prerequisites

### Go

Go **1.21 or later** is required to build from source.

```bash
go version   # must report go1.21 or newer
```

### Authentication

Each provider resolves credentials through its native SDK chain. No additional `cloud-cmdb` config file is needed beyond what the provider SDK already expects.

#### Oracle Cloud (OCI)

Configure the OCI CLI config file (typically `~/.oci/config`) with a valid profile:

```ini
[DEFAULT]
user=ocid1.user.oc1..xxx
fingerprint=xx:xx:xx:...
tenancy=ocid1.tenancy.oc1..xxx
region=us-ashburn-1
key_file=~/.oci/oci_api_key.pem
```

| Flag / env var | Purpose |
|----------------|---------|
| `--config-file` | Path to OCI config (default: `~/.oci/config`) |
| `--profile` | Config profile name (default: `DEFAULT`) |

#### AWS

Credentials are resolved in this order:

1. `AWS_ACCESS_KEY_ID` / `AWS_SECRET_ACCESS_KEY` environment variables
2. `~/.aws/credentials` file (profile via `--profile` flag)
3. IAM instance profile / task role (when running on EC2/ECS/EKS)

| Flag / env var | Purpose |
|----------------|---------|
| `--region` | AWS region (falls back to `AWS_REGION` / `AWS_DEFAULT_REGION`) |
| `--profile` | Named profile in `~/.aws/credentials` |

#### Azure

Authentication uses the **Default Azure Credential** chain:

1. `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_SECRET` environment variables
2. `az login` (Azure CLI cached credentials)
3. Managed Identity (when running on Azure)

| Flag / env var | Purpose |
|----------------|---------|
| `--subscription` | Azure subscription ID |
| `AZURE_SUBSCRIPTION_ID` | Subscription ID when flag is omitted |

#### Google Cloud (GCP)

Authentication uses **Application Default Credentials (ADC)**:

1. `GOOGLE_APPLICATION_CREDENTIALS` environment variable (path to service account JSON)
2. `gcloud auth application-default login`
3. GCE/GKE metadata service (when running on GCP)

| Flag / env var | Purpose |
|----------------|---------|
| `--project` | GCP project ID |
| `GOOGLE_CLOUD_PROJECT` | Project ID when flag is omitted |

---

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

### Cross-platform releases

For production deployments, prefer `make build-all`, which produces Linux and Windows amd64 binaries. Add UPX (`make build-all-upx`) for smaller artifacts when [UPX](https://upx.github.io/) is available.

---

## Usage

### Global flags (all providers)

Persistent flags registered on each provider group:

| Flag | Description |
|------|-------------|
| `--pdf-out` | Custom filename for PDF output (default: `<command>.pdf`) |

Output flags are local to each `list-*` command (see [Output formats](#output-formats)).

Filtering flags (where supported):

| Flag | Description |
|------|-------------|
| `--contains` | Comma-separated name substrings (OR match) |
| `--status` | Filter by resource status/state |
| `--columns` | Comma-separated column names to display |

---

### OCI (Oracle Cloud Infrastructure)

OCI has the broadest command coverage in `cloud-cmdb`.

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

**OCI-specific persistent flags:**

| Flag | Description |
|------|-------------|
| `--compartment-id` | Scope queries to a compartment OCID |
| `--config-file` | OCI config file path |
| `--profile` | OCI config profile |

---

### AWS

Official SDK links (developer guide, API reference, service docs): **[SDK_RESOURCES.md — AWS](SDK_RESOURCES.md#aws)**

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

**AWS `list-instances` columns:** `name`, `instance-id`, `type`, `state`, `private-ip`, `public-ip`, `platform`, `az`, `vpc`, `subnet`, `launched`, `region`

---

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

**Azure-specific flags:**

| Flag | Description |
|------|-------------|
| `--resource-group` | Limit to a single resource group |

---

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

**GCP-specific flags:**

| Flag | Description |
|------|-------------|
| `--zone` | Restrict listing to a single zone (omit to scan all zones) |

---

## Output Formats

All `list-*` commands support the same four output modes. Flags are mutually exclusive pairwise checks — you cannot combine `--csv` and `--text`, for example.

| Flag | Description |
|------|-------------|
| *(default)* | Pretty table with borders and `TOTAL: N` footer |
| `--text` | Tab-separated plain text, no header (script-friendly) |
| `--csv` | CSV with header row (pipe to a file for spreadsheets) |
| `--pdf` | PDF report via `internal/pdf` (maroto); use `--pdf-out <filename>` for a custom name |

**Examples:**

```bash
# Default table
cloud-cmdb aws list-instances

# Pipe-friendly text
cloud-cmdb aws list-instances --text | awk -F'\t' '$3 == "running"'

# Spreadsheet export
cloud-cmdb gcp list-instances --csv > gce-$(date +%F).csv

# Audit report
cloud-cmdb oci list-instances --pdf --pdf-out monthly-oci-audit.pdf
```

Missing or empty field values are rendered as `-` in table and CSV output.

---

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

Run `cloud-cmdb <provider> <command> --help` for the full flag list of any command.

---

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

### Three-layer provider pattern

Every cloud provider follows the same layout:

1. **`internal/<provider>/`** — Pure SDK logic. Defines `*Summary` structs and `List*()` functions. No Cobra dependency.
2. **`cmd/<provider>/`** — Cobra commands, column system, output formatting, and shared flags in `flags.go`.
3. **`cmd/<provider>.go`** — Wires the provider group into `rootCmd`.

When adding a new resource command, create `internal/<provider>/<resource>.go` and `cmd/<provider>/list<resource>.go`, then register it in `cmd/<provider>.go`.

See [AGENTS.md](../AGENTS.md) for contributor conventions and import aliasing rules.

---

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

Build metadata (`Version`, `Commit`, `BuildDate`) is injected via `-ldflags` when building through the Makefile.

---

## Architecture

```
┌─────────────┐     ┌──────────────────┐     ┌─────────────────────┐
│  main.go    │────▶│  cmd/root.go     │────▶│  cmd/<provider>.go  │
└─────────────┘     └──────────────────┘     └──────────┬──────────┘
                                                        │
                        ┌───────────────────────────────┘
                        ▼
              ┌─────────────────────┐     ┌──────────────────────┐
              │ cmd/<provider>/   │────▶│ internal/<provider>/ │
              │ list*.go          │     │ SDK wrappers         │
              └─────────┬─────────┘     └──────────────────────┘
                        │
          ┌─────────────┼─────────────┐
          ▼             ▼             ▼
      table/text     CSV writer    internal/pdf
```

**Design principles:**

- `RunE` (not `Run`) on all commands so errors propagate to `main.go`
- `SilenceUsage` and `SilenceErrors` on `rootCmd` — `main.go` prints errors
- Output goes to `os.Stdout` directly
- SDK clients are created inside functions, not in package `init()`
- Pointer fields from SDK responses are dereferenced via `ptrStr()` helpers; empty values display as `-`

---

## Troubleshooting

| Symptom | Likely cause | Fix |
|---------|--------------|-----|
| `NoCredentialProviders` (AWS) | Missing credentials | Set env vars, configure `~/.aws/credentials`, or use an IAM role |
| `DefaultAzureCredential failed` | Not logged in | Run `az login` or set service principal env vars |
| `could not find default credentials` (GCP) | ADC not configured | Run `gcloud auth application-default login` or set `GOOGLE_APPLICATION_CREDENTIALS` |
| OCI `NotAuthenticated` | Invalid config/profile | Verify `~/.oci/config` and API key permissions |
| Empty results | Wrong scope | Check `--region`, `--compartment-id`, `--zone`, or `--resource-group` |
| `unknown column` error | Typo in `--columns` | Run `--help` on the command for valid column names |

---

## Related

- [Project README (overview)](../README.md)
- [SDK_RESOURCES.md — official provider SDK documentation](SDK_RESOURCES.md)
- [AGENTS.md — contributor guide](../AGENTS.md)
- [LICENSE](../LICENSE)