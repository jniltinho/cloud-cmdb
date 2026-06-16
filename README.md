# cloud-cmdb

**Real-time cloud inventory from your terminal — one CLI, four providers.**

Stop juggling separate tools, portals, and spreadsheets to answer "what do we have running, and where?" `cloud-cmdb` queries live APIs across **OCI**, **AWS**, **Azure**, and **GCP**, then delivers clean inventories you can read, filter, export, and share.

---

## Why teams use cloud-cmdb

| Challenge | How cloud-cmdb helps |
|-----------|----------------------|
| Multi-cloud sprawl | One command pattern: `cloud-cmdb <provider> <command>` |
| Stale spreadsheets | Data comes directly from provider APIs — always current |
| Audit & compliance | Export to **CSV** or **PDF** for reports and evidence |
| Automation & CI | **Text** and **CSV** output pipe cleanly into scripts and pipelines |
| Day-to-day ops | Filter by name or status, pick only the columns you need |

Built for **platform engineers**, **SREs**, **security auditors**, and **FinOps** teams who need accurate infrastructure visibility without standing up another platform.

---

## Supported clouds

| Provider | Namespace | Example |
|----------|-----------|---------|
| Oracle Cloud | `oci` | `cloud-cmdb oci list-instances` |
| Amazon Web Services | `aws` | `cloud-cmdb aws list-instances` |
| Microsoft Azure | `azure` | `cloud-cmdb azure list-instances` |
| Google Cloud | `gcp` | `cloud-cmdb gcp list-instances` |

OCI ships the richest command set today — instances, compartments, VCNs, subnets, OKE clusters, buckets, and more. AWS, Azure, and GCP cover compute inventory with the same output and filtering model.

---

## Get started in 60 seconds

**1. Build the binary**

```bash
git clone git@github.com:jniltinho/cloud-cmdb.git
cd cloud-cmdb
make build
```

**2. Authenticate with your cloud** (uses each provider's standard credential chain — no extra config files required for AWS/Azure/GCP)

```bash
# AWS — env vars, ~/.aws/credentials, or IAM role
export AWS_REGION=us-east-1

# Azure — az login or service principal env vars
export AZURE_SUBSCRIPTION_ID=your-subscription-id

# GCP — gcloud ADC or service account JSON
export GOOGLE_CLOUD_PROJECT=your-project-id

# OCI — ~/.oci/config (same as OCI CLI)
```

**3. Run your first inventory**

```bash
./dist/cloud-cmdb aws list-instances
./dist/cloud-cmdb oci list-instances
./dist/cloud-cmdb azure list-instances --contains=prod-
./dist/cloud-cmdb gcp list-instances --status=RUNNING --csv > inventory.csv
```

That's it. No agent to deploy, no database to maintain.

---

## Output that fits your workflow

```
┌─────────────────┬──────────────┬──────────┬─────────┬────────────┐
│ NAME            │ INSTANCE ID  │ TYPE     │ STATE   │ PRIVATE IP │
├─────────────────┼──────────────┼──────────┼─────────┼────────────┤
│ web-prod-01     │ i-0abc123... │ t3.large │ running │ 10.0.1.15  │
│ api-staging-02  │ i-0def456... │ t3.micro │ running │ 10.0.2.8   │
└─────────────────┴──────────────┴──────────┴─────────┴────────────┘
TOTAL: 2
```

| Format | Flag | Best for |
|--------|------|----------|
| Table | *(default)* | Interactive terminal use |
| Plain text | `--text` | Shell scripts and `grep` |
| CSV | `--csv` | Spreadsheets, BI tools, data lakes |
| PDF | `--pdf` | Stakeholder reports and audits |

Combine with `--columns`, `--contains`, and `--status` to narrow results before export.

---

## Common use cases

- **Weekly inventory snapshots** — cron a CSV export per account/region and diff over time
- **Incident response** — quickly list running instances matching a name pattern across clouds
- **Onboarding** — give new engineers a single tool to explore unfamiliar environments
- **Compliance evidence** — generate PDF reports before review cycles
- **Pre-migration audits** — inventory compute and network resources before a cloud move

---

## Requirements

- **Go 1.21+** to build from source (pre-built binaries: see [Releases](https://github.com/jniltinho/cloud-cmdb/releases))
- Valid credentials for the cloud provider you query (see [Authentication guide](docs/README.md#authentication))

---

## Documentation

Full technical reference — prerequisites, per-provider examples, command matrix, project layout, and build targets:

**[docs/README.md](docs/README.md)**

Quick links:

- [Installation](docs/README.md#installation)
- [OCI commands](docs/README.md#oci-oracle-cloud-infrastructure)
- [AWS commands](docs/README.md#aws)
- [Azure commands](docs/README.md#azure)
- [GCP commands](docs/README.md#gcp)
- [Output formats](docs/README.md#output-formats)
- [Command reference](docs/README.md#command-reference)
- [Project structure](docs/README.md#project-structure)

---

## License

MIT — see [LICENSE](LICENSE).