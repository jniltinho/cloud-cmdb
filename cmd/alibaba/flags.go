// Package alibaba implements the "cloud-cmdb alibaba" subcommand group,
// registering list-instances using the alibabacloud-go ECS SDK with
// Access Key credentials.
package alibaba

import (
	"fmt"
	"os"

	"cloud-cmdb/internal/pdf"
	"github.com/spf13/cobra"
)

var (
	region          string
	accessKeyID     string
	accessKeySecret string
	pdfOut          string
)

// RegisterPersistentFlags binds Alibaba Cloud-wide persistent flags onto the given command.
func RegisterPersistentFlags(cmd *cobra.Command) {
	cmd.PersistentFlags().StringVar(&region, "region", "", "Alibaba Cloud region (default: ALIBABA_CLOUD_REGION env var or cn-hangzhou)")
	cmd.PersistentFlags().StringVar(&accessKeyID, "access-key-id", "", "Access Key ID (default: ALIBABA_CLOUD_ACCESS_KEY_ID env var)")
	cmd.PersistentFlags().StringVar(&accessKeySecret, "access-key-secret", "", "Access Key Secret (default: ALIBABA_CLOUD_ACCESS_KEY_SECRET env var)")
	cmd.PersistentFlags().StringVar(&pdfOut, "pdf-out", "", "Custom output filename when using --pdf (e.g. --pdf-out report.pdf)")
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
