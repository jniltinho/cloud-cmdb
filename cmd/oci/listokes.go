package oci

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	ociapi "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listOKEsTables      bool
	listOKEsText        bool
	listOKEsCSV         bool
	listOKEsPDF         bool
	listOKEsCompartment string
	listOKEsFull        bool
)

// ListOKEsCmd is the cobra command for "cloud-cmdb oci list-okes".
var ListOKEsCmd = &cobra.Command{
	Use:   "list-okes",
	Short: "List Kubernetes (OKE) clusters across the tenancy or in a specific compartment",
	Long: `List Oracle Container Engine for Kubernetes (OKE) clusters.

By default (no --compartment flag) it enumerates ALL compartments and returns
every cluster with its name, Kubernetes version, lifecycle state, compartment name and VCN name.

Default table columns (human-friendly):
  - NAME          (cluster display name)
  - VERSION       (Kubernetes version on control plane)
  - TYPE          (BASIC_CLUSTER or ENHANCED_CLUSTER)
  - STATE         (ACTIVE, CREATING, UPDATING, FAILED, etc.)
  - CREATED       (creation date)
  - UPDATED       (last update date)
  - VCN           (VCN display name — resolved automatically)
  - COMPARTMENT

Use --compartment to restrict to a single compartment (name or OCID supported).

Use --full to include additional columns: CREATED BY, TAGS, ENDPOINT, CLUSTER OCID and VCN OCID.

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row (great for Excel / Google Sheets)

Clusters in DELETED or DELETING state are omitted (matching other list commands).`,
	Example: `  # List every OKE cluster in the entire tenancy (table with friendly names)
  cloud-cmdb oci list-okes

  # Restrict to one compartment by name
  cloud-cmdb oci list-okes --compartment Production

  # By compartment OCID
  cloud-cmdb oci list-okes --compartment ocid1.compartment.oc1...

  # Tab-separated text (great for scripts / awk / cut)
  cloud-cmdb oci list-okes --text

  # Include CREATED BY, TAGS, ENDPOINT + full OCIDs (extra columns)
  cloud-cmdb oci list-okes --full

  # Full details (with CREATED BY and TAGS) + text mode
  cloud-cmdb oci list-okes --text --full

  # Export to CSV (redirect to file for Excel)
  cloud-cmdb oci list-okes --csv > clusters.csv

  # CSV with CREATED BY, TAGS, full OCIDs and endpoints
  cloud-cmdb oci list-okes --csv --full

  # With a non-default profile
  cloud-cmdb oci list-okes --profile PROD

  # PDF with custom name
  cloud-cmdb oci list-okes --pdf --pdf-out oke-clusters.pdf`,
	RunE: runListOKEs,
}

func init() {
	ListOKEsCmd.Flags().BoolVar(&listOKEsTables, "tables", false, "Output as formatted table (default)")
	ListOKEsCmd.Flags().BoolVar(&listOKEsText, "text", false, "Output as plain text (tab-separated)")
	ListOKEsCmd.Flags().BoolVar(&listOKEsCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListOKEsCmd.Flags().BoolVar(&listOKEsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListOKEsCmd.Flags().StringVar(&listOKEsCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
	ListOKEsCmd.Flags().BoolVar(&listOKEsFull, "full", false, "Show CREATED BY, TAGS, ENDPOINT, CLUSTER OCID and VCN OCID")
}

func runListOKEs(cmd *cobra.Command, args []string) error {
	if listOKEsTables && listOKEsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listOKEsCSV && (listOKEsTables || listOKEsText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listOKEsPDF && (listOKEsCSV || listOKEsText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	entries, err := ociapi.ListOKEClusters(context.Background(), listOKEsCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if listOKEsCompartment != "" {
			fmt.Println("No OKE clusters found in the specified compartment.")
		} else {
			fmt.Println("No OKE clusters found in the tenancy.")
		}
		return nil
	}

	if listOKEsPDF {
		var headers []string
		if listOKEsFull {
			headers = []string{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "CREATED BY", "UPDATED", "VCN", "COMPARTMENT", "TAGS", "ENDPOINT", "CLUSTER OCID", "VCN OCID"}
		} else {
			headers = []string{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "UPDATED", "VCN", "COMPARTMENT"}
		}
		data := make([][]string, len(entries))
		for i, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, listOKEsFull)
			}
			endpoint := e.Endpoint
			if endpoint == "" {
				endpoint = "-"
			}
			typeStr := e.Type
			if typeStr == "" {
				typeStr = "-"
			}
			created := e.Created
			if created == "" {
				created = "-"
			}
			updated := e.Updated
			if updated == "" {
				updated = "-"
			}
			createdBy := e.CreatedBy
			if createdBy == "" {
				createdBy = "-"
			}
			tags := e.Tags
			if tags == "" {
				tags = "-"
			}

			if listOKEsFull {
				data[i] = []string{e.Name, e.KubernetesVersion, typeStr, e.State, created, createdBy, updated, vcn, e.CompartmentName, tags, endpoint, e.OCID, e.VcnId}
			} else {
				data[i] = []string{e.Name, e.KubernetesVersion, typeStr, e.State, created, updated, vcn, e.CompartmentName}
			}
		}
		title := "OCI OKE Clusters"
		if listOKEsFull {
			title = "OCI OKE Clusters (Full)"
		}
		return generateAndSavePDF(cmd, title, headers, data)
	}

	if listOKEsCSV {
		w := csv.NewWriter(os.Stdout)

		var header []string
		if listOKEsFull {
			header = []string{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "CREATED BY", "UPDATED", "VCN", "COMPARTMENT", "TAGS", "ENDPOINT", "CLUSTER OCID", "VCN OCID"}
		} else {
			header = []string{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "UPDATED", "VCN", "COMPARTMENT"}
		}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, listOKEsFull)
			}
			endpoint := e.Endpoint
			if endpoint == "" {
				endpoint = "-"
			}
			typeStr := e.Type
			if typeStr == "" {
				typeStr = "-"
			}
			created := e.Created
			if created == "" {
				created = "-"
			}
			updated := e.Updated
			if updated == "" {
				updated = "-"
			}
			createdBy := e.CreatedBy
			if createdBy == "" {
				createdBy = "-"
			}
			tags := e.Tags
			if tags == "" {
				tags = "-"
			}

			var row []string
			if listOKEsFull {
				row = []string{e.Name, e.KubernetesVersion, typeStr, e.State, created, createdBy, updated, vcn, e.CompartmentName, tags, endpoint, e.OCID, e.VcnId}
			} else {
				row = []string{e.Name, e.KubernetesVersion, typeStr, e.State, created, updated, vcn, e.CompartmentName}
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

	if listOKEsText {
		for _, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, listOKEsFull)
			}
			endpoint := e.Endpoint
			if endpoint == "" {
				endpoint = "-"
			}
			typeStr := e.Type
			if typeStr == "" {
				typeStr = "-"
			}
			created := e.Created
			if created == "" {
				created = "-"
			}
			updated := e.Updated
			if updated == "" {
				updated = "-"
			}
			createdBy := e.CreatedBy
			if createdBy == "" {
				createdBy = "-"
			}
			tags := e.Tags
			if tags == "" {
				tags = "-"
			}

			if listOKEsFull {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					e.Name, e.KubernetesVersion, typeStr, e.State, created, createdBy, updated, vcn, e.CompartmentName, tags, endpoint, e.OCID, e.VcnId)
			} else {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					e.Name, e.KubernetesVersion, typeStr, e.State, created, updated, vcn, e.CompartmentName)
			}
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if listOKEsFull {
		t.AppendHeader(table.Row{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "CREATED BY", "UPDATED", "VCN", "COMPARTMENT", "TAGS", "ENDPOINT", "CLUSTER OCID", "VCN OCID"})
		for _, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, listOKEsFull)
			}
			endpoint := e.Endpoint
			if endpoint == "" {
				endpoint = "-"
			}
			typeStr := e.Type
			if typeStr == "" {
				typeStr = "-"
			}
			created := e.Created
			if created == "" {
				created = "-"
			}
			updated := e.Updated
			if updated == "" {
				updated = "-"
			}
			createdBy := e.CreatedBy
			if createdBy == "" {
				createdBy = "-"
			}
			tags := e.Tags
			if tags == "" {
				tags = "-"
			}
			t.AppendRow(table.Row{e.Name, e.KubernetesVersion, typeStr, e.State, created, createdBy, updated, vcn, e.CompartmentName, tags, endpoint, e.OCID, e.VcnId})
		}
		t.AppendFooter(table.Row{"", "", "", "", "", "", "", "", "", "", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	} else {
		t.AppendHeader(table.Row{"NAME", "VERSION", "TYPE", "STATE", "CREATED", "UPDATED", "VCN", "COMPARTMENT"})
		for _, e := range entries {
			vcn := e.VcnName
			if vcn == "" {
				vcn = abbreviateOCID(e.VcnId, listOKEsFull)
			}
			typeStr := e.Type
			if typeStr == "" {
				typeStr = "-"
			}
			created := e.Created
			if created == "" {
				created = "-"
			}
			updated := e.Updated
			if updated == "" {
				updated = "-"
			}
			t.AppendRow(table.Row{e.Name, e.KubernetesVersion, typeStr, e.State, created, updated, vcn, e.CompartmentName})
		}
		t.AppendFooter(table.Row{"", "", "", "", "", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	}

	t.Render()
	return nil
}
