package alibaba

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	alibabainternal "cloud-cmdb/internal/alibaba"
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

// ListInstancesCmd is the cobra command for "cloud-cmdb alibaba list-instances".
var ListInstancesCmd = &cobra.Command{
	Use:   "list-instances",
	Short: "List ECS instances in the configured region",
	Long: `List Alibaba Cloud ECS (Elastic Compute Service) instances using the alibabacloud-go SDK.

Authentication uses Access Key credentials:
  1. --access-key-id / --access-key-secret flags
  2. ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET environment variables

The region is read from --region flag or ALIBABA_CLOUD_REGION env var (default: cn-hangzhou).

By default returns all non-deleted instances in the region.

Default columns:
  NAME, INSTANCE ID, TYPE, STATUS, PRIVATE IP, PUBLIC IP, OS, ZONE

Use --columns to choose exactly which columns to show (comma-separated).

Available columns:
  name, instance-id, type, status, private-ip, public-ip, os, zone, region

Use --contains to filter by instance name (case-insensitive substring, comma-separated OR logic).
Use --status to filter by instance status (e.g. Running, Stopped).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row
  --pdf      Generate PDF report (use --pdf-out for custom filename)`,
	Example: `  # List all instances in the default region (cn-hangzhou)
  cloud-cmdb alibaba list-instances

  # Specific region
  cloud-cmdb alibaba list-instances --region ap-southeast-1

  # Use explicit credentials
  cloud-cmdb alibaba list-instances --access-key-id LTAI5t... --access-key-secret xxx

  # Custom columns
  cloud-cmdb alibaba list-instances --columns="name,instance-id,type,status,private-ip"

  # Filter by name
  cloud-cmdb alibaba list-instances --contains=web-,api-

  # Filter by status
  cloud-cmdb alibaba list-instances --status=Running
  cloud-cmdb alibaba list-instances --status=Running,Stopped

  # Export to CSV
  cloud-cmdb alibaba list-instances --csv > ecs-instances.csv

  # PDF report
  cloud-cmdb alibaba list-instances --pdf --pdf-out alibaba-inventory.pdf

  # Plain text
  cloud-cmdb alibaba list-instances --text`,
	RunE: runListInstances,
}

func init() {
	ListInstancesCmd.Flags().BoolVar(&listInstancesTables, "tables", false, "Output as formatted table (default)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesText, "text", false, "Output as plain text (tab-separated)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListInstancesCmd.Flags().StringVar(&listInstancesColumns, "columns", "", "Columns to show (comma-separated). Default: name,instance-id,type,status,private-ip,public-ip,os,zone. Available: name,instance-id,type,status,private-ip,public-ip,os,zone,region")
	ListInstancesCmd.Flags().StringVar(&listInstancesContains, "contains", "", "Filter by name containing any of the given substrings (comma-separated, case-insensitive). Example: --contains=web-,api-")
	ListInstancesCmd.Flags().StringVar(&listInstancesStatus, "status", "", "Filter by instance status (comma-separated, case-insensitive). Example: --status=Running or --status=Running,Stopped")
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

	resolvedRegion := alibabainternal.GetRegion(region)

	instances, err := alibabainternal.ListInstances(context.Background(), resolvedRegion, accessKeyID, accessKeySecret)
	if err != nil {
		return err
	}

	// Filter by status
	if listInstancesStatus != "" {
		parts := strings.Split(listInstancesStatus, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.ToLower(strings.TrimSpace(p)); t != "" {
				filters = append(filters, t)
			}
		}
		if len(filters) > 0 {
			filtered := make([]alibabainternal.InstanceSummary, 0, len(instances))
			for _, inst := range instances {
				s := strings.ToLower(strings.TrimSpace(inst.Status))
				for _, f := range filters {
					if s == f {
						filtered = append(filtered, inst)
						break
					}
				}
			}
			instances = filtered
		}
	}

	// Filter by name
	if listInstancesContains != "" {
		parts := strings.Split(listInstancesContains, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.ToLower(strings.TrimSpace(p)); t != "" {
				filters = append(filters, t)
			}
		}
		if len(filters) > 0 {
			filtered := make([]alibabainternal.InstanceSummary, 0, len(instances))
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
			fmt.Printf("No instances found matching status '%s' and name filter '%s'.\n", listInstancesStatus, listInstancesContains)
		case listInstancesStatus != "":
			fmt.Printf("No instances found matching status '%s'.\n", listInstancesStatus)
		case listInstancesContains != "":
			fmt.Printf("No instances found matching name filter '%s'.\n", listInstancesContains)
		default:
			fmt.Printf("No instances found in region '%s'.\n", resolvedRegion)
		}
		return nil
	}

	if listInstancesPDF {
		headers := columnHeaders(columns)
		data := make([][]string, len(instances))
		for i, inst := range instances {
			data[i] = columnValues(inst, columns)
		}
		return generateAndSavePDF(cmd, "Alibaba Cloud ECS Instances", headers, data)
	}

	return printInstanceList(instances, listInstancesCSV, listInstancesText, columns)
}

// --- column system ---

type instanceColumn int

const (
	colName instanceColumn = iota
	colInstanceID
	colType
	colStatus
	colPrivateIP
	colPublicIP
	colOS
	colZone
	colRegion
)

var defaultColumns = []instanceColumn{
	colName,
	colInstanceID,
	colType,
	colStatus,
	colPrivateIP,
	colPublicIP,
	colOS,
	colZone,
}

func columnHeader(col instanceColumn) string {
	switch col {
	case colName:
		return "NAME"
	case colInstanceID:
		return "INSTANCE ID"
	case colType:
		return "TYPE"
	case colStatus:
		return "STATUS"
	case colPrivateIP:
		return "PRIVATE IP"
	case colPublicIP:
		return "PUBLIC IP"
	case colOS:
		return "OS"
	case colZone:
		return "ZONE"
	case colRegion:
		return "REGION"
	default:
		return ""
	}
}

func columnValue(inst alibabainternal.InstanceSummary, col instanceColumn) string {
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
	case colStatus:
		return dash(inst.Status)
	case colPrivateIP:
		return dash(inst.PrivateIP)
	case colPublicIP:
		return dash(inst.PublicIP)
	case colOS:
		return dash(inst.OS)
	case colZone:
		return dash(inst.Zone)
	case colRegion:
		return dash(inst.Region)
	default:
		return ""
	}
}

func columnHeaders(cols []instanceColumn) []string {
	h := make([]string, len(cols))
	for i, col := range cols {
		h[i] = columnHeader(col)
	}
	return h
}

func columnValues(inst alibabainternal.InstanceSummary, cols []instanceColumn) []string {
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
			return nil, fmt.Errorf("unknown column %q — available: name, instance-id, type, status, private-ip, public-ip, os, zone, region", p)
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
	case "status", "state":
		return colStatus, true
	case "private-ip", "privateip", "private_ip":
		return colPrivateIP, true
	case "public-ip", "publicip", "public_ip":
		return colPublicIP, true
	case "os", "osname":
		return colOS, true
	case "zone", "zone-id":
		return colZone, true
	case "region", "region-id":
		return colRegion, true
	default:
		return 0, false
	}
}

// --- output renderers ---

func printInstanceList(instances []alibabainternal.InstanceSummary, asCSV, asText bool, columns []instanceColumn) error {
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
