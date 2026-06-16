// Package cmd wires all provider subcommand groups (oci, aws, azure, gcp) into
// the root cobra command and exposes Execute for main to call.
package cmd

import (
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:           "cloud-cmdb",
	Short:         "Multi-cloud CMDB CLI",
	Long:          `cloud-cmdb is a collection of utilities for querying cloud resources across providers.`,
	SilenceUsage:  true,
	SilenceErrors: true,
}

// Execute runs the root cobra command and returns any error.
// main calls this; a non-nil return causes the process to exit with status 1.
func Execute() error {
	return rootCmd.Execute()
}
