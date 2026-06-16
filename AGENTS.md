# AGENTS.md — cloud-cmdb

This file describes the project scope, architecture, and development conventions for AI coding agents working in this repository.

## Project Scope

`cloud-cmdb` is a multi-cloud CMDB (Configuration Management Database) CLI tool written in Go. It queries cloud provider APIs to produce real-time inventories of infrastructure resources — compute instances, networks, Kubernetes clusters, storage buckets, and more — in tabular, CSV, or PDF formats.

**Binary name:** `cloud-cmdb`  
**Module:** `cloud-cmdb` (go.mod)  
**Go version:** 1.21+

Supported cloud providers and their top-level namespaces:

| Namespace | Provider |
|-----------|----------|
| `oci` | Oracle Cloud Infrastructure |
| `aws` | Amazon Web Services |
| `azure` | Microsoft Azure |
| `gcp` | Google Cloud Platform |

## CLI Structure

Commands follow the pattern:

```
cloud-cmdb <provider> <subcommand> [flags]
```

Examples:
```
cloud-cmdb oci list-instances
cloud-cmdb aws list-instances --region us-east-1
cloud-cmdb azure list-instances --resource-group prod-rg
cloud-cmdb gcp list-instances --project my-project --zone us-central1-a
```

## Repository Layout

```
cloud-cmdb/
├── main.go                   # Calls cmd.Execute()
├── cmd/
│   ├── root.go               # rootCmd (Use: "cloud-cmdb"), Execute() func
│   ├── oci.go                # package cmd — wires cmd/oci/ into rootCmd
│   ├── aws.go                # package cmd — wires cmd/aws/ into rootCmd
│   ├── azure.go              # package cmd — wires cmd/azure/ into rootCmd
│   ├── gcp.go                # package cmd — wires cmd/gcp/ into rootCmd
│   ├── oci/                  # package oci — OCI subcommands
│   │   ├── flags.go          # shared vars, RegisterPersistentFlags(), generateAndSavePDF()
│   │   └── list*.go          # exported *Cmd vars
│   ├── aws/                  # package aws — AWS subcommands
│   │   ├── flags.go
│   │   └── listinstances.go
│   ├── azure/                # package azure — Azure subcommands
│   │   ├── flags.go
│   │   └── listinstances.go
│   └── gcp/                  # package gcp — GCP subcommands
│       ├── flags.go
│       └── listinstances.go
└── internal/
    ├── oci/                  # package oci — OCI SDK wrappers, data types
    ├── aws/                  # package aws — AWS SDK wrappers, data types
    ├── azure/                # package azure — Azure SDK wrappers, data types
    ├── gcp/                  # package gcp — GCP SDK wrappers, data types
    └── pdf/                  # package pdf — PDF generation (maroto)
```

## Provider Pattern

Every cloud provider follows the same three-layer pattern. Understanding this pattern is essential before modifying or extending any provider.

### Layer 1 — `internal/<provider>/`

Contains pure SDK logic with no Cobra dependency. Each file defines:
- An `InstanceSummary` (or similar) struct with flat string fields
- A `List*()` function that calls the cloud SDK and returns `[]SomeSummary`
- Helper functions for resolving credentials, parsing resource IDs, dereferencing pointer fields

Example (`internal/gcp/instances.go`):
```go
package gcp

type InstanceSummary struct { Name, Zone, MachineType, Status, PrivateIP, PublicIP, Project string }

func GetProjectID(flagValue string) (string, error) { ... }
func ListInstances(ctx context.Context, projectID, zone string) ([]InstanceSummary, error) { ... }
```

### Layer 2 — `cmd/<provider>/`

Contains Cobra command definitions. The package name is `<provider>` (e.g., `package gcp`). Each provider package has:

**`flags.go`** — shared package-level variables, persistent flag registration, PDF helper:
```go
package gcp

var (
    project string
    pdfOut  string
)

func RegisterPersistentFlags(cmd *cobra.Command) {
    cmd.PersistentFlags().StringVar(&project, "project", "", "GCP project ID")
    cmd.PersistentFlags().StringVar(&pdfOut, "pdf-out", "", "Custom PDF output filename")
}

func generateAndSavePDF(cmd *cobra.Command, title string, headers []string, rows [][]string) error { ... }
```

**`list*.go`** — exported `*Cmd` variable of type `*cobra.Command`, plus `init()` for local flags:
```go
package gcp

var ListInstancesCmd = &cobra.Command{
    Use:   "list-instances",
    Short: "List Compute Engine instances in the project",
    RunE:  runListInstances,
}

func init() {
    ListInstancesCmd.Flags().StringVar(&listInstancesZone, "zone", "", "GCP zone")
    // ...
}
```

### Layer 3 — `cmd/<provider>.go`

A single file in `package cmd` that:
1. Declares the provider group `*cobra.Command` (e.g., `gcpCmd`)
2. Calls `cmdgcp.RegisterPersistentFlags(gcpCmd)` in `init()`
3. Adds all subcommands via `gcpCmd.AddCommand(...)`
4. Adds the group to `rootCmd`

```go
package cmd

import (
    cmdgcp "cloud-cmdb/cmd/gcp"
    "github.com/spf13/cobra"
)

var gcpCmd = &cobra.Command{
    Use:   "gcp",
    Short: "Google Cloud Platform commands",
}

func init() {
    cmdgcp.RegisterPersistentFlags(gcpCmd)
    gcpCmd.AddCommand(cmdgcp.ListInstancesCmd)
    rootCmd.AddCommand(gcpCmd)
}
```

## Import Aliasing

Because `cmd/<provider>/` is `package <provider>` and `internal/<provider>/` is also `package <provider>`, imports must be aliased to avoid collisions:

| Importing file | Import path | Alias |
|----------------|-------------|-------|
| `cmd/oci/*.go` | `cloud-cmdb/internal/oci` | `ociinternal` |
| `cmd/aws/*.go` | `cloud-cmdb/internal/aws` | `awsinternal` |
| `cmd/azure/*.go` | `cloud-cmdb/internal/azure` | `azureinternal` |
| `cmd/gcp/*.go` | `cloud-cmdb/internal/gcp` | `gcpinternal` |
| `cmd/oci.go` | `cloud-cmdb/cmd/oci` | `cmdoci` |
| `cmd/aws.go` | `cloud-cmdb/cmd/aws` | `cmdaws` |
| `cmd/azure.go` | `cloud-cmdb/cmd/azure` | `cmdazure` |
| `cmd/gcp.go` | `cloud-cmdb/cmd/gcp` | `cmdgcp` |

## How to Add a New Provider

Follow these steps exactly when adding a new cloud provider (e.g., `alibaba`):

1. **Create `internal/alibaba/`** with at least one file:
   - Define `type InstanceSummary struct { ... }` with flat string fields
   - Implement `func ListInstances(ctx context.Context, ...) ([]InstanceSummary, error)`
   - Add credential resolution helper (flag → env var → SDK default)

2. **Create `cmd/alibaba/flags.go`** (`package alibaba`):
   - Declare shared package vars (region, pdfOut, etc.)
   - Implement `func RegisterPersistentFlags(cmd *cobra.Command)`
   - Copy `generateAndSavePDF()` from any existing provider's `flags.go`

3. **Create `cmd/alibaba/listinstances.go`** (`package alibaba`):
   - Declare `var ListInstancesCmd = &cobra.Command{...}`
   - Implement `func runListInstances(...)` with output format logic
   - Implement the column system: `type instanceColumn int`, iota constants, `parseColumns()`, `columnHeaders()`, `columnValues()`
   - Import `internal/alibaba` aliased as `alibabainternal`

4. **Create `cmd/alibaba.go`** (`package cmd`):
   - Declare `var alibabaCmd = &cobra.Command{Use: "alibaba", ...}`
   - Wire in `init()`: call `RegisterPersistentFlags`, `AddCommand`, `rootCmd.AddCommand`

5. **Add SDK dependency**: `go get <alibaba-sdk-package>`

6. **Update `README.md`**: add provider to Prerequisites, Usage, and Command Reference sections.

## How to Add a New Command to an Existing Provider

1. Create `cmd/<provider>/list<resource>.go` with `package <provider>`
2. Declare an exported `List<Resource>Cmd *cobra.Command`
3. Add the command in `cmd/<provider>.go`'s `init()` via `providerCmd.AddCommand(cmd<Provider>.List<Resource>Cmd)`
4. Create the corresponding `internal/<provider>/<resource>.go` with SDK logic if needed

## Column System

Every `list-*` command implements a typed column system for `--columns` support:

```go
type instanceColumn int

const (
    colName instanceColumn = iota
    colZone
    // ...
)

var defaultColumns = []instanceColumn{colName, colZone, ...}

func parseColumns(cols string) ([]instanceColumn, error) { ... }
func columnHeader(col instanceColumn) string { ... }
func columnValue(inst SomeSummary, col instanceColumn) string { ... }
```

Unknown column names return an error listing valid options. Missing values are rendered as `"-"`.

## Output Format Flags

All list commands expose the same four output flags as local `bool` vars:

| Flag | Behavior |
|------|----------|
| `--tables` (default) | go-pretty table with header and `TOTAL: N` footer |
| `--text` | Tab-separated lines, no header |
| `--csv` | CSV with header row via `encoding/csv` |
| `--pdf` | PDF via `internal/pdf`; filename from `--pdf-out` or `<cmd-use>.pdf` |

Mutually exclusive combinations are validated at the start of `RunE`:
```go
if listInstancesCSV && listInstancesText { return fmt.Errorf("cannot use --csv and --text together") }
```

## Authentication Patterns

| Provider | Mechanism | Key env vars |
|----------|-----------|--------------|
| OCI | `~/.oci/config` file | `OCI_CONFIG_FILE` (optional) |
| AWS | SDK default chain | `AWS_ACCESS_KEY_ID`, `AWS_SECRET_ACCESS_KEY`, `AWS_PROFILE`, `AWS_REGION` |
| Azure | `azidentity.NewDefaultAzureCredential` | `AZURE_CLIENT_ID`, `AZURE_TENANT_ID`, `AZURE_CLIENT_SECRET`, `AZURE_SUBSCRIPTION_ID` |
| GCP | Application Default Credentials | `GOOGLE_APPLICATION_CREDENTIALS`, `GOOGLE_CLOUD_PROJECT` |

## Key Dependencies

| Package | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI command tree, flags, help generation |
| `github.com/jedib0t/go-pretty/v6/table` | Terminal table rendering |
| `github.com/johnfercher/maroto/v2` | PDF generation (`internal/pdf`) |
| `github.com/oracle/oci-go-sdk/v65` | OCI SDK |
| `github.com/aws/aws-sdk-go-v2` | AWS SDK v2 |
| `github.com/Azure/azure-sdk-for-go/sdk` | Azure SDK (azidentity, armcompute/v6, armnetwork/v6) |
| `cloud.google.com/go/compute/apiv1` | GCP Compute Engine SDK |
| `google.golang.org/api/iterator` | GCP iterator sentinel (`iterator.Done`) |

## Coding Conventions

- All command vars are exported: `ListInstancesCmd`, `ListBucketsCmd`, etc.
- All output goes to `os.Stdout` directly (not `cmd.OutOrStdout()`) — this is a CLI tool, not a library; no test interception needed.
- `RunE` is always used instead of `Run` so errors propagate naturally.
- `SilenceUsage: true` and `SilenceErrors: true` are set on `rootCmd` — `main.go` handles error printing.
- No global `init()` side effects in `internal/` packages; SDK clients are created inside functions.
- Pointer fields from SDK responses are always dereferenced through a `ptrStr(*string) string` helper that returns `""` on nil. Values displayed to users use `"-"` for empty/missing fields.

## Build

```bash
make build          # native
make build-all      # linux + windows amd64
make fmt && make vet
```

Binaries land in `./dist/`. UPX compression is applied automatically if `upx` is installed; non-fatal otherwise.
