package oci

import (
	"context"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	ociapi "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listCompartmentsTables bool
	listCompartmentsText   bool
	listCompartmentsPDF    bool
)

// ListCompartmentsCmd is the cobra command for "cloud-cmdb oci list-compartments".
var ListCompartmentsCmd = &cobra.Command{
	Use:   "list-compartments",
	Short: "List all compartments (name + OCID) in the tenancy",
	Long: `List all compartments under the tenancy, including the root tenancy itself.

Each entry shows:
  - Name (display name)
  - OCID  (full compartment or tenancy OCID)

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (one line per compartment: name<TAB>ocid)
  --pdf      PDF file with title + formatted table

Useful for scripting or quickly finding OCIDs by name.`,
	Example: `  # Default table output (name + OCID)
  cloud-cmdb oci list-compartments

  # Explicit table
  cloud-cmdb oci list-compartments --tables

  # Plain text (tab-separated, good for scripts/awk)
  cloud-cmdb oci list-compartments --text

  # PDF report
  cloud-cmdb oci list-compartments --pdf

  # Custom PDF filename
  cloud-cmdb oci list-compartments --pdf --pdf-out compartments.pdf

  # With a specific profile
  cloud-cmdb oci list-compartments --profile PROD`,
	RunE: runListCompartments,
}

func init() {
	ListCompartmentsCmd.Flags().BoolVar(&listCompartmentsTables, "tables", false, "Output as formatted table (default)")
	ListCompartmentsCmd.Flags().BoolVar(&listCompartmentsText, "text", false, "Output as plain text (name + OCID per line)")
	ListCompartmentsCmd.Flags().BoolVar(&listCompartmentsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
}

func runListCompartments(cmd *cobra.Command, args []string) error {
	if listCompartmentsTables && listCompartmentsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listCompartmentsPDF && listCompartmentsText {
		return fmt.Errorf("cannot use --pdf and --text together")
	}

	entries, err := ociapi.ListAllCompartments(context.Background(), configFile, profile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		fmt.Println("No compartments found.")
		return nil
	}

	if listCompartmentsPDF {
		headers := []string{"NAME", "OCID"}
		data := make([][]string, len(entries))
		for i, e := range entries {
			data[i] = []string{e.Name, e.OCID}
		}
		return generateAndSavePDF(cmd, "OCI Compartments", headers, data)
	}

	if listCompartmentsText {
		for _, e := range entries {
			fmt.Printf("%s\t%s\n", e.Name, e.OCID)
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"NAME", "OCID"})

	for _, e := range entries {
		t.AppendRow(table.Row{e.Name, e.OCID})
	}

	t.AppendFooter(table.Row{"", fmt.Sprintf("TOTAL: %d", len(entries))})
	t.Render()
	return nil
}
