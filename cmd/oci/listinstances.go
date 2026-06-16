package oci

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	ociinternal "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listInstancesTables      bool
	listInstancesText        bool
	listInstancesCSV         bool
	listInstancesPDF         bool
	listInstancesCompartment string
	listInstancesColumns     string
	listInstancesContains    string
	listInstancesStatus      string
)

// ListInstancesCmd is the cobra command for "cloud-cmdb oci list-instances".
var ListInstancesCmd = &cobra.Command{
	Use:   "list-instances",
	Short: "List instances across the tenancy or in a compartment (unified rich inventory)",
	Long: `List compute instances in the tenancy (unified view combining old list-instances + list-servers).

By default (no --compartment flag) it enumerates ALL compartments and returns
every non-terminated instance with full server inventory details.

	Default columns:
  NAME, OS, RAM (GB), OCPU, PRIVATE IP, STATUS, LAUNCHED, COMPARTMENT

Use --columns to choose exactly which columns to show (comma-separated).
When --columns is set, only the listed columns are displayed (in the given order).

Available columns:
  name, os, ram, ocpu, shape, vcn, private-ip, public-ip, status,
  capacity, launched, compartment, instance-id

Example: --columns="name,private-ip,status,compartment"
Example: --columns="name,shape,vcn,public-ip,capacity"

Use --compartment to restrict to a single compartment (name or OCID supported).

Use --contains to filter instances by name (case-insensitive substring match).
  Accepts multiple values separated by comma (OR logic).
  Examples:
    --contains=oke-
    --contains=app01
    --contains=app01,app03,oke-
    --contains="web-prod,db" --compartment Production

Use --status to filter by lifecycle state (case-insensitive, comma-separated OR logic).
  Examples:
    --status=RUNNING
    --status=STOPPED
    --status=RUNNING,STOPPED

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row (great for Excel / Google Sheets)
  --pdf      Generate PDF report (use --pdf-out for custom filename)

Use --columns to pick columns (e.g. --columns="name,shape,vcn,public-ip").
Use --contains to filter by name substrings (comma-separated).
Use --status to filter by lifecycle state (e.g. --status=RUNNING).

This command replaces the old "list-servers" functionality.`,
	Example: `  # List every instance across the entire tenancy (default columns)
  cloud-cmdb oci list-instances

  # Choose specific columns only
  cloud-cmdb oci list-instances --columns="name,private-ip,status,compartment"

  # Rich view with optional columns
  cloud-cmdb oci list-instances --columns="name,os,ram,ocpu,shape,vcn,private-ip,public-ip,status,capacity,launched,compartment"

  # With PDF
  cloud-cmdb oci list-instances --pdf --pdf-out inventario.pdf

  # Restrict to one compartment + custom columns
  cloud-cmdb oci list-instances --compartment Production --columns="name,shape,vcn,public-ip"

  # Filter by name (contains) - supports comma-separated list
  cloud-cmdb oci list-instances --contains=oke-
  cloud-cmdb oci list-instances --contains=app01
  cloud-cmdb oci list-instances --contains=app01,app03,oke-
  cloud-cmdb oci list-instances --contains="web-prod,db" --compartment Production

  # Filter by status
  cloud-cmdb oci list-instances --status=RUNNING
  cloud-cmdb oci list-instances --status=STOPPED
  cloud-cmdb oci list-instances --status=RUNNING,STOPPED --compartment Production

  # Export to CSV
  cloud-cmdb oci list-instances --csv > servers.csv

  # Plain text
  cloud-cmdb oci list-instances --text`,
	RunE: runListInstances,
}

func init() {
	ListInstancesCmd.Flags().BoolVar(&listInstancesTables, "tables", false, "Output as formatted table (default)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesText, "text", false, "Output as plain text (tab-separated)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListInstancesCmd.Flags().StringVar(&listInstancesCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
	ListInstancesCmd.Flags().StringVar(&listInstancesColumns, "columns", "", "Columns to show (comma-separated). Default: name,os,ram,ocpu,private-ip,status,launched,compartment. Available: name,os,ram,ocpu,shape,vcn,private-ip,public-ip,status,capacity,launched,compartment,instance-id")
	ListInstancesCmd.Flags().StringVar(&listInstancesContains, "contains", "", "Filter by name containing any of the given substrings (comma-separated, case-insensitive). Example: --contains=app01,app03,oke-")
	ListInstancesCmd.Flags().StringVar(&listInstancesStatus, "status", "", "Filter by lifecycle state (comma-separated, case-insensitive, OR logic). Example: --status=RUNNING or --status=RUNNING,STOPPED")
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

	columns, err := parseInstanceColumns(listInstancesColumns)
	if err != nil {
		return err
	}

	var statusFilters []string
	if listInstancesStatus != "" {
		statusFilters, err = parseInstanceStatusFilters(listInstancesStatus)
		if err != nil {
			return err
		}
	}

	instances, err := ociinternal.ListInstances(context.Background(), listInstancesCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(statusFilters) > 0 {
		filtered := make([]ociinternal.InstanceSummary, 0, len(instances))
		for _, inst := range instances {
			instStatus := strings.ToUpper(strings.TrimSpace(inst.Status))
			for _, f := range statusFilters {
				if instStatus == f {
					filtered = append(filtered, inst)
					break
				}
			}
		}
		instances = filtered
	}

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
			filtered := make([]ociinternal.InstanceSummary, 0, len(instances))
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
			fmt.Printf("No instances found matching status filter '%s' and name filter '%s'.\n", listInstancesStatus, listInstancesContains)
		case listInstancesStatus != "":
			fmt.Printf("No instances found matching status filter '%s'.\n", listInstancesStatus)
		case listInstancesContains != "":
			fmt.Printf("No instances found matching name filter '%s'.\n", listInstancesContains)
		case listInstancesCompartment != "":
			fmt.Println("No instances found in the specified compartment.")
		default:
			fmt.Println("No instances found in the tenancy.")
		}
		return nil
	}

	if listInstancesPDF {
		headers := instanceColumnHeaders(columns)
		data := make([][]string, len(instances))
		for i, inst := range instances {
			data[i] = instanceColumnValues(inst, columns)
		}
		return generateAndSavePDF(cmd, "OCI Instances", headers, data)
	}

	return printInstanceList(instances, listInstancesCSV, listInstancesText, columns)
}

type instanceColumn int

const (
	instanceColName instanceColumn = iota
	instanceColOS
	instanceColRAM
	instanceColOCPU
	instanceColShape
	instanceColVCN
	instanceColPrivateIP
	instanceColPublicIP
	instanceColStatus
	instanceColCapacity
	instanceColLaunched
	instanceColCompartment
	instanceColInstanceID
)

var defaultInstanceColumns = []instanceColumn{
	instanceColName,
	instanceColOS,
	instanceColRAM,
	instanceColOCPU,
	instanceColPrivateIP,
	instanceColStatus,
	instanceColLaunched,
	instanceColCompartment,
}

func instanceColumnHeader(col instanceColumn) string {
	switch col {
	case instanceColName:
		return "NAME"
	case instanceColOS:
		return "OS"
	case instanceColRAM:
		return "RAM (GB)"
	case instanceColOCPU:
		return "OCPU"
	case instanceColShape:
		return "SHAPE"
	case instanceColVCN:
		return "VCN"
	case instanceColPrivateIP:
		return "PRIVATE IP"
	case instanceColPublicIP:
		return "PUBLIC IP"
	case instanceColStatus:
		return "STATUS"
	case instanceColCapacity:
		return "CAPACITY TYPE"
	case instanceColLaunched:
		return "LAUNCHED"
	case instanceColCompartment:
		return "COMPARTMENT"
	case instanceColInstanceID:
		return "INSTANCE ID"
	default:
		return ""
	}
}

func instanceColumnValue(inst ociinternal.InstanceSummary, col instanceColumn) string {
	switch col {
	case instanceColName:
		return inst.Name
	case instanceColOS:
		return dashIfEmpty(inst.OS)
	case instanceColRAM:
		return dashIfEmpty(inst.RAMGB)
	case instanceColOCPU:
		return dashIfEmpty(inst.OCPU)
	case instanceColShape:
		return dashIfEmpty(inst.Shape)
	case instanceColVCN:
		return dashIfEmpty(inst.VcnName)
	case instanceColPrivateIP:
		return inst.PrivateIP
	case instanceColPublicIP:
		return dashIfEmpty(inst.PublicIP)
	case instanceColStatus:
		return dashIfEmpty(inst.Status)
	case instanceColCapacity:
		return dashIfEmpty(inst.CapacityType)
	case instanceColLaunched:
		return dashIfEmpty(inst.Launched)
	case instanceColCompartment:
		return inst.CompartmentName
	case instanceColInstanceID:
		return inst.InstanceID
	default:
		return ""
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func instanceColumnHeaders(columns []instanceColumn) []string {
	headers := make([]string, len(columns))
	for i, col := range columns {
		headers[i] = instanceColumnHeader(col)
	}
	return headers
}

func instanceColumnValues(inst ociinternal.InstanceSummary, columns []instanceColumn) []string {
	row := make([]string, len(columns))
	for i, col := range columns {
		row[i] = instanceColumnValue(inst, col)
	}
	return row
}

var validInstanceStatuses = map[string]struct{}{
	"MOVING":         {},
	"PROVISIONING":   {},
	"RUNNING":        {},
	"STARTING":       {},
	"STOPPING":       {},
	"STOPPED":        {},
	"CREATING_IMAGE": {},
	"TERMINATING":    {},
	"TERMINATED":     {},
}

func parseInstanceStatusFilters(status string) ([]string, error) {
	parts := strings.Split(status, ",")
	filters := make([]string, 0, len(parts))
	seen := make(map[string]bool, len(parts))

	for _, p := range parts {
		trimmed := strings.ToUpper(strings.TrimSpace(p))
		if trimmed == "" {
			continue
		}
		if _, ok := validInstanceStatuses[trimmed]; !ok {
			return nil, fmt.Errorf("unknown status %q — available: MOVING, PROVISIONING, RUNNING, STARTING, STOPPING, STOPPED, CREATING_IMAGE, TERMINATING, TERMINATED", p)
		}
		if seen[trimmed] {
			continue
		}
		seen[trimmed] = true
		filters = append(filters, trimmed)
	}

	return filters, nil
}

func parseInstanceColumns(cols string) ([]instanceColumn, error) {
	if cols == "" {
		out := make([]instanceColumn, len(defaultInstanceColumns))
		copy(out, defaultInstanceColumns)
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

		col, ok := parseInstanceColumnName(trimmed)
		if !ok {
			return nil, fmt.Errorf("unknown column %q — available: name, os, ram, ocpu, shape, vcn, private-ip, public-ip, status, capacity, launched, compartment, instance-id", p)
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

func parseInstanceColumnName(name string) (instanceColumn, bool) {
	switch name {
	case "name":
		return instanceColName, true
	case "os":
		return instanceColOS, true
	case "ram", "ram-gb", "ram_gb", "ramgb":
		return instanceColRAM, true
	case "ocpu":
		return instanceColOCPU, true
	case "shape":
		return instanceColShape, true
	case "vcn":
		return instanceColVCN, true
	case "private-ip", "privateip", "private_ip", "private":
		return instanceColPrivateIP, true
	case "public-ip", "publicip", "public_ip", "public":
		return instanceColPublicIP, true
	case "status":
		return instanceColStatus, true
	case "capacity", "capacity-type", "capacitytype":
		return instanceColCapacity, true
	case "launched":
		return instanceColLaunched, true
	case "compartment":
		return instanceColCompartment, true
	case "instance-id", "instanceid", "instance_id", "id":
		return instanceColInstanceID, true
	default:
		return 0, false
	}
}

func printInstanceList(instances []ociinternal.InstanceSummary, asCSV, asText bool, columns []instanceColumn) error {
	headers := instanceColumnHeaders(columns)

	if asCSV {
		w := csv.NewWriter(os.Stdout)
		if err := w.Write(headers); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, inst := range instances {
			row := instanceColumnValues(inst, columns)
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

	if asText {
		for _, inst := range instances {
			fmt.Println(strings.Join(instanceColumnValues(inst, columns), "\t"))
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
		values := instanceColumnValues(inst, columns)
		row := make(table.Row, len(values))
		for i, v := range values {
			row[i] = v
		}
		t.AppendRow(row)
	}

	footer := make(table.Row, len(headers))
	for i := 0; i < len(footer)-1; i++ {
		footer[i] = ""
	}
	footer[len(footer)-1] = fmt.Sprintf("TOTAL: %d", len(instances))
	t.AppendFooter(footer)

	t.Render()
	return nil
}
