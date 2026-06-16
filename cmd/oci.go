package cmd

import (
	cmdoci "cloud-cmdb/cmd/oci"
	"github.com/spf13/cobra"
)

var ociCmd = &cobra.Command{
	Use:   "oci",
	Short: "Oracle Cloud Infrastructure commands",
	Long: `Commands for Oracle Cloud Infrastructure (OCI) resources.

Global OCI flags (available to all oci subcommands):
  --config-file   Path to OCI config file (default: ~/.oci/config)
  --profile       OCI config profile name (default: DEFAULT)
  --pdf-out       Custom PDF output filename when using --pdf

Output format flags (per subcommand): --tables (default), --text, --csv, --pdf`,
	Example: `  # List all compartments
  cloud-cmdb oci list-compartments

  # List VCNs in a compartment
  cloud-cmdb oci list-vcns --compartment Production

  # List all instances with PDF output
  cloud-cmdb oci list-instances --pdf --pdf-out servidores.pdf

  # List OKE clusters
  cloud-cmdb oci list-okes --compartment Production

  # List subnets with OCID details
  cloud-cmdb oci list-subnets --full

  # List buckets as CSV
  cloud-cmdb oci list-buckets --csv > buckets.csv

  # List private IPs in a subnet
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --profile PROD`,
}

func init() {
	cmdoci.RegisterPersistentFlags(ociCmd)
	ociCmd.AddCommand(
		cmdoci.ListBucketsCmd,
		cmdoci.ListCompartmentsCmd,
		cmdoci.ListInstancesCmd,
		cmdoci.ListOKEsCmd,
		cmdoci.ListPrivateIPsCmd,
		cmdoci.ListServersCmd,
		cmdoci.ListSubnetsCmd,
		cmdoci.ListVcnsCmd,
	)
	rootCmd.AddCommand(ociCmd)
}
