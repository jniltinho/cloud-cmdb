// Package gcp provides functions for querying Google Cloud Compute Engine resources
// using the Cloud Compute REST API with Application Default Credentials (ADC).
package gcp

import (
	"context"
	"fmt"
	"os"
	"strings"

	compute "cloud.google.com/go/compute/apiv1"
	"cloud.google.com/go/compute/apiv1/computepb"
	"google.golang.org/api/iterator"
)

// InstanceSummary is the flat representation of a GCE instance used by list-instances.
// PublicIP is empty for instances without an external IP (internal-only VMs).
type InstanceSummary struct {
	Name        string
	Zone        string
	MachineType string
	Status      string
	PrivateIP   string
	PublicIP    string
	Project     string
}

// GetProjectID returns the project ID from the flag value or well-known env vars.
func GetProjectID(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	for _, env := range []string{"GOOGLE_CLOUD_PROJECT", "GCLOUD_PROJECT", "GCP_PROJECT"} {
		if v := os.Getenv(env); v != "" {
			return v, nil
		}
	}
	return "", fmt.Errorf("project ID required: use --project flag or set GOOGLE_CLOUD_PROJECT")
}

// ListInstances returns all non-terminated Compute Engine instances in the project.
// When zone is non-empty, only instances in that zone are returned.
// Authentication uses Application Default Credentials (ADC):
//   - GOOGLE_APPLICATION_CREDENTIALS env var (service account JSON)
//   - gcloud auth application-default login
//   - GCE metadata service (when running on GCP)
func ListInstances(ctx context.Context, projectID, zone string) ([]InstanceSummary, error) {
	client, err := compute.NewInstancesRESTClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to create Compute Engine client: %w", err)
	}
	defer client.Close()

	if zone != "" {
		return listByZone(ctx, client, projectID, zone)
	}
	return listAll(ctx, client, projectID)
}

func listAll(ctx context.Context, client *compute.InstancesClient, projectID string) ([]InstanceSummary, error) {
	it := client.AggregatedList(ctx, &computepb.AggregatedListInstancesRequest{
		Project: projectID,
	})

	var instances []InstanceSummary
	for {
		pair, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list instances: %w", err)
		}
		if pair.Value == nil {
			continue
		}
		for _, inst := range pair.Value.Instances {
			if s := ptrStr(inst.Status); s == "TERMINATED" {
				continue
			}
			instances = append(instances, toSummary(inst, projectID))
		}
	}
	return instances, nil
}

func listByZone(ctx context.Context, client *compute.InstancesClient, projectID, zone string) ([]InstanceSummary, error) {
	it := client.List(ctx, &computepb.ListInstancesRequest{
		Project: projectID,
		Zone:    zone,
	})

	var instances []InstanceSummary
	for {
		inst, err := it.Next()
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("failed to list instances in zone %q: %w", zone, err)
		}
		if s := ptrStr(inst.Status); s == "TERMINATED" {
			continue
		}
		instances = append(instances, toSummary(inst, projectID))
	}
	return instances, nil
}

func toSummary(inst *computepb.Instance, projectID string) InstanceSummary {
	s := InstanceSummary{
		Name:        ptrStr(inst.Name),
		Zone:        shortName(ptrStr(inst.Zone)),
		MachineType: shortName(ptrStr(inst.MachineType)),
		Status:      ptrStr(inst.Status),
		PrivateIP:   "-",
		PublicIP:    "-",
		Project:     projectID,
	}

	if len(inst.NetworkInterfaces) > 0 {
		nic := inst.NetworkInterfaces[0]
		if nic.NetworkIP != nil {
			s.PrivateIP = *nic.NetworkIP
		}
		if len(nic.AccessConfigs) > 0 && nic.AccessConfigs[0].NatIP != nil {
			s.PublicIP = *nic.AccessConfigs[0].NatIP
		}
	}

	return s
}

// shortName extracts the last path segment from a GCP resource URL or path.
// e.g. ".../zones/us-central1-a" → "us-central1-a"
//
//	".../machineTypes/n1-standard-2" → "n1-standard-2"
func shortName(url string) string {
	if url == "" {
		return "-"
	}
	parts := strings.Split(url, "/")
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			return parts[i]
		}
	}
	return url
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
