package oci

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	ociinternal "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listSubnetsTables      bool
	listSubnetsText        bool
	listSubnetsCSV         bool
	listSubnetsPDF         bool
	listSubnetsCompartment string
	listSubnetsFull        bool
)

// ListSubnetsCmd is the cobra command for "cloud-cmdb oci list-subnets".
var ListSubnetsCmd = &cobra.Command{
	Use:   "list-subnets",
	Short: "List all subnets with name, CIDR, VCN name, compartment and allocated IP count",
	Long: `List subnets in the tenancy.

By default (no --compartment flag) it enumerates ALL compartments and returns
every subnet with its display name, CIDR block, VCN name, and compartment name.

Default table columns (human-friendly names):
  - NAME (subnet display name)
  - CIDR (primary IPv4 CIDR block)
  - ALLOCATED (number of private IPs currently assigned in the subnet)
  - VCN (VCN display name — resolved automatically)
  - COMPARTMENT (compartment display name)

When --full is used, "SUBNET OCID" is added as the last column.

The ALLOCATED column is always shown (it makes additional ListPrivateIps API calls).

Use --full to include the subnet OCID as an additional column (full value).

Use --compartment to restrict to a single compartment (name or OCID supported).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row (great for Excel / Google Sheets)
  --pdf      PDF report (styled table + title, supports --pdf-out)

Useful for quickly finding subnet CIDRs and which VCN/compartment they belong to.`,
	Example: `  # List every subnet in the entire tenancy (table with names)
  cloud-cmdb oci list-subnets

  # Restrict to one compartment by name
  cloud-cmdb oci list-subnets --compartment Production

  # By compartment OCID
  cloud-cmdb oci list-subnets --compartment ocid1.compartment.oc1...

  # Tab-separated text (great for scripts / awk / cut)
  cloud-cmdb oci list-subnets --text

  # Include full subnet OCID (extra column)
  cloud-cmdb oci list-subnets --full

  # Full OCID + text mode
  cloud-cmdb oci list-subnets --text --full

  # Export to CSV (redirect to file for Excel)
  cloud-cmdb oci list-subnets --csv > subnets.csv

  # CSV with full subnet OCID
  cloud-cmdb oci list-subnets --csv --full

  # Export utilization report to CSV (includes ALLOCATED by default)
  cloud-cmdb oci list-subnets --csv > subnet-utilization.csv

  # PDF report including allocated IP counts
  cloud-cmdb oci list-subnets --pdf --pdf-out subnets-alocacao.pdf

  # With a non-default profile
  cloud-cmdb oci list-subnets --profile PROD`,
	RunE: runListSubnets,
}

func init() {
	ListSubnetsCmd.Flags().BoolVar(&listSubnetsTables, "tables", false, "Output as formatted table (default)")
	ListSubnetsCmd.Flags().BoolVar(&listSubnetsText, "text", false, "Output as plain text (tab-separated)")
	ListSubnetsCmd.Flags().BoolVar(&listSubnetsCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListSubnetsCmd.Flags().BoolVar(&listSubnetsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListSubnetsCmd.Flags().StringVar(&listSubnetsCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
	ListSubnetsCmd.Flags().BoolVar(&listSubnetsFull, "full", false, "Show subnet OCID in an additional column (full value)")
}

// abbreviateOCID returns a short form of the OCID (used as fallback when name is unavailable).
func abbreviateOCID(ocid string, forceFull bool) string {
	if ocid == "" {
		return "-"
	}
	if forceFull || len(ocid) <= 40 {
		return ocid
	}
	parts := strings.Split(ocid, ".")
	if len(parts) >= 5 {
		last := parts[len(parts)-1]
		if len(last) > 8 {
			last = last[len(last)-8:]
		}
		return parts[0] + "." + parts[1] + "." + parts[2] + "..." + last
	}
	return ocid[:20] + "..." + ocid[len(ocid)-8:]
}

func runListSubnets(cmd *cobra.Command, args []string) error {
	if listSubnetsTables && listSubnetsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listSubnetsCSV && (listSubnetsTables || listSubnetsText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listSubnetsPDF && (listSubnetsCSV || listSubnetsText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	entries, err := ociinternal.ListSubnets(context.Background(), listSubnetsCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if listSubnetsCompartment != "" {
			fmt.Println("No subnets found in the specified compartment.")
		} else {
			fmt.Println("No subnets found in the tenancy.")
		}
		return nil
	}

	if listSubnetsPDF {
		showFull := listSubnetsFull

		var headers []string
		if showFull {
			headers = []string{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT", "SUBNET OCID"}
		} else {
			headers = []string{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT"}
		}

		data := make([][]string, len(entries))
		for i, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, showFull)
			}
			alloc := fmt.Sprintf("%d", e.AllocatedIPs)

			if showFull {
				data[i] = []string{e.Name, cidr, alloc, vcn, e.CompartmentName, e.OCID}
			} else {
				data[i] = []string{e.Name, cidr, alloc, vcn, e.CompartmentName}
			}
		}

		title := "OCI Subnets"
		if showFull {
			title = "OCI Subnets (Full)"
		}
		return generateAndSavePDF(cmd, title, headers, data)
	}

	if listSubnetsCSV {
		w := csv.NewWriter(os.Stdout)

		showFull := listSubnetsFull

		var header []string
		if showFull {
			header = []string{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT", "SUBNET OCID"}
		} else {
			header = []string{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT"}
		}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, showFull)
			}
			alloc := fmt.Sprintf("%d", e.AllocatedIPs)

			var row []string
			if showFull {
				row = []string{e.Name, cidr, alloc, vcn, e.CompartmentName, e.OCID}
			} else {
				row = []string{e.Name, cidr, alloc, vcn, e.CompartmentName}
			}
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

	if listSubnetsText {
		showFull := listSubnetsFull

		for _, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, showFull)
			}
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			alloc := fmt.Sprintf("%d", e.AllocatedIPs)

			if showFull {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\n",
					e.Name, cidr, alloc, vcn, e.CompartmentName, e.OCID)
			} else {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\n",
					e.Name, cidr, alloc, vcn, e.CompartmentName)
			}
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	showFull := listSubnetsFull

	if showFull {
		t.AppendHeader(table.Row{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT", "SUBNET OCID"})
		for _, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, showFull)
			}
			t.AppendRow(table.Row{e.Name, cidr, e.AllocatedIPs, vcn, e.CompartmentName, e.OCID})
		}
		t.AppendFooter(table.Row{"", "", "", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	} else {
		t.AppendHeader(table.Row{"NAME", "CIDR", "ALLOCATED", "VCN", "COMPARTMENT"})
		for _, e := range entries {
			cidr := e.CidrBlock
			if cidr == "" {
				cidr = "-"
			}
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, showFull)
			}
			t.AppendRow(table.Row{e.Name, cidr, e.AllocatedIPs, vcn, e.CompartmentName})
		}
		t.AppendFooter(table.Row{"", "", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	}

	t.Render()
	return nil
}
