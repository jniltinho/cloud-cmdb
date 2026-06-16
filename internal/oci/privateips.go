package oci

import (
	"context"
	"fmt"
	"time"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/core"
)

// ListPrivateIPsOptions holds all parameters for listing private IPs.
type ListPrivateIPsOptions struct {
	SubnetID   string
	Delay      time.Duration // delay between pagination pages
	ConfigFile string        // path to OCI config file (optional)
	Profile    string        // profile name (optional)
}

// ListPrivateIPs retrieves all Private IPs in a subnet using the OCI SDK.
// It handles full pagination and applies a configurable delay between pages
// to avoid rate limiting (429 TooManyRequests).
func ListPrivateIPs(ctx context.Context, opts ListPrivateIPsOptions) ([]core.PrivateIp, error) {
	if opts.SubnetID == "" {
		return nil, fmt.Errorf("subnet-id is required")
	}

	// OCI Configuration
	var configProvider common.ConfigurationProvider
	if opts.ConfigFile != "" || opts.Profile != "" {
		configProvider = common.CustomProfileConfigProvider(opts.ConfigFile, opts.Profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirtualNetwork client: %w", err)
	}

	// Enable automatic SDK retry for 429 TooManyRequests.
	retryPolicy := common.DefaultRetryPolicy()
	client.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Request
	request := core.ListPrivateIpsRequest{
		SubnetId: &opts.SubnetID,
	}

	var allPrivateIPs []core.PrivateIp

	for {
		response, err := client.ListPrivateIps(ctx, request)
		if err != nil {
			return nil, fmt.Errorf("failed to list Private IPs (after SDK retries): %w", err)
		}

		allPrivateIPs = append(allPrivateIPs, response.Items...)

		if response.OpcNextPage == nil {
			break
		}

		// Controlled delay between pages to avoid rate limits on large subnets.
		if opts.Delay > 0 {
			time.Sleep(opts.Delay)
		}

		request.Page = response.OpcNextPage
	}

	return allPrivateIPs, nil
}

// SubnetSummary holds minimal subnet information for --sum output (CIDR range).
type SubnetSummary struct {
	CIDR string `json:"cidr,omitempty"`
}

// GetSubnetSummary fetches the CIDR block (range) for a given subnet ID.
// Used by the list-private-ips --sum flag to report the IP range + allocation count.
func GetSubnetSummary(ctx context.Context, subnetID, configFile, profile string) (*SubnetSummary, error) {
	if subnetID == "" {
		return nil, fmt.Errorf("subnet-id is required")
	}

	// OCI Configuration (same pattern as ListPrivateIPs)
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	client, err := core.NewVirtualNetworkClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create VirtualNetwork client: %w", err)
	}

	// Enable automatic SDK retry for 429 TooManyRequests.
	retryPolicy := common.DefaultRetryPolicy()
	client.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	resp, err := client.GetSubnet(ctx, core.GetSubnetRequest{
		SubnetId: &subnetID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get subnet %s: %w", subnetID, err)
	}

	summary := &SubnetSummary{}
	if resp.Subnet.CidrBlock != nil {
		summary.CIDR = *resp.Subnet.CidrBlock
	}
	return summary, nil
}
