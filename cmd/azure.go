package cmd

import (
	cmdazure "cloud-cmdb/cmd/azure"
	"github.com/spf13/cobra"
)

var azureCmd = &cobra.Command{
	Use:   "azure",
	Short: "Microsoft Azure commands",
	Long: `Commands for Microsoft Azure resources.

Authentication uses the default Azure credential chain:
  1. Environment variables (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID)
  2. Azure CLI (az login)
  3. Managed Identity (when running in Azure)

The subscription ID is read from --subscription flag or AZURE_SUBSCRIPTION_ID env var.

Global Azure flags (available to all azure subcommands):
  --subscription  Azure Subscription ID (default: AZURE_SUBSCRIPTION_ID env var)
  --pdf-out       Custom PDF output filename when using --pdf

Output format flags (per subcommand): --tables (default), --text, --csv, --pdf`,
	Example: `  # List VMs in the subscription (uses AZURE_SUBSCRIPTION_ID)
  cloud-cmdb azure list-instances

  # Explicit subscription ID
  cloud-cmdb azure list-instances --subscription 00000000-0000-0000-0000-000000000000

  # Scope to a resource group
  cloud-cmdb azure list-instances --resource-group prod-rg

  # Export to CSV
  cloud-cmdb azure list-instances --csv > azure-vms.csv

  # PDF report
  cloud-cmdb azure list-instances --pdf --pdf-out azure-inventory.pdf`,
}

func init() {
	cmdazure.RegisterPersistentFlags(azureCmd)
	azureCmd.AddCommand(
		cmdazure.ListInstancesCmd,
	)
	rootCmd.AddCommand(azureCmd)
}
