package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// VCNEntry is a minimal view of a VCN for display (name, primary CIDR, compartment name).
type VCNEntry struct {
	Name            string `json:"name"`
	OCID            string `json:"ocid"`
	CidrBlock       string `json:"cidr_block"`
	CompartmentName string `json:"compartment_name"`
}

// ListVcns lists VCNs in OCI.
//   - If compartmentID is empty: enumerates ALL compartments (incl. root tenancy)
//     and returns VCNs from every compartment.
//   - If compartmentID is provided (name or OCID): lists only VCNs in that compartment.
//
// CompartmentName is resolved via ListAllCompartments (human-friendly name).
//
// Skips TERMINATED and TERMINATING VCNs.
// Uses the official VirtualNetwork client + automatic SDK retries for rate limits.
func ListVcns(ctx context.Context, compartmentID, configFile, profile string) ([]VCNEntry, error) {
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirtualNetwork client: %w", err)
	}

	// Enable automatic SDK retry for 429 TooManyRequests
	retryPolicy := common.DefaultRetryPolicy()
	vnClient.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Fetch all compartments upfront so we can show nice names instead of OCIDs
	comps, err := ListAllCompartments(ctx, configFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments for VCN enumeration: %w", err)
	}
	nameByID := make(map[string]string, len(comps))
	for _, c := range comps {
		nameByID[c.OCID] = c.Name
	}

	// Determine which compartment(s) to query
	var targetCompartments []string
	if compartmentID != "" {
		resolved, err := ResolveCompartmentID(ctx, compartmentID, configFile, profile)
		if err != nil {
			return nil, err
		}
		targetCompartments = []string{resolved}
	} else {
		for _, c := range comps {
			targetCompartments = append(targetCompartments, c.OCID)
		}
	}

	var results []VCNEntry

	for _, compID := range targetCompartments {
		compartmentName := nameByID[compID]
		if compartmentName == "" {
			compartmentName = compID // fallback
		}

		req := core.ListVcnsRequest{
			CompartmentId: &compID,
		}

		for {
			resp, err := vnClient.ListVcns(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to list VCNs in compartment %s: %w", compID, err)
			}

			for _, v := range resp.Items {
				// Skip terminated/terminating VCNs
				if v.LifecycleState == core.VcnLifecycleStateTerminated ||
					v.LifecycleState == core.VcnLifecycleStateTerminating {
					continue
				}

				entry := VCNEntry{
					CompartmentName: compartmentName,
				}
				if v.DisplayName != nil {
					entry.Name = *v.DisplayName
				}
				if v.Id != nil {
					entry.OCID = *v.Id
				}
				if v.CidrBlock != nil {
					entry.CidrBlock = *v.CidrBlock
				}
				results = append(results, entry)
			}

			if resp.OpcNextPage == nil {
				break
			}
			req.Page = resp.OpcNextPage
		}
	}

	return results, nil
}
