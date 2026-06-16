package cmd

import (
	cmdgcp "cloud-cmdb/cmd/gcp"
	"github.com/spf13/cobra"
)

var gcpCmd = &cobra.Command{
	Use:   "gcp",
	Short: "Google Cloud Platform commands",
	Long: `Commands for Google Cloud Platform (GCP) resources.

Authentication uses Application Default Credentials (ADC):
  1. GOOGLE_APPLICATION_CREDENTIALS env var (path to service account JSON)
  2. gcloud auth application-default login
  3. GCE metadata service (when running on GCP)

The project ID is read from --project flag or GOOGLE_CLOUD_PROJECT env var.

Global GCP flags (available to all gcp subcommands):
  --project   GCP project ID (default: GOOGLE_CLOUD_PROJECT env var)
  --pdf-out   Custom PDF output filename when using --pdf

Output format flags (per subcommand): --tables (default), --text, --csv, --pdf`,
	Example: `  # List all GCE instances (uses GOOGLE_CLOUD_PROJECT)
  cloud-cmdb gcp list-instances

  # Explicit project ID
  cloud-cmdb gcp list-instances --project my-gcp-project

  # Scope to a zone
  cloud-cmdb gcp list-instances --zone us-central1-a

  # Filter by name and export to CSV
  cloud-cmdb gcp list-instances --contains=web- --csv > gce.csv

  # PDF report
  cloud-cmdb gcp list-instances --pdf --pdf-out gce-inventory.pdf`,
}

func init() {
	cmdgcp.RegisterPersistentFlags(gcpCmd)
	gcpCmd.AddCommand(
		cmdgcp.ListInstancesCmd,
	)
	rootCmd.AddCommand(gcpCmd)
}
