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
	listBucketsTables      bool
	listBucketsText        bool
	listBucketsCSV         bool
	listBucketsPDF         bool
	listBucketsCompartment string
)

// ListBucketsCmd is the cobra command for "cloud-cmdb oci list-buckets".
var ListBucketsCmd = &cobra.Command{
	Use:   "list-buckets",
	Short: "List buckets with name, namespace, compartment and creation time",
	Long: `List Object Storage buckets in the tenancy.

By default (no --compartment flag) it enumerates ALL compartments and returns
every bucket with its name, namespace, compartment name, and creation time.

Default table columns (human-friendly names):
  - NAME (bucket name)
  - NAMESPACE (Object Storage namespace)
  - COMPARTMENT (compartment display name)
  - CREATED (creation timestamp)

Use --compartment to restrict to a single compartment (name or OCID supported).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row (great for Excel / Google Sheets)
  --pdf      PDF file with title + formatted table (e.g. list-buckets.pdf)

The namespace is retrieved automatically and is required for all Object Storage operations.`,
	Example: `  # List every bucket in the entire tenancy (table with names)
  cloud-cmdb oci list-buckets

  # Restrict to one compartment by name
  cloud-cmdb oci list-buckets --compartment Production

  # By compartment OCID
  cloud-cmdb oci list-buckets --compartment ocid1.compartment.oc1...

  # Tab-separated text (great for scripts / awk / cut)
  cloud-cmdb oci list-buckets --text

  # Export to CSV (redirect to file for Excel)
  cloud-cmdb oci list-buckets --csv > buckets.csv

  # Generate PDF report (styled table + title)
  cloud-cmdb oci list-buckets --pdf

  # PDF with custom output name
  cloud-cmdb oci list-buckets --pdf --pdf-out buckets.pdf

  # With a non-default profile
  cloud-cmdb oci list-buckets --profile PROD`,
	RunE: runListBuckets,
}

func init() {
	ListBucketsCmd.Flags().BoolVar(&listBucketsTables, "tables", false, "Output as formatted table (default)")
	ListBucketsCmd.Flags().BoolVar(&listBucketsText, "text", false, "Output as plain text (tab-separated)")
	ListBucketsCmd.Flags().BoolVar(&listBucketsCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListBucketsCmd.Flags().BoolVar(&listBucketsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListBucketsCmd.Flags().StringVar(&listBucketsCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
}

func runListBuckets(cmd *cobra.Command, args []string) error {
	if listBucketsTables && listBucketsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listBucketsCSV && (listBucketsTables || listBucketsText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listBucketsPDF && (listBucketsCSV || listBucketsText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	entries, err := ociinternal.ListBuckets(context.Background(), listBucketsCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(entries) == 0 {
		if listBucketsCompartment != "" {
			fmt.Println("No buckets found in the specified compartment.")
		} else {
			fmt.Println("No buckets found in the tenancy.")
		}
		return nil
	}

	if listBucketsPDF {
		headers := []string{"NAME", "COMPARTMENT", "CREATED"}
		data := make([][]string, len(entries))
		for i, e := range entries {
			created := e.Created
			if created == "" {
				created = "-"
			}
			data[i] = []string{e.Name, e.CompartmentName, created}
		}
		return generateAndSavePDF(cmd, "OCI Buckets", headers, data)
	}

	if listBucketsCSV {
		w := csv.NewWriter(os.Stdout)

		header := []string{"NAME", "NAMESPACE", "COMPARTMENT", "CREATED"}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, e := range entries {
			created := e.Created
			if created == "" {
				created = "-"
			}
			row := []string{e.Name, e.Namespace, e.CompartmentName, created}
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

	if listBucketsText {
		for _, e := range entries {
			created := e.Created
			if created == "" {
				created = "-"
			}
			fmt.Printf("%s\t%s\t%s\t%s\n", e.Name, e.Namespace, e.CompartmentName, created)
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"NAME", "NAMESPACE", "COMPARTMENT", "CREATED"})

	for _, e := range entries {
		created := e.Created
		if created == "" {
			created = "-"
		}
		t.AppendRow(table.Row{e.Name, e.Namespace, e.CompartmentName, created})
	}

	t.AppendFooter(table.Row{"", "", "", fmt.Sprintf("TOTAL: %d", len(entries))})
	t.Render()
	return nil
}
