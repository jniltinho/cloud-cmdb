// Package oci provides functions for querying Oracle Cloud Infrastructure resources
// using the official oci-go-sdk with automatic SDK retries on rate limits (429).
package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// InstanceSummary is the unified rich view of a compute instance used by list-instances.
// It contains both basic networking info and server inventory details (OS, RAM, OCPU, Shape).
type InstanceSummary struct {
	Name            string `json:"name"`
	InstanceID      string `json:"instance_id"`
	Status          string `json:"status"`
	PrivateIP       string `json:"private_ip"`
	PublicIP        string `json:"public_ip,omitempty"`
	CompartmentName string `json:"compartment_name"`

	// Rich inventory fields (populated when using ListInstances)
	OS    string `json:"os,omitempty"`
	RAMGB string `json:"ram_gb,omitempty"`
	OCPU  string `json:"ocpu,omitempty"`
	Shape string `json:"shape,omitempty"`

	// Additional inventory fields
	VcnName      string `json:"vcn_name,omitempty"`
	CapacityType string `json:"capacity_type,omitempty"`
	Launched     string `json:"launched,omitempty"`
}

// ServerInfo represents a richer server inventory view used by "list-servers".
// It includes operating system (from the source image) and configured RAM.
type ServerInfo struct {
	Name            string `json:"name"`
	OS              string `json:"os"`
	RAMGB           string `json:"ram_gb"`
	PrivateIP       string `json:"private_ip"`
	PublicIP        string `json:"public_ip,omitempty"`
	Status          string `json:"status"`
	CompartmentName string `json:"compartment_name"`
	Shape           string `json:"shape,omitempty"`
}

// ListInstances lists non-terminated compute instances (excludes TERMINATED/TERMINATING).
//   - If compartmentID is empty: enumerates ALL compartments (incl. root tenancy)
//     and returns every instance with its compartment display name.
//   - If compartmentID is provided (name or OCID): lists only instances in that compartment.
//
// Each instance is enriched with the primary VNIC's private and public IP (when assigned).
// Uses the official OCI Go SDK with automatic retries on rate limits.
func ListInstances(ctx context.Context, compartmentID, configFile, profile string) ([]InstanceSummary, error) {
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	computeClient, err := core.NewComputeClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Compute client: %w", err)
	}

	vnClient, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirtualNetwork client: %w", err)
	}

	// Enable SDK retry for 429
	retryPolicy := common.DefaultRetryPolicy()
	computeClient.SetCustomClientConfiguration(common.CustomClientConfiguration{RetryPolicy: &retryPolicy})
	vnClient.SetCustomClientConfiguration(common.CustomClientConfiguration{RetryPolicy: &retryPolicy})

	// Fetch all compartments upfront so we can show nice names instead of OCIDs
	comps, err := ListAllCompartments(ctx, configFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments for instance enumeration: %w", err)
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

	// Cache for image names (to resolve OS without repeated GetImage calls)
	imageNameCache := make(map[string]string)

	// Caches for VCN resolution (to avoid repeated GetSubnet / GetVcn calls)
	vcnNameCache   := make(map[string]string)
	subnetVcnCache := make(map[string]string)

	var results []InstanceSummary

	for _, compID := range targetCompartments {
		compartmentName := nameByID[compID]
		if compartmentName == "" {
			compartmentName = compID // fallback
		}

		// 1. List all instances in the compartment (with pagination)
		var allInstances []core.Instance
		instReq := core.ListInstancesRequest{
			CompartmentId: &compID,
		}

		for {
			resp, err := computeClient.ListInstances(ctx, instReq)
			if err != nil {
				return nil, fmt.Errorf("failed to list instances in compartment %s: %w", compID, err)
			}
			allInstances = append(allInstances, resp.Items...)

			if resp.OpcNextPage == nil {
				break
			}
			instReq.Page = resp.OpcNextPage
		}

		// 2. For each instance, collect VNIC attachments, fetch primary VNIC IPs,
		//    and enrich with Shape, OCPU, RAM and OS information.
		for _, inst := range allInstances {
			// Skip terminated/terminating instances
			if inst.LifecycleState == core.InstanceLifecycleStateTerminated ||
				inst.LifecycleState == core.InstanceLifecycleStateTerminating {
				continue
			}

			name := ""
			if inst.DisplayName != nil {
				name = *inst.DisplayName
			}
			instID := ""
			if inst.Id != nil {
				instID = *inst.Id
			}

			// Shape
			shape := ""
			if inst.Shape != nil {
				shape = *inst.Shape
			}

			// OCPU and RAM from ShapeConfig
			ocpu := ""
			ram := ""
			if inst.ShapeConfig != nil {
				if inst.ShapeConfig.Ocpus != nil {
					ocpu = fmt.Sprintf("%.1f", *inst.ShapeConfig.Ocpus)
				}
				if inst.ShapeConfig.MemoryInGBs != nil {
					ram = fmt.Sprintf("%.0f", *inst.ShapeConfig.MemoryInGBs)
				}
			}

			// Resolve OS name from source image (when available)
			osName := ""
			if inst.SourceDetails != nil {
				if viaImage, ok := inst.SourceDetails.(core.InstanceSourceViaImageDetails); ok {
					if viaImage.ImageId != nil {
						imgID := *viaImage.ImageId
						if cached, ok := imageNameCache[imgID]; ok {
							osName = cached
						} else {
							getImg, err := computeClient.GetImage(ctx, core.GetImageRequest{ImageId: &imgID})
							if err == nil && getImg.Image.DisplayName != nil {
								osName = *getImg.Image.DisplayName
								imageNameCache[imgID] = osName
							}
						}
					}
				}
			}
			if osName == "" {
				osName = "-"
			}

			// List VNIC attachments for this instance
			vaReq := core.ListVnicAttachmentsRequest{
				CompartmentId: &compID,
				InstanceId:    inst.Id,
			}

			var vnicIDs []string
			for {
				vaResp, err := computeClient.ListVnicAttachments(ctx, vaReq)
				if err != nil {
					// Do not fail the whole run for one instance
					break
				}
				for _, va := range vaResp.Items {
					if va.VnicId != nil && va.LifecycleState != core.VnicAttachmentLifecycleStateDetached {
						vnicIDs = append(vnicIDs, *va.VnicId)
					}
				}
				if vaResp.OpcNextPage == nil {
					break
				}
				vaReq.Page = vaResp.OpcNextPage
			}

			// Fetch VNIC details, prefer primary VNIC
			privateIP := ""
			publicIP := ""
			vcnName := ""
			for _, vid := range vnicIDs {
				getResp, err := vnClient.GetVnic(ctx, core.GetVnicRequest{VnicId: &vid})
				if err != nil {
					continue
				}
				vnic := getResp.Vnic

				isPrimary := vnic.IsPrimary != nil && *vnic.IsPrimary

				if isPrimary {
					// Primary VNIC: always prefer its IPs
					if vnic.PrivateIp != nil {
						privateIP = *vnic.PrivateIp
					}
					if vnic.PublicIp != nil {
						publicIP = *vnic.PublicIp
					}

					// Resolve VCN name (with caching)
					if vnic.SubnetId != nil {
						subnetID := *vnic.SubnetId
						if cachedVcnID, ok := subnetVcnCache[subnetID]; ok {
							if cachedVcnID != "" {
								if cachedName, ok := vcnNameCache[cachedVcnID]; ok {
									vcnName = cachedName
								}
							}
						} else {
							// Get Subnet to find its VCN
							subnetResp, err := vnClient.GetSubnet(ctx, core.GetSubnetRequest{SubnetId: &subnetID})
							if err == nil && subnetResp.Subnet.VcnId != nil {
								vcnID := *subnetResp.Subnet.VcnId
								subnetVcnCache[subnetID] = vcnID

								if cachedName, ok := vcnNameCache[vcnID]; ok {
									vcnName = cachedName
								} else {
									vcnResp, err := vnClient.GetVcn(ctx, core.GetVcnRequest{VcnId: &vcnID})
									if err == nil && vcnResp.Vcn.DisplayName != nil {
										vcnName = *vcnResp.Vcn.DisplayName
										vcnNameCache[vcnID] = vcnName
									} else {
										vcnNameCache[vcnID] = "" // avoid retrying
									}
								}
							} else {
								subnetVcnCache[subnetID] = "" // avoid retrying
							}
						}
					}

					// We can break early if we only care about primary
					break
				} else if privateIP == "" {
					// Fallback: use first non-primary if no primary found yet
					if vnic.PrivateIp != nil {
						privateIP = *vnic.PrivateIp
					}
					if vnic.PublicIp != nil {
						publicIP = *vnic.PublicIp
					}
				}
			}

			// Capacity Type
			capacityType := "On-Demand"
			if inst.PreemptibleInstanceConfig != nil {
				capacityType = "Preemptible"
			} else if inst.CapacityReservationId != nil && *inst.CapacityReservationId != "" {
				capacityType = "Reserved"
			}

			// Launched (TimeCreated)
			launched := ""
			if inst.TimeCreated != nil {
				launched = inst.TimeCreated.Format("2006-01-02")
			}

			results = append(results, InstanceSummary{
				Name:            name,
				InstanceID:      instID,
				Status:          string(inst.LifecycleState),
				PrivateIP:       privateIP,
				PublicIP:        publicIP,
				CompartmentName: compartmentName,
				OS:              osName,
				RAMGB:           ram,
				OCPU:            ocpu,
				Shape:           shape,
				VcnName:         vcnName,
				CapacityType:    capacityType,
				Launched:        launched,
			})
		}
	}

	return results, nil
}

// ListServers returns a detailed server list (kept for backward compatibility with the
// "list-servers" command). It now delegates to the unified ListInstances and converts
// the result to the legacy ServerInfo shape.
func ListServers(ctx context.Context, compartmentID, configFile, profile string) ([]ServerInfo, error) {
	instances, err := ListInstances(ctx, compartmentID, configFile, profile)
	if err != nil {
		return nil, err
	}

	results := make([]ServerInfo, len(instances))
	for i, inst := range instances {
		results[i] = ServerInfo{
			Name:            inst.Name,
			OS:              inst.OS,
			RAMGB:           inst.RAMGB,
			PrivateIP:       inst.PrivateIP,
			PublicIP:        inst.PublicIP,
			Status:          inst.Status,
			CompartmentName: inst.CompartmentName,
			Shape:           inst.Shape,
		}
	}
	return results, nil
}
