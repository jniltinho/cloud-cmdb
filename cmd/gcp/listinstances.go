package gcp

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	gcpinternal "cloud-cmdb/internal/gcp"
	"github.com/spf13/cobra"
)

var (
	listInstancesTables   bool
	listInstancesText     bool
	listInstancesCSV      bool
	listInstancesPDF      bool
	listInstancesZone     string
	listInstancesColumns  string
	listInstancesContains string
	listInstancesStatus   string
)

// ListInstancesCmd is the cobra command for "cloud-cmdb gcp list-instances".
var ListInstancesCmd = &cobra.Command{
	Use:   "list-instances",
	Short: "List Compute Engine instances in the project",
	Long: `List Google Compute Engine (GCE) VM instances using the Cloud Compute API.

Authentication uses Application Default Credentials (ADC):
  1. GOOGLE_APPLICATION_CREDENTIALS env var (path to service account JSON)
  2. gcloud auth application-default login
  3. GCE metadata service (when running on GCP)

The project ID is read from --project flag or GOOGLE_CLOUD_PROJECT env var.

By default lists all non-terminated instances across all zones in the project
using the AggregatedList API (single call, very efficient).
Use --zone to restrict to a single zone.

Default columns:
  NAME, ZONE, MACHINE TYPE, STATUS, PRIVATE IP, PUBLIC IP

Use --columns to choose exactly which columns to show (comma-separated).

Available columns:
  name, zone, machine-type, status, private-ip, public-ip, project

Use --contains to filter by instance name (case-insensitive substring, comma-separated OR logic).
Use --status to filter by instance status (e.g. RUNNING, STOPPED, TERMINATED).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row
  --pdf      Generate PDF report (use --pdf-out for custom filename)`,
	Example: `  # List all instances in the project (uses GOOGLE_CLOUD_PROJECT)
  cloud-cmdb gcp list-instances

  # Explicit project ID
  cloud-cmdb gcp list-instances --project my-gcp-project

  # Scope to a specific zone (faster for large projects)
  cloud-cmdb gcp list-instances --zone us-central1-a

  # Custom columns
  cloud-cmdb gcp list-instances --columns="name,zone,machine-type,status,private-ip"

  # Filter by instance name
  cloud-cmdb gcp list-instances --contains=web-
  cloud-cmdb gcp list-instances --contains=web-,api-,worker-

  # Filter by status
  cloud-cmdb gcp list-instances --status=RUNNING
  cloud-cmdb gcp list-instances --status=RUNNING,STAGING

  # Export to CSV
  cloud-cmdb gcp list-instances --csv > gce-instances.csv

  # PDF report
  cloud-cmdb gcp list-instances --pdf --pdf-out gce-inventory.pdf

  # Plain text (great for scripts)
  cloud-cmdb gcp list-instances --text`,
	RunE: runListInstances,
}

func init() {
	ListInstancesCmd.Flags().BoolVar(&listInstancesTables, "tables", false, "Output as formatted table (default)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesText, "text", false, "Output as plain text (tab-separated)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListInstancesCmd.Flags().StringVar(&listInstancesZone, "zone", "", "GCP zone (optional - when omitted, lists across all zones using AggregatedList)")
	ListInstancesCmd.Flags().StringVar(&listInstancesColumns, "columns", "", "Columns to show (comma-separated). Default: name,zone,machine-type,status,private-ip,public-ip. Available: name,zone,machine-type,status,private-ip,public-ip,project")
	ListInstancesCmd.Flags().StringVar(&listInstancesContains, "contains", "", "Filter by instance name containing any of the given substrings (comma-separated, case-insensitive). Example: --contains=web-,api-")
	ListInstancesCmd.Flags().StringVar(&listInstancesStatus, "status", "", "Filter by instance status (comma-separated, case-insensitive). Example: --status=RUNNING or --status=RUNNING,STAGING")
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

	projectID, err := gcpinternal.GetProjectID(project)
	if err != nil {
		return err
	}

	columns, err := parseColumns(listInstancesColumns)
	if err != nil {
		return err
	}

	instances, err := gcpinternal.ListInstances(context.Background(), projectID, listInstancesZone)
	if err != nil {
		return err
	}

	// Filter by status
	if listInstancesStatus != "" {
		parts := strings.Split(listInstancesStatus, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.ToUpper(strings.TrimSpace(p)); t != "" {
				filters = append(filters, t)
			}
		}
		if len(filters) > 0 {
			filtered := make([]gcpinternal.InstanceSummary, 0, len(instances))
			for _, inst := range instances {
				status := strings.ToUpper(strings.TrimSpace(inst.Status))
				for _, f := range filters {
					if status == f {
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
			filtered := make([]gcpinternal.InstanceSummary, 0, len(instances))
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
		case listInstancesZone != "":
			fmt.Println("No instances found in the specified zone.")
		default:
			fmt.Println("No instances found in the project.")
		}
		return nil
	}

	if listInstancesPDF {
		headers := columnHeaders(columns)
		data := make([][]string, len(instances))
		for i, inst := range instances {
			data[i] = columnValues(inst, columns)
		}
		return generateAndSavePDF(cmd, "GCP Compute Engine Instances", headers, data)
	}

	return printInstanceList(instances, listInstancesCSV, listInstancesText, columns)
}

// --- column system ---

type instanceColumn int

const (
	colName instanceColumn = iota
	colZone
	colMachineType
	colStatus
	colPrivateIP
	colPublicIP
	colProject
)

var defaultColumns = []instanceColumn{
	colName,
	colZone,
	colMachineType,
	colStatus,
	colPrivateIP,
	colPublicIP,
}

func columnHeader(col instanceColumn) string {
	switch col {
	case colName:
		return "NAME"
	case colZone:
		return "ZONE"
	case colMachineType:
		return "MACHINE TYPE"
	case colStatus:
		return "STATUS"
	case colPrivateIP:
		return "PRIVATE IP"
	case colPublicIP:
		return "PUBLIC IP"
	case colProject:
		return "PROJECT"
	default:
		return ""
	}
}

func columnValue(inst gcpinternal.InstanceSummary, col instanceColumn) string {
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	switch col {
	case colName:
		return dash(inst.Name)
	case colZone:
		return dash(inst.Zone)
	case colMachineType:
		return dash(inst.MachineType)
	case colStatus:
		return dash(inst.Status)
	case colPrivateIP:
		return dash(inst.PrivateIP)
	case colPublicIP:
		return dash(inst.PublicIP)
	case colProject:
		return dash(inst.Project)
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

func columnValues(inst gcpinternal.InstanceSummary, cols []instanceColumn) []string {
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
			return nil, fmt.Errorf("unknown column %q — available: name, zone, machine-type, status, private-ip, public-ip, project", p)
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
	case "zone":
		return colZone, true
	case "machine-type", "machinetype", "type":
		return colMachineType, true
	case "status", "state":
		return colStatus, true
	case "private-ip", "privateip", "internal-ip":
		return colPrivateIP, true
	case "public-ip", "publicip", "external-ip", "nat-ip":
		return colPublicIP, true
	case "project":
		return colProject, true
	default:
		return 0, false
	}
}

// --- output renderers ---

func printInstanceList(instances []gcpinternal.InstanceSummary, asCSV, asText bool, columns []instanceColumn) error {
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
