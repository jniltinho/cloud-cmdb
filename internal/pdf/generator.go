// Package pdf generates styled table PDF documents for cloud-cmdb list commands.
// It wraps the maroto v2 library. All provider packages call GenerateTablePDF
// when the --pdf flag is set.
package pdf

import (
	"fmt"
	"math"
	"path/filepath"
	"strings"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/consts/align"
	"github.com/johnfercher/maroto/v2/pkg/consts/border"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontfamily"
	"github.com/johnfercher/maroto/v2/pkg/consts/fontstyle"
	"github.com/johnfercher/maroto/v2/pkg/consts/orientation"
	"github.com/johnfercher/maroto/v2/pkg/core"
	"github.com/johnfercher/maroto/v2/pkg/props"
)

// GenerateTablePDF creates a PDF document with a title and a styled table
// (title first, then dark blue header row directly below it, then data rows
// with alternating light gray/white background + full borders on all cells
// for visible row and column lines).
// Column widths are calculated proportionally so the widest value in each column fits.
// Tables with 3 or more columns are always generated in landscape (Horizontal) orientation.
// Tables with 1-2 columns are generated in portrait (Vertical).
// Inspired by the reportlab GRID style from dist/get_vms_oci.py.
// Returns the raw PDF bytes. Caller is responsible for saving to file.
func GenerateTablePDF(title string, headers []string, rows [][]string) ([]byte, error) {
	if len(headers) == 0 {
		return nil, fmt.Errorf("headers cannot be empty")
	}

	orient := orientation.Vertical
	if needsLandscape(headers, rows) {
		orient = orientation.Horizontal
	}

	// Column widths are calculated to fit the longest string in each column.
	sizes := buildColSizes(headers, rows)

	cfg := config.NewBuilder().
		WithPageNumber().
		WithLeftMargin(8).
		WithRightMargin(8).
		WithTopMargin(10).
		WithOrientation(orient).
		Build()

	mrt := maroto.New(cfg)
	m := maroto.NewMetricsDecorator(mrt)

	// Title on first page only (above the table header)
	if strings.TrimSpace(title) != "" {
		fullTitle := fmt.Sprintf("Bionexo Tasy Cloud - %s -> Total: %d", title, len(rows))
		m.AddRows(text.NewRow(11, fullTitle, props.Text{
			Top:    1,
			Size:   14,
			Style:  fontstyle.Bold,
			Align:  align.Center,
			Family: fontfamily.Arial,
		}))
		// small vertical spacer row
		m.AddRow(3, col.New(12))
	}

	// Table header row (blue styled column headers) - placed directly under title
	// to match the layout in dist/tabela.png. Header does not repeat on page breaks.
	// Top+Bottom padding keeps header text vertically centered inside the blue cells.
	headerRow := buildHeaderRow(headers, sizes)
	m.AddRows(headerRow)

	// Data rows (alternating row colors)
	for i, r := range rows {
		dataRow := buildDataRow(sizes, r, i%2 == 1)
		m.AddRows(dataRow)
	}

	// Handle empty data case gracefully
	if len(rows) == 0 {
		m.AddRows(text.NewRow(10, "No data found.", props.Text{
			Top:   4,
			Size:  10,
			Align: align.Center,
		}))
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("failed to generate PDF: %w", err)
	}
	return doc.GetBytes(), nil
}

// DefaultPDFFilename returns a safe filename based on the cobra command path.
func DefaultPDFFilename(cmdPath string) string {
	safe := strings.Map(func(r rune) rune {
		switch {
		case r >= 'a' && r <= 'z':
			return r
		case r >= 'A' && r <= 'Z':
			return r
		case r >= '0' && r <= '9':
			return r
		case r == '-', r == '_', r == '.':
			return r
		default:
			return '-'
		}
	}, cmdPath)

	safe = strings.Trim(safe, "-.")
	if safe == "" {
		safe = "output"
	}
	if !strings.HasSuffix(strings.ToLower(safe), ".pdf") {
		safe += ".pdf"
	}
	return safe
}

// ResolvePDFFilename returns the filename to use for PDF output.
// If userProvided is non-empty (after trim), it ensures the basename ends with .pdf
// (without mangling directory components). This allows users to specify custom names
// or even paths via --pdf-out.
// If userProvided is empty, it falls back to DefaultPDFFilename(cmdPath).
func ResolvePDFFilename(userProvided, cmdPath string) string {
	if strings.TrimSpace(userProvided) != "" {
		name := strings.TrimSpace(userProvided)
		base := filepath.Base(name)
		dir := filepath.Dir(name)

		if !strings.HasSuffix(strings.ToLower(base), ".pdf") {
			base += ".pdf"
		}
		if dir != "." && dir != "" {
			return filepath.Join(dir, base)
		}
		return base
	}
	return DefaultPDFFilename(cmdPath)
}

// buildColSizes distributes the 12 grid units across columns proportionally
// to the length of the longest string in each column (header + data rows).
// This ensures each column is wide enough to fit its largest content.
func buildColSizes(headers []string, rows [][]string) []int {
	n := len(headers)
	if n == 0 {
		return nil
	}

	// Find the longest string in each column (including the header)
	maxLen := make([]int, n)
	for i, h := range headers {
		maxLen[i] = len(h)
	}
	for _, row := range rows {
		for i := 0; i < n && i < len(row); i++ {
			if l := len(row[i]); l > maxLen[i] {
				maxLen[i] = l
			}
		}
	}

	// Calculate total length for proportional distribution
	total := 0
	for _, l := range maxLen {
		total += l
	}
	if total == 0 {
		total = n
	}

	// Assign column sizes proportionally out of 12 units
	sizes := make([]int, n)
	sumAssigned := 0
	for i, l := range maxLen {
		proportion := float64(l) / float64(total)
		sizes[i] = int(math.Round(proportion * 12))
		if sizes[i] < 1 {
			sizes[i] = 1
		}
		sumAssigned += sizes[i]
	}

	// Fix rounding errors by distributing the difference
	diff := 12 - sumAssigned
	for i := 0; i < n && diff != 0; i++ {
		if diff > 0 {
			sizes[i]++
			diff--
		} else if sizes[i] > 1 {
			sizes[i]--
			diff++
		}
	}

	return sizes
}

// needsLandscape decides the page orientation for the PDF.
// - Tables with 3 or more columns → always landscape (Horizontal)
// - Tables with 1-2 columns → always portrait (Vertical)
func needsLandscape(headers []string, rows [][]string) bool {
	return len(headers) >= 3
}

// truncateForPDF currently does not truncate any values (pass-through).
func truncateForPDF(s string) string {
	return s
}

// darkBlue header color (HTML #0F1490)
var darkBlueHeader = &props.Color{Red: 15, Green: 20, Blue: 144}

// lightGray for alternating rows
var lightGrayRow = &props.Color{Red: 242, Green: 242, Blue: 245}

// tableBorderColor for visible grid lines around all cells (rows + columns)
var tableBorderColor = &props.BlackColor

func buildHeaderRow(headers []string, sizes []int) core.Row {
	n := len(headers)
	cols := make([]core.Col, n)

	headerCellStyle := &props.Cell{
		BackgroundColor: darkBlueHeader,
		BorderType:      border.Full,
		BorderThickness: 0.12,
		BorderColor:     tableBorderColor,
	}

	for i, h := range headers {
		cols[i] = col.New(sizes[i]).Add(
			text.New(strings.ToUpper(h), props.Text{
				Size:   7.5,
				Style:  fontstyle.Bold,
				Align:  align.Center,
				Color:  &props.WhiteColor,
				Top:    2.0,
				Bottom: 2.0,
				Left:   1.2,
				Right:  1.2,
				Family: fontfamily.Helvetica,
			}),
		).WithStyle(headerCellStyle)
	}

	// Auto-height header so long (non-truncated) headers can wrap inside the cell.
	r := row.New().Add(cols...)
	return r
}

func buildDataRow(sizes []int, values []string, alt bool) core.Row {
	n := len(sizes)
	cols := make([]core.Col, n)

	dataCellStyle := &props.Cell{
		BorderType:      border.Full,
		BorderThickness: 0.12,
		BorderColor:     tableBorderColor,
	}
	if alt {
		dataCellStyle.BackgroundColor = lightGrayRow
	}

	for i := 0; i < n; i++ {
		val := ""
		if i < len(values) {
			val = truncateForPDF(values[i])
		}
		cols[i] = col.New(sizes[i]).Add(
			text.New(val, props.Text{
				Size:   7.0,
				Align:  align.Left,
				Top:    1.5,
				Bottom: 1.5,
				Left:   1.2,
				Right:  1.2,
				Family: fontfamily.Helvetica,
				// Use default EmptySpaceStrategy (western text with spaces/dots/dashes in names/CIDRs)
			}),
		).WithStyle(dataCellStyle)
	}

	// Use auto height so long values (hostnames, OCIDs, etc) can wrap inside their column
	// instead of overflowing. Row height adapts to the tallest cell in the row.
	// Top+Bottom padding ensures text is vertically balanced inside each cell band.
	r := row.New().Add(cols...)
	return r
}
