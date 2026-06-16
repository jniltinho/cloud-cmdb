package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// SubnetEntry is a minimal view of a subnet for display (name, CIDR, VCN name, compartment name).
// OCIDs are always populated for JSON / --full use cases.
// When requested via withIPCount, AllocatedIPs contains the actual count of assigned private IPs.
type SubnetEntry struct {
	Name            string `json:"name"`
	OCID            string `json:"ocid"`
	CidrBlock       string `json:"cidr_block"`
	VcnName         string `json:"vcn_name"`
	VcnId           string `json:"vcn_id"`
	CompartmentName string `json:"compartment_name"`
	AllocatedIPs    int    `json:"allocated_ips,omitempty"`
}

// ListSubnets lists subnets in OCI.
//   - If compartmentID is empty: enumerates ALL compartments (incl. root tenancy)
//     and returns subnets from every compartment.
//   - If compartmentID is provided (name or OCID): lists only subnets in that compartment.
//
// VcnName is resolved via ListVcns (human-friendly name). Falls back to empty string
// if the VCN cannot be resolved (entry will still contain VcnId for JSON/--full).
//
// Uses the official VirtualNetwork client + automatic SDK retries for rate limits.
func ListSubnets(ctx context.Context, compartmentID, configFile, profile string) ([]SubnetEntry, error) {
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

	// Enable automatic SDK retry for 429 TooManyRequests (consistent with other commands)
	retryPolicy := common.DefaultRetryPolicy()
	vnClient.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Fetch all compartments upfront so we can show nice names instead of OCIDs
	comps, err := ListAllCompartments(ctx, configFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments for subnet enumeration: %w", err)
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

	var results []SubnetEntry

	for _, compID := range targetCompartments {
		compartmentName := nameByID[compID]
		if compartmentName == "" {
			compartmentName = compID // fallback
		}

		// 1. List all VCNs in this compartment so we can show VCN names (not just OCIDs)
		vcnNameByID := make(map[string]string)
		vcnReq := core.ListVcnsRequest{
			CompartmentId: &compID,
		}
		for {
			vcnResp, err := vnClient.ListVcns(ctx, vcnReq)
			if err != nil {
				// Non-fatal: we can still return subnets with VcnId only
				break
			}
			for _, v := range vcnResp.Items {
				if v.Id != nil {
					name := ""
					if v.DisplayName != nil {
						name = *v.DisplayName
					}
					vcnNameByID[*v.Id] = name
				}
			}
			if vcnResp.OpcNextPage == nil {
				break
			}
			vcnReq.Page = vcnResp.OpcNextPage
		}

		// 2. List all subnets in the compartment (with pagination)
		req := core.ListSubnetsRequest{
			CompartmentId: &compID,
		}

		for {
			resp, err := vnClient.ListSubnets(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to list subnets in compartment %s: %w", compID, err)
			}

			for _, s := range resp.Items {
				// Skip terminated/terminating subnets
				if s.LifecycleState == core.SubnetLifecycleStateTerminated ||
					s.LifecycleState == core.SubnetLifecycleStateTerminating {
					continue
				}

				entry := SubnetEntry{
					CompartmentName: compartmentName,
				}
				if s.DisplayName != nil {
					entry.Name = *s.DisplayName
				}
				if s.Id != nil {
					entry.OCID = *s.Id
				}
				if s.CidrBlock != nil {
					entry.CidrBlock = *s.CidrBlock
				}
				if s.VcnId != nil {
					entry.VcnId = *s.VcnId
					if vcnName, ok := vcnNameByID[entry.VcnId]; ok && vcnName != "" {
						entry.VcnName = vcnName
					}
				}

				if s.Id != nil {
					if count, err := countAllocatedIPs(ctx, vnClient, *s.Id); err == nil {
						entry.AllocatedIPs = count
					}
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

// countAllocatedIPs returns the number of private IPs currently assigned in the given subnet.
// It uses ListPrivateIps with SubnetId filter and handles pagination.
func countAllocatedIPs(ctx context.Context, vnClient core.VirtualNetworkClient, subnetID string) (int, error) {
	var count int
	req := core.ListPrivateIpsRequest{
		SubnetId: &subnetID,
	}

	for {
		resp, err := vnClient.ListPrivateIps(ctx, req)
		if err != nil {
			return 0, err
		}
		count += len(resp.Items)

		if resp.OpcNextPage == nil {
			break
		}
		req.Page = resp.OpcNextPage
	}

	return count, nil
}
