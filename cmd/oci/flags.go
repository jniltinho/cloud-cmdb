// Package oci implements the "cloud-cmdb oci" subcommand group, registering
// list-buckets, list-compartments, list-instances, list-okes, list-private-ips,
// list-servers, list-subnets, and list-vcns commands.
package oci

import (
	"fmt"
	"os"

	"cloud-cmdb/internal/pdf"
	"github.com/spf13/cobra"
)

var (
	configFile string
	profile    string
	pdfOut     string
)

// RegisterPersistentFlags binds OCI-wide persistent flags onto the given command.
func RegisterPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&configFile, "config-file", "", "Path to OCI config file (default: ~/.oci/config)")
	cmd.PersistentFlags().StringVar(&profile, "profile", "", "OCI config profile name (default: DEFAULT)")
	cmd.PersistentFlags().StringVar(&pdfOut, "pdf-out", "", "Custom output filename when using --pdf (e.g. --pdf-out relatorio.pdf)")
}

func generateAndSavePDF(cmd *cobra.Command, title string, headers []string, rows [][]string) error {
	pdfBytes, err := pdf.GenerateTablePDF(title, headers, rows)
	if err != nil {
		return fmt.Errorf("failed to generate PDF: %w", err)
	}
	name := cmd.Use
	if name == "" {
		name = cmd.CommandPath()
	}
	filename := pdf.ResolvePDFFilename(pdfOut, name)
	if err := os.WriteFile(filename, pdfBytes, 0644); err != nil {
		return fmt.Errorf("failed to write PDF file %s: %w", filename, err)
	}
	fmt.Printf("PDF generated: %s (%d rows)\n", filename, len(rows))
	return nil
}
