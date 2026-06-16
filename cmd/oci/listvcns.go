package oci

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	ociinternal "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listVcnsTables      bool
	listVcnsText        bool
	listVcnsCSV         bool
	listVcnsPDF         bool
	listVcnsCompartment string
)

// ListVcnsCmd is the cobra command for "cloud-cmdb oci list-vcns".
var ListVcnsCmd = &cobra.Command{
	Use:   "list-vcns",
	Short: "List VCNs with name, CIDR range and compartment name",
	Long: `List Virtual Cloud Networks (VCNs) in the tenancy.

By default (no --compartment flag) it enumerates ALL compartments and returns
every VCN with its display name, primary CIDR block, and compartment name.

Default table columns (human-friendly names):
  - NAME (VCN display name)
  - CIDR (primary IPv4 CIDR block)
  - COMPARTMENT (compartment display name)
  - VCN OCID (full OCID of the VCN)

Use --compartment to restrict to a single compartment (name or OCID supported).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row (great for Excel / Google Sheets)
  --pdf      PDF file with title + formatted table (e.g. list-vcns.pdf)

Useful for quickly seeing VCN CIDR ranges and which compartment they belong to.`,
	Example: `  # List every VCN in the entire tenancy (table with names)
  cloud-cmdb oci list-vcns

  # Restrict to one compartment by name
  cloud-cmdb oci list-vcns --compartment Production

  # By compartment OCID
  cloud-cmdb oci list-vcns --compartment ocid1.compartment.oc1...

  # Tab-separated text (great for scripts / awk / cut)
  cloud-cmdb oci list-vcns --text

  # Export to CSV (redirect to file for Excel)
  cloud-cmdb oci list-vcns --csv > vcns.csv

  # Generate PDF report
  cloud-cmdb oci list-vcns --pdf

  # PDF with custom filename
  cloud-cmdb oci list-vcns --pdf --pdf-out relatorio-vcns.pdf

  # With a non-default profile
  cloud-cmdb oci list-vcns --profile PROD`,
	RunE: runListVcns,
}

func init() {
	ListVcnsCmd.Flags().BoolVar(&listVcnsTables, "tables", false, "Output as formatted table (default)")
	ListVcnsCmd.Flags().BoolVar(&listVcnsText, "text", false, "Output as plain text (tab-separated)")
	ListVcnsCmd.Flags().BoolVar(&listVcnsCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListVcnsCmd.Flags().BoolVar(&listVcnsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListVcnsCmd.Flags().StringVar(&listVcnsCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
}

func runListVcns(cmd *cobra.Command, args []string) error {
	if listVcnsTables && listVcnsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listVcnsCSV && (listVcnsTables || listVcnsText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listVcnsPDF && (listVcnsCSV || listVcnsText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	entries, err := ociinternal.ListVcns(context.Background(), listVcnsCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if listVcnsCompartment != "" {
			fmt.Println("No VCNs found in the specified compartment.")
		} else {
			fmt.Println("No VCNs found in the tenancy.")
		}
		return nil
	}

	if listVcnsPDF {
		headers := []string{"NAME", "CIDR", "COMPARTMENT", "VCN OCID"}
		data := make([][]string, len(entries))
		for i, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcnOcid := e.OCID
			if vcnOcid == "" {
				vcnOcid = "-"
			}
			data[i] = []string{e.Name, cidr, e.CompartmentName, vcnOcid}
		}
		return generateAndSavePDF(cmd, "OCI VCNs", headers, data)
	}

	if listVcnsCSV {
		w := csv.NewWriter(os.Stdout)

		header := []string{"NAME", "CIDR", "COMPARTMENT", "VCN OCID"}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcnOcid := e.OCID
			if vcnOcid == "" {
				vcnOcid = "-"
			}
			row := []string{e.Name, cidr, e.CompartmentName, vcnOcid}
			if err := w.Write(row); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}

		w.Flush()
		if err := w.Error(); err != nil {
			return fmt.Errorf("failed to flush CSV: %w", err)
		}
		return nil
	}

	if listVcnsText {
		for _, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcnOcid := e.OCID
			if vcnOcid == "" {
				vcnOcid = "-"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", e.Name, cidr, e.CompartmentName, vcnOcid)
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"NAME", "CIDR", "COMPARTMENT", "VCN OCID"})

	for _, e := range entries {
		cidr := e.CidrBlock
		if cidr == "" {
			cidr = "-"
		}
		vcnOcid := e.OCID
		if vcnOcid == "" {
			vcnOcid = "-"
		}
		t.AppendRow(table.Row{e.Name, cidr, e.CompartmentName, vcnOcid})
	}

	t.AppendFooter(table.Row{"", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	t.Render()
	return nil
}
