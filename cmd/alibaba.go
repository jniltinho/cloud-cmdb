package cmd

import (
	cmdalibaba "cloud-cmdb/cmd/alibaba"
	"github.com/spf13/cobra"
)

var alibabaCmd = &cobra.Command{
	Use:   "alibaba",
	Short: "Alibaba Cloud commands",
	Long: `Commands for Alibaba Cloud ECS resources.

Authentication uses Access Key credentials:
  1. --access-key-id / --access-key-secret flags
  2. ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variables

The region is read from --region flag or ALIBABA_CLOUD_REGION env var (default: cn-hangzhou).

Global Alibaba Cloud flags (available to all alibaba subcommands):
  --region            Alibaba Cloud region (e.g. cn-hangzhou, ap-southeast-1)
  --access-key-id     Access Key ID
  --access-key-secret Access Key Secret
  --pdf-out           Custom PDF output filename when using --pdf

Output format flags (per subcommand): --tables (default), --text, --csv, --pdf`,
	Example: `  # List ECS instances in the default region (cn-hangzhou)
  cloud-cmdb alibaba list-instances

  # Specific region
  cloud-cmdb alibaba list-instances --region ap-southeast-1

  # Filter by name and export to CSV
  cloud-cmdb alibaba list-instances --contains=web- --csv > ecs.csv

  # PDF report
  cloud-cmdb alibaba list-instances --pdf --pdf-out ecs-inventory.pdf`,
}

func init() {
	cmdalibaba.RegisterPersistentFlags(alibabaCmd)
	alibabaCmd.AddCommand(
		cmdalibaba.ListInstancesCmd,
	)
	rootCmd.AddCommand(alibabaCmd)
}
