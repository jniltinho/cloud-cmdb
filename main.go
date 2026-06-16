// Package main is the entry point for the cloud-cmdb CLI.
package main

import (
	"os"

	"cloud-cmdb/cmd"
)

func main() {
	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
