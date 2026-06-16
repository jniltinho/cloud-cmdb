// Package azure provides functions for querying Azure Virtual Machine resources
// using the Azure SDK for Go with the Default Azure Credential chain
// (env vars → Azure CLI → Managed Identity).
package azure

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/Azure/azure-sdk-for-go/sdk/azidentity"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/compute/armcompute/v6"
	"github.com/Azure/azure-sdk-for-go/sdk/resourcemanager/network/armnetwork/v6"
)

// InstanceSummary is the flat representation of an Azure VM used by list-instances.
type InstanceSummary struct {
	Name          string
	ResourceGroup string
	Location      string
	Type          string // VM size, e.g. Standard_D2s_v3
	OSType        string // Windows, Linux
	State         string // provisioning state: Succeeded, Failed, etc.
	PowerState    string // running, deallocated, stopped, etc.
	PrivateIP     string
	PublicIP      string
}

// GetSubscriptionID returns the subscription ID from the flag value or AZURE_SUBSCRIPTION_ID env var.
func GetSubscriptionID(flagValue string) (string, error) {
	if flagValue != "" {
		return flagValue, nil
	}
	sub := os.Getenv("AZURE_SUBSCRIPTION_ID")
	if sub == "" {
		return "", fmt.Errorf("subscription ID required: use --subscription flag or set AZURE_SUBSCRIPTION_ID")
	}
	return sub, nil
}

// ListInstances returns all non-terminated VM instances in the subscription.
// When resourceGroup is non-empty, only VMs in that resource group are returned.
// Authentication uses the default Azure credential chain (env vars → Azure CLI → Managed Identity).
func ListInstances(ctx context.Context, subscriptionID, resourceGroup string) ([]InstanceSummary, error) {
	cred, err := azidentity.NewDefaultAzureCredential(nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create Azure credential: %w", err)
	}

	vmClient, err := armcompute.NewVirtualMachinesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create VM client: %w", err)
	}

	nicClient, err := armnetwork.NewInterfacesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create NIC client: %w", err)
	}

	pubIPClient, err := armnetwork.NewPublicIPAddressesClient(subscriptionID, cred, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create public IP client: %w", err)
	}

	vms, err := fetchVMs(ctx, vmClient, resourceGroup)
	if err != nil {
		return nil, err
	}

	instances := make([]InstanceSummary, 0, len(vms))
	for _, vm := range vms {
		instances = append(instances, buildSummary(ctx, vm, nicClient, pubIPClient))
	}
	return instances, nil
}

// fetchVMs retrieves all VMs in the subscription or a single resource group.
// The Azure SDK requires different expand type constants for NewListPager (by resource group)
// vs NewListAllPager (subscription-wide), so each branch declares its own expand variable.
func fetchVMs(ctx context.Context, client *armcompute.VirtualMachinesClient, resourceGroup string) ([]*armcompute.VirtualMachine, error) {
	var vms []*armcompute.VirtualMachine

	if resourceGroup != "" {
		expand := armcompute.ExpandTypeForListVMsInstanceView
		pager := client.NewListPager(resourceGroup, &armcompute.VirtualMachinesClientListOptions{
			Expand: &expand,
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list VMs in resource group %q: %w", resourceGroup, err)
			}
			vms = append(vms, page.Value...)
		}
	} else {
		expand := armcompute.ExpandTypesForListVMsInstanceView
		pager := client.NewListAllPager(&armcompute.VirtualMachinesClientListAllOptions{
			Expand: &expand,
		})
		for pager.More() {
			page, err := pager.NextPage(ctx)
			if err != nil {
				return nil, fmt.Errorf("failed to list all VMs: %w", err)
			}
			vms = append(vms, page.Value...)
		}
	}

	return vms, nil
}

// buildSummary constructs an InstanceSummary from an Azure VM, resolving
// its power state from the instance view and its IPs via NIC lookup.
func buildSummary(ctx context.Context, vm *armcompute.VirtualMachine,
	nicClient *armnetwork.InterfacesClient,
	pubIPClient *armnetwork.PublicIPAddressesClient,
) InstanceSummary {
	s := InstanceSummary{
		Name:          ptrStr(vm.Name),
		Location:      ptrStr(vm.Location),
		ResourceGroup: parseResourceGroup(ptrStr(vm.ID)),
		PrivateIP:     "-",
		PublicIP:      "-",
		PowerState:    "-",
	}

	if p := vm.Properties; p != nil {
		s.State = ptrStr(p.ProvisioningState)

		if p.HardwareProfile != nil && p.HardwareProfile.VMSize != nil {
			s.Type = string(*p.HardwareProfile.VMSize)
		}
		if p.StorageProfile != nil && p.StorageProfile.OSDisk != nil && p.StorageProfile.OSDisk.OSType != nil {
			s.OSType = string(*p.StorageProfile.OSDisk.OSType)
		}
		if p.InstanceView != nil {
			s.PowerState = extractPowerState(p.InstanceView.Statuses)
		}

		if p.NetworkProfile != nil && len(p.NetworkProfile.NetworkInterfaces) > 0 {
			nicID := ptrStr(p.NetworkProfile.NetworkInterfaces[0].ID)
			s.PrivateIP, s.PublicIP = resolveNICIPs(ctx, nicID, nicClient, pubIPClient)
		}
	}

	return s
}

// resolveNICIPs fetches private and public IPs for a NIC resource ID.
// Two separate API calls are needed: one for the NIC (for PrivateIPAddress and
// the public IP resource reference), and one for the public IP resource itself.
func resolveNICIPs(ctx context.Context, nicID string,
	nicClient *armnetwork.InterfacesClient,
	pubIPClient *armnetwork.PublicIPAddressesClient,
) (privateIP, publicIP string) {
	privateIP, publicIP = "-", "-"

	rg, name := parseRGAndName(nicID)
	if rg == "" || name == "" {
		return
	}

	resp, err := nicClient.Get(ctx, rg, name, nil)
	if err != nil || resp.Properties == nil || len(resp.Properties.IPConfigurations) == 0 {
		return
	}

	cfg := resp.Properties.IPConfigurations[0]
	if cfg.Properties == nil {
		return
	}

	if cfg.Properties.PrivateIPAddress != nil {
		privateIP = *cfg.Properties.PrivateIPAddress
	}

	if cfg.Properties.PublicIPAddress != nil {
		pubRG, pubName := parseRGAndName(ptrStr(cfg.Properties.PublicIPAddress.ID))
		if pubRG != "" && pubName != "" {
			pubResp, err := pubIPClient.Get(ctx, pubRG, pubName, nil)
			if err == nil && pubResp.Properties != nil && pubResp.Properties.IPAddress != nil {
				publicIP = *pubResp.Properties.IPAddress
			}
		}
	}

	return
}

// extractPowerState scans instance view statuses for the "PowerState/" prefix
// and returns the suffix (e.g. "running", "deallocated", "stopped").
func extractPowerState(statuses []*armcompute.InstanceViewStatus) string {
	for _, s := range statuses {
		if s.Code != nil && strings.HasPrefix(*s.Code, "PowerState/") {
			return strings.TrimPrefix(*s.Code, "PowerState/")
		}
	}
	return "-"
}

// parseResourceGroup extracts the resource group name from an Azure resource ID.
func parseResourceGroup(id string) string {
	rg, _ := parseRGAndName(id)
	return rg
}

// parseRGAndName extracts (resourceGroup, resourceName) from an Azure resource ID.
// Azure resource ID: /subscriptions/{sub}/resourceGroups/{rg}/providers/{ns}/{type}/{name}
func parseRGAndName(id string) (rg, name string) {
	if id == "" {
		return "", ""
	}
	parts := strings.Split(id, "/")
	for i, p := range parts {
		if strings.EqualFold(p, "resourceGroups") && i+1 < len(parts) {
			rg = parts[i+1]
		}
	}
	for i := len(parts) - 1; i >= 0; i-- {
		if parts[i] != "" {
			name = parts[i]
			break
		}
	}
	return
}

func ptrStr(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
