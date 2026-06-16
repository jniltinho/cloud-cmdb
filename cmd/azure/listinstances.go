package azure

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"
	"strings"

	"github.com/jedib0t/go-pretty/v6/table"
	azureinternal "cloud-cmdb/internal/azure"
	"github.com/spf13/cobra"
)

var (
	listInstancesTables        bool
	listInstancesText          bool
	listInstancesCSV           bool
	listInstancesPDF           bool
	listInstancesResourceGroup string
	listInstancesColumns       string
	listInstancesContains      string
	listInstancesStatus        string
)

// ListInstancesCmd is the cobra command for "cloud-cmdb azure list-instances".
var ListInstancesCmd = &cobra.Command{
	Use:   "list-instances",
	Short: "List Azure Virtual Machines in the subscription",
	Long: `List Azure Virtual Machines using the Azure SDK for Go.

Authentication uses the default Azure credential chain:
  1. Environment variables (AZURE_CLIENT_ID, AZURE_CLIENT_SECRET, AZURE_TENANT_ID)
  2. Azure CLI (az login)
  3. Managed Identity (when running in Azure)

The subscription ID is read from --subscription flag or AZURE_SUBSCRIPTION_ID env var.

By default lists all non-terminated VMs across the subscription.
Use --resource-group to restrict to a single resource group.

Default columns:
  NAME, RESOURCE GROUP, TYPE, POWER STATE, PRIVATE IP, PUBLIC IP, LOCATION

Use --columns to choose exactly which columns to show (comma-separated).

Available columns:
  name, resource-group, type, os-type, state, power-state, private-ip, public-ip, location

Note: listing IPs requires extra API calls per VM (NIC lookup). This may be slow
for large subscriptions. Use --resource-group to scope and reduce API calls.

Use --contains to filter by VM name (case-insensitive substring, comma-separated OR logic).
Use --status to filter by power state (e.g. running, deallocated).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated, script friendly)
  --csv      CSV format with header row
  --pdf      Generate PDF report (use --pdf-out for custom filename)`,
	Example: `  # List all VMs in the subscription (uses AZURE_SUBSCRIPTION_ID)
  cloud-cmdb azure list-instances

  # Explicit subscription ID
  cloud-cmdb azure list-instances --subscription 00000000-0000-0000-0000-000000000000

  # Scope to a resource group (faster, fewer API calls)
  cloud-cmdb azure list-instances --resource-group my-rg

  # Custom columns
  cloud-cmdb azure list-instances --columns="name,resource-group,type,power-state,private-ip"

  # Filter by VM name
  cloud-cmdb azure list-instances --contains=web-
  cloud-cmdb azure list-instances --contains=web-,api-,db-

  # Filter by power state
  cloud-cmdb azure list-instances --status=running
  cloud-cmdb azure list-instances --status=running,deallocated

  # Export to CSV
  cloud-cmdb azure list-instances --csv > azure-vms.csv

  # PDF report scoped to a resource group
  cloud-cmdb azure list-instances --resource-group prod-rg --pdf --pdf-out prod-vms.pdf

  # Plain text
  cloud-cmdb azure list-instances --text`,
	RunE: runListInstances,
}

func init() {
	ListInstancesCmd.Flags().BoolVar(&listInstancesTables, "tables", false, "Output as formatted table (default)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesText, "text", false, "Output as plain text (tab-separated)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListInstancesCmd.Flags().BoolVar(&listInstancesPDF, "pdf", false, "Generate PDF with table (use --pdf-out <name> for custom output filename)")
	ListInstancesCmd.Flags().StringVar(&listInstancesResourceGroup, "resource-group", "", "Azure Resource Group name (optional - when omitted, lists across the subscription)")
	ListInstancesCmd.Flags().StringVar(&listInstancesColumns, "columns", "", "Columns to show (comma-separated). Default: name,resource-group,type,power-state,private-ip,public-ip,location. Available: name,resource-group,type,os-type,state,power-state,private-ip,public-ip,location")
	ListInstancesCmd.Flags().StringVar(&listInstancesContains, "contains", "", "Filter by VM name containing any of the given substrings (comma-separated, case-insensitive). Example: --contains=web-,api-")
	ListInstancesCmd.Flags().StringVar(&listInstancesStatus, "status", "", "Filter by power state (comma-separated, case-insensitive). Example: --status=running or --status=running,deallocated")
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

	sub, err := azureinternal.GetSubscriptionID(subscriptionID)
	if err != nil {
		return err
	}

	columns, err := parseColumns(listInstancesColumns)
	if err != nil {
		return err
	}

	instances, err := azureinternal.ListInstances(context.Background(), sub, listInstancesResourceGroup)
	if err != nil {
		return err
	}

	// Filter by power state
	if listInstancesStatus != "" {
		parts := strings.Split(listInstancesStatus, ",")
		filters := make([]string, 0, len(parts))
		for _, p := range parts {
			if t := strings.ToLower(strings.TrimSpace(p)); t != "" {
				filters = append(filters, t)
			}
		}
		if len(filters) > 0 {
			filtered := make([]azureinternal.InstanceSummary, 0, len(instances))
			for _, inst := range instances {
				state := strings.ToLower(strings.TrimSpace(inst.PowerState))
				for _, f := range filters {
					if state == f {
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
			filtered := make([]azureinternal.InstanceSummary, 0, len(instances))
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
			fmt.Printf("No VMs found matching power state '%s' and name filter '%s'.\n", listInstancesStatus, listInstancesContains)
		case listInstancesStatus != "":
			fmt.Printf("No VMs found matching power state '%s'.\n", listInstancesStatus)
		case listInstancesContains != "":
			fmt.Printf("No VMs found matching name filter '%s'.\n", listInstancesContains)
		case listInstancesResourceGroup != "":
			fmt.Println("No VMs found in the specified resource group.")
		default:
			fmt.Println("No VMs found in the subscription.")
		}
		return nil
	}

	if listInstancesPDF {
		headers := columnHeaders(columns)
		data := make([][]string, len(instances))
		for i, inst := range instances {
			data[i] = columnValues(inst, columns)
		}
		return generateAndSavePDF(cmd, "Azure Virtual Machines", headers, data)
	}

	return printInstanceList(instances, listInstancesCSV, listInstancesText, columns)
}

// --- column system ---

type instanceColumn int

const (
	colName instanceColumn = iota
	colResourceGroup
	colType
	colOSType
	colState
	colPowerState
	colPrivateIP
	colPublicIP
	colLocation
)

var defaultColumns = []instanceColumn{
	colName,
	colResourceGroup,
	colType,
	colPowerState,
	colPrivateIP,
	colPublicIP,
	colLocation,
}

func columnHeader(col instanceColumn) string {
	switch col {
	case colName:
		return "NAME"
	case colResourceGroup:
		return "RESOURCE GROUP"
	case colType:
		return "TYPE"
	case colOSType:
		return "OS TYPE"
	case colState:
		return "STATE"
	case colPowerState:
		return "POWER STATE"
	case colPrivateIP:
		return "PRIVATE IP"
	case colPublicIP:
		return "PUBLIC IP"
	case colLocation:
		return "LOCATION"
	default:
		return ""
	}
}

func columnValue(inst azureinternal.InstanceSummary, col instanceColumn) string {
	dash := func(s string) string {
		if s == "" {
			return "-"
		}
		return s
	}
	switch col {
	case colName:
		return dash(inst.Name)
	case colResourceGroup:
		return dash(inst.ResourceGroup)
	case colType:
		return dash(inst.Type)
	case colOSType:
		return dash(inst.OSType)
	case colState:
		return dash(inst.State)
	case colPowerState:
		return dash(inst.PowerState)
	case colPrivateIP:
		return dash(inst.PrivateIP)
	case colPublicIP:
		return dash(inst.PublicIP)
	case colLocation:
		return dash(inst.Location)
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

func columnValues(inst azureinternal.InstanceSummary, cols []instanceColumn) []string {
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
			return nil, fmt.Errorf("unknown column %q — available: name, resource-group, type, os-type, state, power-state, private-ip, public-ip, location", p)
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
	case "resource-group", "resourcegroup", "rg":
		return colResourceGroup, true
	case "type", "vm-size", "size":
		return colType, true
	case "os-type", "ostype", "os":
		return colOSType, true
	case "state", "provisioning-state":
		return colState, true
	case "power-state", "powerstate", "status":
		return colPowerState, true
	case "private-ip", "privateip":
		return colPrivateIP, true
	case "public-ip", "publicip":
		return colPublicIP, true
	case "location", "region":
		return colLocation, true
	default:
		return 0, false
	}
}

// --- output renderers ---

func printInstanceList(instances []azureinternal.InstanceSummary, asCSV, asText bool, columns []instanceColumn) error {
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
