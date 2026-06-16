package oci

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"os"
	"time"

	"github.com/jedib0t/go-pretty/v6/table"
	ociapi "cloud-cmdb/internal/oci"
	"github.com/oracle/oci-go-sdk/v65/core"
	"github.com/spf13/cobra"
)

var (
	subnetID string
	delayMs  int

	privateIpsTables bool
	privateIpsText   bool
	privateIpsCSV    bool
	privateIpsPDF    bool

	// Legacy --output support (for backward compatibility)
	output string

	privateIpsSum bool
)

// ListPrivateIPsCmd is the cobra command for "cloud-cmdb oci list-private-ips".
var ListPrivateIPsCmd = &cobra.Command{
	Use:   "list-private-ips",
	Short: "List Private IPs in a subnet (IP, name, primary, hostname, VNIC)",
	Long: `List all Private IP addresses in a given Subnet using the OCI SDK.

Supports full pagination and a configurable delay between pages to avoid
rate limiting (429 TooManyRequests) on large subnets.

Output formats:
  --tables   Pretty table (default)
  --text     One IP per line (ideal for scripts, xargs, while read)
  --csv      CSV format with header row (IP, NAME, PRIMARY, HOSTNAME, VNIC)
  --pdf      PDF file (full list or --sum summary)
  --sum      Compact summary (RANGE + ALLOCATED). Combines with --tables/--text/--csv/--pdf.

Use --output for advanced formats (json, json-simple) — deprecated in favor of
standard flags for consistency with other commands.`,
	Example: `  # Default: nice table with IP, Name, Primary, Hostname, VNIC
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1...

  # One IP per line (perfect for scripts / pipelines)
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --text

  # Export to CSV
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --csv > private-ips.csv

  # Higher delay for very large subnets
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --delay 750

  # Use a non-default OCI profile
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --profile PROD

  # Summary: CIDR range + total allocated (table by default)
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --sum

  # Summary in CSV (great for monitoring/scripts)
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --sum --csv

  # Summary as tab-separated (one line: CIDR<TAB>COUNT)
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --sum --text

  # PDF report of IPs or summary
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --pdf
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --sum --pdf

  # Custom PDF output filename
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --pdf --pdf-out ips-subnet.pdf

  # Advanced: full JSON (all SDK fields)
  cloud-cmdb oci list-private-ips --subnet-id ocid1.subnet.oc1... --output json`,
	RunE: runListPrivateIPs,
}

func init() {
	ListPrivateIPsCmd.Flags().BoolVar(&privateIpsTables, "tables", false, "Output as formatted table (default)")
	ListPrivateIPsCmd.Flags().BoolVar(&privateIpsText, "text", false, "Output as plain text (one IP per line)")
	ListPrivateIPsCmd.Flags().BoolVar(&privateIpsCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListPrivateIPsCmd.Flags().BoolVar(&privateIpsPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListPrivateIPsCmd.Flags().StringVar(&subnetID, "subnet-id", "", "Subnet OCID (required)")
	ListPrivateIPsCmd.Flags().IntVar(&delayMs, "delay", 250, "Delay in ms between pagination pages (avoid 429 rate limits)")
	ListPrivateIPsCmd.Flags().StringVar(&output, "output", "", "Deprecated: use --tables/--text/--csv. Still supports: json, json-simple, list")
	ListPrivateIPsCmd.Flags().BoolVar(&privateIpsSum, "sum", false, "Summary only (RANGE + ALLOCATED). Use with --tables/--text/--csv for output format")

	_ = ListPrivateIPsCmd.MarkFlagRequired("subnet-id")
}

func runListPrivateIPs(cmd *cobra.Command, args []string) error {
	if privateIpsTables && privateIpsText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if privateIpsCSV && (privateIpsTables || privateIpsText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if privateIpsPDF && (privateIpsCSV || privateIpsText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	if privateIpsSum {
		return runListPrivateIPsSum(cmd)
	}

	if output != "" {
		if privateIpsPDF {
			return fmt.Errorf("cannot use --pdf together with legacy --output")
		}
		return runPrivateIPsLegacy()
	}

	delay := time.Duration(delayMs) * time.Millisecond

	opts := ociapi.ListPrivateIPsOptions{
		SubnetID:   subnetID,
		Delay:      delay,
		ConfigFile: configFile,
		Profile:    profile,
	}

	ips, err := ociapi.ListPrivateIPs(context.Background(), opts)
	if err != nil {
		if delayMs < 500 {
			return fmt.Errorf("%w\n\nTip: for large subnets, try --delay 500 or --delay 1000", err)
		}
		return err
	}

	if len(ips) == 0 {
		fmt.Println("No private IPs found in the subnet.")
		return nil
	}

	if privateIpsPDF {
		headers := []string{"IP ADDRESS", "NAME", "PRIMARY", "HOSTNAME", "VNIC ID"}
		data := make([][]string, len(ips))
		for i, p := range ips {
			data[i] = []string{
				valOrEmpty(p.IpAddress),
				valOrEmpty(p.DisplayName),
				formatPrimaryCSV(p.IsPrimary),
				valOrEmpty(p.HostnameLabel),
				valOrEmpty(p.VnicId),
			}
		}
		return generateAndSavePDF(cmd, "OCI Private IPs", headers, data)
	}

	if privateIpsCSV {
		return printPrivateIPsCSV(ips)
	}
	if privateIpsText {
		return printPrivateIPsText(ips)
	}
	return printPrivateIPsTable(ips)
}

func runPrivateIPsLegacy() error {
	validOutputs := map[string]bool{
		"text": true, "json": true, "json-simple": true,
		"list": true, "plain": true, "ips": true,
	}
	if !validOutputs[output] {
		return fmt.Errorf("invalid --output value %q (valid: text, json, json-simple, list) — consider migrating to --tables/--text/--csv", output)
	}

	delay := time.Duration(delayMs) * time.Millisecond

	opts := ociapi.ListPrivateIPsOptions{
		SubnetID:   subnetID,
		Delay:      delay,
		ConfigFile: configFile,
		Profile:    profile,
	}

	ips, err := ociapi.ListPrivateIPs(context.Background(), opts)
	if err != nil {
		if delayMs < 500 {
			return fmt.Errorf("%w\n\nTip: for large subnets, try --delay 500 or --delay 1000", err)
		}
		return err
	}

	return printOutputLegacy(ips)
}

func runListPrivateIPsSum(cmd *cobra.Command) error {
	delay := time.Duration(delayMs) * time.Millisecond

	opts := ociapi.ListPrivateIPsOptions{
		SubnetID:   subnetID,
		Delay:      delay,
		ConfigFile: configFile,
		Profile:    profile,
	}

	ips, err := ociapi.ListPrivateIPs(context.Background(), opts)
	if err != nil {
		if delayMs < 500 {
			return fmt.Errorf("%w\n\nTip: for large subnets, try --delay 500 or --delay 1000", err)
		}
		return err
	}

	sum, err := ociapi.GetSubnetSummary(context.Background(), subnetID, configFile, profile)
	if err != nil {
		return fmt.Errorf("failed to get subnet CIDR for --sum: %w", err)
	}

	cidr := sum.CIDR
	if cidr == "" {
		cidr = "-"
	}
	count := len(ips)

	if privateIpsPDF {
		headers := []string{"RANGE", "ALLOCATED"}
		data := [][]string{{cidr, fmt.Sprintf("%d", count)}}
		return generateAndSavePDF(cmd, "OCI Private IPs Summary", headers, data)
	}

	if privateIpsCSV {
		return printPrivateIPsSumCSV(cidr, count)
	}
	if privateIpsText {
		return printPrivateIPsSumText(cidr, count)
	}
	return printPrivateIPsSumTable(cidr, count)
}

func printPrivateIPsTable(ips []core.PrivateIp) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"IP ADDRESS", "NAME", "PRIMARY", "HOSTNAME", "VNIC"})

	for _, p := range ips {
		ip := valOrDash(p.IpAddress)
		name := valOrDash(p.DisplayName)
		primary := formatPrimary(p.IsPrimary)
		hostname := valOrDash(p.HostnameLabel)
		vnic := abbrevOCID(valOrEmpty(p.VnicId))

		t.AppendRow(table.Row{ip, name, primary, hostname, vnic})
	}

	t.AppendFooter(table.Row{"", "", "", "", fmt.Sprintf("TOTAL: %d", len(ips))})
	t.Render()
	return nil
}

func printPrivateIPsText(ips []core.PrivateIp) error {
	for _, p := range ips {
		if p.IpAddress != nil {
			fmt.Println(*p.IpAddress)
		}
	}
	return nil
}

func printPrivateIPsCSV(ips []core.PrivateIp) error {
	w := csv.NewWriter(os.Stdout)

	header := []string{"IP ADDRESS", "NAME", "PRIMARY", "HOSTNAME", "VNIC ID"}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	for _, p := range ips {
		ip := valOrEmpty(p.IpAddress)
		name := valOrEmpty(p.DisplayName)
		primary := formatPrimaryCSV(p.IsPrimary)
		hostname := valOrEmpty(p.HostnameLabel)
		vnic := valOrEmpty(p.VnicId)

		row := []string{ip, name, primary, hostname, vnic}
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

func printPrivateIPsSumTable(cidr string, count int) error {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"RANGE", "ALLOCATED"})
	t.AppendRow(table.Row{cidr, fmt.Sprintf("%d", count)})
	t.Render()
	return nil
}

func printPrivateIPsSumText(cidr string, count int) error {
	fmt.Printf("%s\t%d\n", cidr, count)
	return nil
}

func printPrivateIPsSumCSV(cidr string, count int) error {
	w := csv.NewWriter(os.Stdout)

	header := []string{"RANGE", "ALLOCATED"}
	if err := w.Write(header); err != nil {
		return fmt.Errorf("failed to write CSV header: %w", err)
	}

	row := []string{cidr, fmt.Sprintf("%d", count)}
	if err := w.Write(row); err != nil {
		return fmt.Errorf("failed to write CSV row: %w", err)
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return fmt.Errorf("failed to flush CSV: %w", err)
	}
	return nil
}

func printOutputLegacy(ips []core.PrivateIp) error {
	switch output {
	case "json":
		payload := struct {
			Count      int              `json:"count"`
			PrivateIps []core.PrivateIp `json:"private_ips"`
			SubnetId   string           `json:"subnet_id,omitempty"`
		}{
			Count:      len(ips),
			PrivateIps: ips,
			SubnetId:   subnetID,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)

	case "json-simple":
		simple := make([]string, 0, len(ips))
		for _, p := range ips {
			if p.IpAddress != nil {
				simple = append(simple, *p.IpAddress)
			}
		}
		payload := struct {
			Count      int      `json:"count"`
			PrivateIps []string `json:"private_ips"`
		}{
			Count:      len(simple),
			PrivateIps: simple,
		}
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		return enc.Encode(payload)

	case "list", "plain", "ips":
		for _, p := range ips {
			if p.IpAddress != nil {
				fmt.Println(*p.IpAddress)
			}
		}
		return nil

	default:
		fmt.Printf("private_ips_found: %d|", len(ips))
		if subnetID != "" {
			fmt.Printf("subnet_id: %s", subnetID)
		}
		fmt.Println()
		return nil
	}
}

// --- helpers ---

func valOrDash(p *string) string {
	if p == nil || *p == "" {
		return "-"
	}
	return *p
}

func valOrEmpty(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func formatPrimary(b *bool) string {
	if b == nil {
		return "-"
	}
	if *b {
		return "yes"
	}
	return "no"
}

func formatPrimaryCSV(b *bool) string {
	if b == nil {
		return ""
	}
	if *b {
		return "true"
	}
	return "false"
}

func abbrevOCID(s string) string {
	if s == "" {
		return "-"
	}
	if len(s) <= 30 {
		return s
	}
	parts := []rune(s)
	if len(parts) > 28 {
		return string(parts[:20]) + "..." + string(parts[len(parts)-8:])
	}
	return s
}
