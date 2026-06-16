package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/containerengine"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// OKEClusterEntry is a minimal view of an OKE cluster for display.
type OKEClusterEntry struct {
	Name              string `json:"name"`
	OCID              string `json:"ocid"`
	KubernetesVersion string `json:"kubernetes_version"`
	State             string `json:"state"`
	CompartmentName   string `json:"compartment_name"`
	VcnName           string `json:"vcn_name"`
	VcnId             string `json:"vcn_id"`
	Endpoint          string `json:"endpoint"`
	Type              string `json:"type"`

	// New fields
	Created   string `json:"created"`
	CreatedBy string `json:"created_by"`
	Updated   string `json:"updated"`
	Tags      string `json:"tags"`
}

// ListOKEClusters lists OKE (Container Engine for Kubernetes) clusters.
//   - If compartmentID is empty: enumerates ALL compartments (incl. root tenancy)
//     and returns clusters from every compartment.
//   - If compartmentID is provided (name or OCID): lists only clusters in that compartment.
//
// VcnName is resolved automatically for human-friendly output.
// Skips clusters in DELETED or DELETING state (consistent with other list commands).
// Uses the official ContainerEngine client + automatic SDK retries for rate limits.
func ListOKEClusters(ctx context.Context, compartmentID, configFile, profile string) ([]OKEClusterEntry, error) {
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	ceClient, err := containerengine.NewContainerEngineClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create ContainerEngine client: %w", err)
	}

	// Enable automatic SDK retry for 429 TooManyRequests (consistent with other commands)
	retryPolicy := common.DefaultRetryPolicy()
	ceClient.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Also create VirtualNetwork client for VCN name resolution (used inside per-compartment loop)
	vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirtualNetwork client: %w", err)
	}
	vnClient.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Fetch all compartments upfront so we can show nice names instead of OCIDs
	comps, err := ListAllCompartments(ctx, configFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments for OKE cluster enumeration: %w", err)
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

	var results []OKEClusterEntry

	for _, compID := range targetCompartments {
		compartmentName := nameByID[compID]
		if compartmentName == "" {
			compartmentName = compID // fallback
		}

		// Build VCN name map for this compartment (for friendly VCN display in cluster list)
		vcnNameByID := make(map[string]string)
		vcnReq := core.ListVcnsRequest{
			CompartmentId: &compID,
		}
		for {
			vcnResp, err := vnClient.ListVcns(ctx, vcnReq)
			if err != nil {
				// Non-fatal: clusters will still show VcnId (or abbreviated)
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

		// Now list clusters in this compartment
		req := containerengine.ListClustersRequest{
			CompartmentId: &compID,
		}

		for {
			resp, err := ceClient.ListClusters(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to list OKE clusters in compartment %s: %w", compID, err)
			}

			for _, c := range resp.Items {
				// Skip deleted/terminating clusters (user rarely wants to see these in list)
				if c.LifecycleState == containerengine.ClusterLifecycleStateDeleted ||
					c.LifecycleState == containerengine.ClusterLifecycleStateDeleting {
					continue
				}

				entry := OKEClusterEntry{
					CompartmentName: compartmentName,
				}

				if c.Name != nil {
					entry.Name = *c.Name
				}
				if c.Id != nil {
					entry.OCID = *c.Id
				}
				if c.KubernetesVersion != nil {
					entry.KubernetesVersion = *c.KubernetesVersion
				}
				entry.State = string(c.LifecycleState)

				if c.VcnId != nil {
					entry.VcnId = *c.VcnId
					if name := vcnNameByID[entry.VcnId]; name != "" {
						entry.VcnName = name
					}
				}

				// Pick the most useful endpoint for display
				if c.Endpoints != nil {
					switch {
					case c.Endpoints.PublicEndpoint != nil && *c.Endpoints.PublicEndpoint != "":
						entry.Endpoint = *c.Endpoints.PublicEndpoint
					case c.Endpoints.PrivateEndpoint != nil && *c.Endpoints.PrivateEndpoint != "":
						entry.Endpoint = *c.Endpoints.PrivateEndpoint
					case c.Endpoints.VcnHostnameEndpoint != nil && *c.Endpoints.VcnHostnameEndpoint != "":
						entry.Endpoint = *c.Endpoints.VcnHostnameEndpoint
					case c.Endpoints.Kubernetes != nil && *c.Endpoints.Kubernetes != "":
						entry.Endpoint = *c.Endpoints.Kubernetes
					}
				}

				if c.Type != "" {
					entry.Type = string(c.Type)
				}

				// Populate new metadata fields
				if c.Metadata != nil {
					if c.Metadata.TimeCreated != nil {
						entry.Created = c.Metadata.TimeCreated.Format("2006-01-02")
					}
					if c.Metadata.CreatedByUserId != nil {
						entry.CreatedBy = *c.Metadata.CreatedByUserId
					}
					if c.Metadata.TimeUpdated != nil {
						entry.Updated = c.Metadata.TimeUpdated.Format("2006-01-02")
					}
				}

				// Freeform tags as key=value,key2=value2
				if len(c.FreeformTags) > 0 {
					tagList := make([]string, 0, len(c.FreeformTags))
					for k, v := range c.FreeformTags {
						tagList = append(tagList, k+"="+v)
					}
					entry.Tags = strings.Join(tagList, ",")
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
