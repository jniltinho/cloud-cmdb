package cmd

import (
	cmdaws "cloud-cmdb/cmd/aws"
	"github.com/spf13/cobra"
)

var awsCmd = &cobra.Command{
	Use:   "aws",
	Short: "Amazon Web Services commands",
	Long: `Commands for Amazon Web Services (AWS) resources.

Uses the standard AWS credential chain:
  1. Environment variables (AWS_ACCESS_KEY_ID, AWS_SECRET_ACCESS_KEY)
  2. ~/.aws/credentials and ~/.aws/config
  3. IAM instance profile (when running on EC2)

Global AWS flags (available to all aws subcommands):
  --region    AWS region (e.g. us-east-1). Defaults to AWS_DEFAULT_REGION or ~/.aws/config
  --profile   AWS named profile from ~/.aws/credentials (default: default)
  --pdf-out   Custom PDF output filename when using --pdf

Output format flags (per subcommand): --tables (default), --text, --csv, --pdf`,
	Example: `  # List EC2 instances in the default region
  cloud-cmdb aws list-instances

  # Specific region with named profile
  cloud-cmdb aws list-instances --region us-east-1 --profile production

  # Filter by name tag and export to CSV
  cloud-cmdb aws list-instances --contains=web- --csv > web-instances.csv

  # PDF report for a region
  cloud-cmdb aws list-instances --region eu-west-1 --pdf --pdf-out ec2-eu.pdf`,
}

func init() {
	cmdaws.RegisterPersistentFlags(awsCmd)
	awsCmd.AddCommand(
		cmdaws.ListInstancesCmd,
	)
	rootCmd.AddCommand(awsCmd)
}
