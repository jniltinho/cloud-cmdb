package oci

import (
	"context"
	"encoding/csv"
	"fmt"
	"os"

	"github.com/jedib0t/go-pretty/v6/table"
	ociapi "cloud-cmdb/internal/oci"
	"github.com/spf13/cobra"
)

var (
	listServersTables      bool
	listServersText        bool
	listServersCSV         bool
	listServersPDF         bool
	listServersCompartment string
	listServersFull        bool
)

// ListServersCmd is kept for backward compatibility.
// New users should prefer: cloud-cmdb oci list-instances
var ListServersCmd = &cobra.Command{
	Use:   "list-servers",
	Short: "List compute instances (servers) with OS, RAM, IPs and compartment (deprecated - use list-instances)",
	Long: `List compute instances (servers) across the tenancy or a specific compartment.

This command is kept for backward compatibility.
The recommended command is now:

    cloud-cmdb oci list-instances

(list-instances shows the rich inventory by default, including OS, RAM (GB), OCPU and SHAPE).

Columns (this command):
  - NAME, OS, RAM (GB), PRIVATE IP, PUBLIC IP, STATUS, COMPARTMENT
  - --full also adds SHAPE

Use --compartment to restrict to a single compartment (name or OCID supported).

Output formats:
  --tables   Pretty table (default)
  --text     Plain text (tab-separated)
  --csv      CSV format with header row
  --pdf      PDF report (use --pdf-out for custom filename)`,
	Example: `  # Preferred (rich inventory is default, includes OCPU):
  cloud-cmdb oci list-instances

  # Legacy command still works:
  cloud-cmdb oci list-servers

  # Legacy with Shape column
  cloud-cmdb oci list-servers --full

  # List servers inside one compartment
  cloud-cmdb oci list-servers --compartment Production --full

  # Export to CSV
  cloud-cmdb oci list-servers --csv > servers.csv

  # Generate PDF report
  cloud-cmdb oci list-servers --pdf --pdf-out inventario.pdf`,
	RunE: runListServers,
}

func init() {
	ListServersCmd.Flags().BoolVar(&listServersTables, "tables", false, "Output as formatted table (default)")
	ListServersCmd.Flags().BoolVar(&listServersText, "text", false, "Output as plain text (tab-separated)")
	ListServersCmd.Flags().BoolVar(&listServersCSV, "csv", false, "Output as CSV with header row (suitable for spreadsheets)")
	ListServersCmd.Flags().BoolVar(&listServersPDF, "pdf", false, "Generate PDF with table (use --pdf-out for custom filename)")
	ListServersCmd.Flags().StringVar(&listServersCompartment, "compartment", "", "Compartment name or OCID (optional - when omitted, lists all compartments)")
	ListServersCmd.Flags().BoolVar(&listServersFull, "full", false, "Show Shape column (in addition to OS, RAM, etc.)")
}

func runListServers(cmd *cobra.Command, args []string) error {
	if listServersTables && listServersText {
		return fmt.Errorf("cannot use --tables and --text together")
	}
	if listServersCSV && (listServersTables || listServersText) {
		return fmt.Errorf("cannot use --csv together with --tables or --text")
	}
	if listServersPDF && (listServersCSV || listServersText) {
		return fmt.Errorf("cannot use --pdf together with --csv or --text")
	}

	servers, err := ociapi.ListServers(context.Background(), listServersCompartment, configFile, profile)
	if err != nil {
		return err
	}

	if len(servers) == 0 {
		if listServersCompartment != "" {
			fmt.Println("No servers found in the specified compartment.")
		} else {
			fmt.Println("No servers found in the tenancy.")
		}
		return nil
	}

	if listServersPDF {
		headers := []string{"NAME", "OS", "RAM (GB)", "PRIVATE IP", "PUBLIC IP", "STATUS", "COMPARTMENT"}
		if listServersFull {
			headers = append(headers, "SHAPE")
		}

		data := make([][]string, len(servers))
		for i, s := range servers {
			pub := s.PublicIP
			if pub == "" {
				pub = "-"
			}
			ram := s.RAMGB
			if ram == "" {
				ram = "-"
			}
			status := s.Status
			if status == "" {
				status = "-"
			}
			srvOS := s.OS
			if srvOS == "" {
				srvOS = "-"
			}

			row := []string{s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName}
			if listServersFull {
				shape := s.Shape
				if shape == "" {
					shape = "-"
				}
				row = append(row, shape)
			}
			data[i] = row
		}
		return generateAndSavePDF(cmd, "OCI Servers", headers, data)
	}

	return printServerList(servers, listServersCSV, listServersText)
}

func printServerList(servers []ociapi.ServerInfo, asCSV, asText bool) error {
	if asCSV {
		w := csv.NewWriter(os.Stdout)

		header := []string{"NAME", "OS", "RAM (GB)", "PRIVATE IP", "PUBLIC IP", "STATUS", "COMPARTMENT"}
		if listServersFull {
			header = append(header, "SHAPE")
		}
		if err := w.Write(header); err != nil {
			return fmt.Errorf("failed to write CSV header: %w", err)
		}

		for _, s := range servers {
			pub := s.PublicIP
			if pub == "" {
				pub = "-"
			}
			ram := s.RAMGB
			if ram == "" {
				ram = "-"
			}
			status := s.Status
			if status == "" {
				status = "-"
			}
			srvOS := s.OS
			if srvOS == "" {
				srvOS = "-"
			}

			row := []string{s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName}
			if listServersFull {
				shape := s.Shape
				if shape == "" {
					shape = "-"
				}
				row = append(row, shape)
			}
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
		for _, s := range servers {
			pub := s.PublicIP
			if pub == "" {
				pub = "-"
			}
			ram := s.RAMGB
			if ram == "" {
				ram = "-"
			}
			status := s.Status
			if status == "" {
				status = "-"
			}
			srvOS := s.OS
			if srvOS == "" {
				srvOS = "-"
			}

			if listServersFull {
				shape := s.Shape
				if shape == "" {
					shape = "-"
				}
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName, shape)
			} else {
				fmt.Printf("%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
					s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName)
			}
		}
		return nil
	}

	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)

	if listServersFull {
		t.AppendHeader(table.Row{"NAME", "OS", "RAM (GB)", "PRIVATE IP", "PUBLIC IP", "STATUS", "COMPARTMENT", "SHAPE"})
	} else {
		t.AppendHeader(table.Row{"NAME", "OS", "RAM (GB)", "PRIVATE IP", "PUBLIC IP", "STATUS", "COMPARTMENT"})
	}

	for _, s := range servers {
		pub := s.PublicIP
		if pub == "" {
			pub = "-"
		}
		ram := s.RAMGB
		if ram == "" {
			ram = "-"
		}
		status := s.Status
		if status == "" {
			status = "-"
		}
		srvOS := s.OS
		if srvOS == "" {
			srvOS = "-"
		}

		if listServersFull {
			shape := s.Shape
			if shape == "" {
				shape = "-"
			}
			t.AppendRow(table.Row{s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName, shape})
		} else {
			t.AppendRow(table.Row{s.Name, srvOS, ram, s.PrivateIP, pub, status, s.CompartmentName})
		}
	}

	if listServersFull {
		t.AppendFooter(table.Row{"", "", "", "", "", "", "", fmt.Sprintf("TOTAL: %d", len(servers))})
	} else {
		t.AppendFooter(table.Row{"", "", "", "", "", "", fmt.Sprintf("TOTAL: %d", len(servers))})
	}
	t.Render()
	return nil
}
