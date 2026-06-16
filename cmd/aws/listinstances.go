package aws

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	awsinternal "cloud-cmdb/internal/aws"
	"github.com/spf13/cobra"
)

var (
	listInstancesTables   bool
	listInstancesText     bool
	listInstancesCSV      bool
	listInstancesPDF      bool
	listInstancesColumns  string
	listInstancesContains string
	listInstancesStatus   string
)

// ListInstancesCmd is the cobra command for "cloud-cmdb aws list-instances".
var ListInstancesCmd = &cobra.Command{
	Use:   "list-instances",
	Short: "List EC2 instances in the configured region",
	Long: `List EC2 compute instances using the AWS SDK v2.

Uses the standard AWS credential chain: environment variables, ~/.aws/credentials,
IAM instance profiles. Use --profile to select a named profile and --region to
override the target region.

By default (no filters) it returns all non-terminated instances.

Default columns:
  NAME, INSTANCE ID, TYPE, STATE, PRIVATE IP, PUBLIC IP, AZ

Use --columns to choose exactly which columns to show (comma-separated).

Available columns:
  name, instance-id, type, state, private-ip, public-ip, platform, az, vpc, subnet, launched, region

Use --contains to filter by name tag (case-insensitive substring, comma-separated OR logic).
Use --status to filter by instance state (e.g. running, stopped).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row
  --pdf      Generate PDF report (use --pdf-out for custom filename)`,
	Example: `  # List all instances in the default region
  cloud-cmdb aws list-instances

  # Specific region
  cloud-cmdb aws list-instances --region us-east-1

  # Use a named AWS profile
  cloud-cmdb aws list-instances --profile production --region eu-west-1

  # Custom columns
  cloud-cmdb aws list-instances --columns="name,instance-id,type,state,private-ip"

  # Filter by name tag
  cloud-cmdb aws list-instances --contains=web-
  cloud-cmdb aws list-instances --contains=web-,api-,db-

  # Filter by state
  cloud-cmdb aws list-instances --status=running
  cloud-cmdb aws list-instances --status=running,stopped

  # Export to CSV
  cloud-cmdb aws list-instances --csv > ec2.csv

  # PDF report
  cloud-cmdb aws list-instances --pdf --pdf-out ec2-inventory.pdf

  # Plain text
  cloud-cmdb aws list-instances --text`,
	RunE: runListInstances,
}

func init() {
	ListInstancesCmd.Flags().BoolVar(&listInstancesTables, "tables", false, "Output as formatted table (default)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesText, "text", false, "Output as plain text (tab-separated)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListInstancesCmd.Flags().StringVar(&listInstancesColumns, "columns", "", "Columns to show (comma-separated). Default: name,instance-id,type,state,private-ip,public-ip,az. Available: name,instance-id,type,state,private-ip,public-ip,platform,az,vpc,subnet,launched,region")
	ListInstancesCmd.Flags().StringVar(&listInstancesContains, "contains", "", "Filter by Name tag containing any of the given substrings (comma-separated, case-insensitive). Example: --contains=web-,api-")
	ListInstancesCmd.Flags().StringVar(&listInstancesStatus, "status", "", "Filter by instance state (comma-separated, case-insensitive). Example: --status=running or --status=running,stopped")
}

func runListInstances(cmd *cobra.Command, args []string) error {
	if listInstancesTables && listInstancesText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listInstancesCSV && (listInstancesTables || listInstancesText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listInstancesPDF && (listInstancesCSV || listInstancesText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	columns, err := parseColumns(listInstancesColumns)
	if err != nil {
		return err
	}

	instances, err := awsinternal.ListInstances(context.Background(), region, profile)
	if err != nil {
		return err
	}

	// Filter by state
	if listInstancesStatus != "" {
		parts := strings.Split(listInstancesStatus, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.ToLower(strings.TrimSpace(p))
			if trimmed != "" {
				filters = append(filters, trimmed)
			}
		}
		if len(filters) > 0 {
			filtered := make([]awsinternal.InstanceSummary, 0, len(instances))
			for _, inst := range instances {
				instState := strings.ToLower(strings.TrimSpace(inst.State))
				for _, f := range filters {
					if instState == f {
						filtered = append(filtered, inst)
						break
					}
				}
			}
			instances = filtered
		}
	}

	// Filter by Name tag
	if listInstancesContains != "" {
		parts := strings.Split(listInstancesContains, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			trimmed := strings.ToLower(strings.TrimSpace(p))
			if trimmed != "" {
				filters = append(filters, trimmed)
			}
		}
		if len(filters) > 0 {
			filtered := make([]awsinternal.InstanceSummary, 0, len(instances))
			for _, inst := range instances {
				nameLower := strings.ToLower(inst.Name)
				for _, f := range filters {
					if strings.Contains(nameLower, f) {
						filtered = append(filtered, inst)
						break
					}
				}
			}
			instances = filtered
		}
	}

	if len(instances) == 0 {
		switch {
		case listInstancesStatus != "" && listInstancesContains != "":
			fmt.Printf("No instances found matching state '%s' and name filter '%s'.\n", listInstancesStatus, listInstancesContains)
		case listInstancesStatus != "":
			fmt.Printf("No instances found matching state '%s'.\n", listInstancesStatus)
		case listInstancesContains != "":
			fmt.Printf("No instances found matching name filter '%s'.\n", listInstancesContains)
		default:
			fmt.Println("No instances found in the region.")
		}
		return nil
	}

	if listInstancesPDF {
		headers := columnHeaders(columns)
		data := make([][]string, len(instances))
		for i, inst := range instances {
			data[i] = columnValues(inst, columns)
		}
		return generateAndSavePDF(cmd, "AWS EC2 Instances", headers, data)
	}

	return printInstanceList(instances, listInstancesCSV, listInstancesText, columns)
}

// --- column system ---

type instanceColumn int

const (
	colName instanceColumn = iota
	colInstanceID
	colType
	colState
	colPrivateIP
	colPublicIP
	colPlatform
	colAZ
	colVPC
	colSubnet
	colLaunched
	colRegion
)

var defaultColumns = []instanceColumn{
	colName,
	colInstanceID,
	colType,
	colState,
	colPrivateIP,
	colPublicIP,
	colAZ,
}

func columnHeader(col instanceColumn) string {
	switch col {
	case colName:
		return "NAME"
	case colInstanceID:
		return "INSTANCE ID"
	case colType:
		return "TYPE"
	case colState:
		return "STATE"
	case colPrivateIP:
		return "PRIVATE IP"
	case colPublicIP:
		return "PUBLIC IP"
	case colPlatform:
		return "PLATFORM"
	case colAZ:
		return "AZ"
	case colVPC:
		return "VPC"
	case colSubnet:
		return "SUBNET"
	case colLaunched:
		return "LAUNCHED"
	case colRegion:
		return "REGION"
	default:
		return ""
	}
}

func columnValue(inst awsinternal.InstanceSummary, col instanceColumn) string {
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	switch col {
	case colName:
		return dash(inst.Name)
	case colInstanceID:
		return dash(inst.InstanceID)
	case colType:
		return dash(inst.Type)
	case colState:
		return dash(inst.State)
	case colPrivateIP:
		return dash(inst.PrivateIP)
	case colPublicIP:
		return dash(inst.PublicIP)
	case colPlatform:
		return dash(inst.Platform)
	case colAZ:
		return dash(inst.AZ)
	case colVPC:
		return dash(inst.VPC)
	case colSubnet:
		return dash(inst.Subnet)
	case colLaunched:
		return dash(inst.LaunchTime)
	case colRegion:
		return dash(inst.Region)
	default:
		return ""
	}
}

func columnHeaders(cols []instanceColumn) []string {
	headers := make([]string, len(cols))
	for i, col := range cols {
		headers[i] = columnHeader(col)
	}
	return headers
}

func columnValues(inst awsinternal.InstanceSummary, cols []instanceColumn) []string {
	row := make([]string, len(cols))
	for i, col := range cols {
		row[i] = columnValue(inst, col)
	}
	return row
}

func parseColumns(cols string) ([]instanceColumn, error) {
	if cols == "" {
		out := make([]instanceColumn, len(defaultColumns))
		copy(out, defaultColumns)
		return out, nil
	}

	parts := strings.Split(cols, ",")
	columns := make([]instanceColumn, 0, len(parts))
	seen := make(map[instanceColumn]bool, len(parts))

	for _, p := range parts {
		trimmed := strings.ToLower(strings.TrimSpace(p))
		if trimmed == "" {
			continue
		}
		col, ok := parseColumnName(trimmed)
		if !ok {
			return nil, fmt.Errorf("unknown column %q — available: name, instance-id, type, state, private-ip, public-ip, platform, az, vpc, subnet, launched, region", p)
		}
		if seen[col] {
			continue
		}
		seen[col] = true
		columns = append(columns, col)
	}

	if len(columns) == 0 {
		return nil, fmt.Errorf("no valid columns specified in --columns")
	}
	return columns, nil
}

func parseColumnName(name string) (instanceColumn, bool) {
	switch name {
	case "name":
		return colName, true
	case "instance-id", "instanceid", "id":
		return colInstanceID, true
	case "type", "instance-type":
		return colType, true
	case "state", "status":
		return colState, true
	case "private-ip", "private_ip", "privateip":
		return colPrivateIP, true
	case "public-ip", "public_ip", "publicip":
		return colPublicIP, true
	case "platform", "os":
		return colPlatform, true
	case "az", "availability-zone":
		return colAZ, true
	case "vpc", "vpc-id":
		return colVPC, true
	case "subnet", "subnet-id":
		return colSubnet, true
	case "launched", "launch-time":
		return colLaunched, true
	case "region":
		return colRegion, true
	default:
		return 0, false
	}
}

// --- output renderers ---

func printInstanceList(instances []awsinternal.InstanceSummary, asCSV, asText bool, columns []instanceColumn) error {
	headers := columnHeaders(columns)

	if asCSV {
		w := csv.NewWriter(os.Stdout)
		if err := w.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}
		for _, inst := range instances {
			if err := w.Write(columnValues(inst, columns)); err != nil {
				return fmt.Errorf("failed to write CSV row: %w", err)
			}
		}
		w.Flush()
		return w.Error()
	}

	if asText {
		for _, inst := range instances {
			fmt.Println(strings.Join(columnValues(inst, columns), "\t"))
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	header := make(table.Row, len(headers))
	for i, h := range headers {
		header[i] = h
	}
	t.AppendHeader(header)

	for _, inst := range instances {
		values := columnValues(inst, columns)
		row := make(table.Row, len(values))
		for i, v := range values {
			row[i] = v
		}
		t.AppendRow(row)
	}

	footer := make(table.Row, len(headers))
	for i := range footer {
		footer[i] = ""
	}
	footer[len(footer)-1] = fmt.Sprintf("TOTAL: %d", len(instances))
	t.AppendFooter(footer)

	t.Render()
	return nil
}
