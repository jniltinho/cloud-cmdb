// Package gcp implements the "cloud-cmdb gcp" subcommand group,
// registering list-instances using Application Default Credentials (ADC).
package gcp

import (
	"fmt"
	"os"

	"cloud-cmdb/internal/pdf"
	"github.com/spf13/cobra"
)

var (
	project string
	pdfOut  string
)

// RegisterPersistentFlags binds GCP-wide persistent flags onto the given command.
func RegisterPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&project, "project", "", "GCP project ID (default: GOOGLE_CLOUD_PROJECT env var)")
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
