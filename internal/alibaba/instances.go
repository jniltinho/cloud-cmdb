// Package alibaba provides functions for querying Alibaba Cloud ECS resources
// using the alibabacloud-go SDK with Access Key credentials.
package alibaba

import (
	"context"
	"fmt"
	"os"

	openapi "github.com/alibabacloud-go/darabonba-openapi/v2/client"
	ecs "github.com/alibabacloud-go/ecs-20140526/v4/client"
	"github.com/alibabacloud-go/tea/tea"
)

// InstanceSummary is the flat representation of an Alibaba Cloud ECS instance
// used by list-instances. Optional fields are empty strings when absent;
// callers render them as "-" for display.
type InstanceSummary struct {
	Name       string
	InstanceID string
	Type       string
	Status     string
	PrivateIP  string
	PublicIP   string
	OS         string
	Zone       string
	Region     string
}

// GetRegion returns the region from the flag value, then ALIBABA_CLOUD_REGION
// env var, falling back to "cn-hangzhou" as the default.
func GetRegion(flagValue string) string {
	if flagValue != "" {
		return flagValue
	}
	if v := os.Getenv("ALIBABA_CLOUD_REGION"); v != "" {
		return v
	}
	return "cn-hangzhou"
}

// GetCredentials resolves the Access Key ID and Secret from flags first,
// then from ALIBABA_CLOUD_ACCESS_KEY_ID / ALIBABA_CLOUD_ACCESS_KEY_SECRET
// environment variables. Returns an error if either value is missing.
func GetCredentials(accessKeyID, accessKeySecret string) (string, string, error) {
	id := accessKeyID
	if id == "" {
		id = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_ID")
	}
	secret := accessKeySecret
	if secret == "" {
		secret = os.Getenv("ALIBABA_CLOUD_ACCESS_KEY_SECRET")
	}
	if id == "" || secret == "" {
		return "", "", fmt.Errorf(
			"credentials required: use --access-key-id/--access-key-secret or set " +
				"ALIBABA_CLOUD_ACCESS_KEY_ID/ALIBABA_CLOUD_ACCESS_KEY_SECRET",
		)
	}
	return id, secret, nil
}

// ListInstances returns all non-deleted ECS instances in the given region.
// It paginates automatically using DescribeInstances (page size 100).
// ctx is accepted for API consistency with other providers but is not forwarded
// to the SDK (the alibabacloud-go SDK does not accept context).
func ListInstances(_ context.Context, region, accessKeyID, accessKeySecret string) ([]InstanceSummary, error) {
	id, secret, err := GetCredentials(accessKeyID, accessKeySecret)
	if err != nil {
		return nil, err
	}

	cfg := &openapi.Config{
		AccessKeyId:     tea.String(id),
		AccessKeySecret: tea.String(secret),
		RegionId:        tea.String(region),
	}
	client, err := ecs.NewClient(cfg)
	if err != nil {
		return nil, fmt.Errorf("failed to create ECS client: %w", err)
	}

	var results []InstanceSummary
	pageNumber := int32(1)
	const pageSize = int32(100)

	for {
		req := &ecs.DescribeInstancesRequest{
			RegionId:   tea.String(region),
			PageNumber: tea.Int32(pageNumber),
			PageSize:   tea.Int32(pageSize),
		}

		resp, err := client.DescribeInstances(req)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances (page %d): %w", pageNumber, err)
		}
		if resp.Body == nil || resp.Body.Instances == nil {
			break
		}

		for _, inst := range resp.Body.Instances.Instance {
			results = append(results, toSummary(inst, region))
		}

		total := tea.Int32Value(resp.Body.TotalCount)
		fetched := (pageNumber-1)*pageSize + int32(len(resp.Body.Instances.Instance))
		if fetched >= total {
			break
		}
		pageNumber++
	}

	return results, nil
}

func toSummary(inst *ecs.DescribeInstancesResponseBodyInstancesInstance, region string) InstanceSummary {
	s := InstanceSummary{
		Name:       tea.StringValue(inst.InstanceName),
		InstanceID: tea.StringValue(inst.InstanceId),
		Type:       tea.StringValue(inst.InstanceType),
		Status:     tea.StringValue(inst.Status),
		OS:         tea.StringValue(inst.OSName),
		Zone:       tea.StringValue(inst.ZoneId),
		Region:     region,
		PrivateIP:  "-",
		PublicIP:   "-",
	}

	// Private IP — VPC instances expose it via VpcAttributes
	if inst.VpcAttributes != nil && inst.VpcAttributes.PrivateIpAddress != nil {
		if ips := inst.VpcAttributes.PrivateIpAddress.IpAddress; len(ips) > 0 && ips[0] != nil {
			s.PrivateIP = *ips[0]
		}
	}
	// Fallback: classic-network InnerIpAddress
	if s.PrivateIP == "-" && inst.InnerIpAddress != nil {
		if ips := inst.InnerIpAddress.IpAddress; len(ips) > 0 && ips[0] != nil {
			s.PrivateIP = *ips[0]
		}
	}

	// Public IP — standard assignment
	if inst.PublicIpAddress != nil {
		if ips := inst.PublicIpAddress.IpAddress; len(ips) > 0 && ips[0] != nil {
			s.PublicIP = *ips[0]
		}
	}
	// Fallback: Elastic IP (EIP)
	if s.PublicIP == "-" && inst.EipAddress != nil && tea.StringValue(inst.EipAddress.IpAddress) != "" {
		s.PublicIP = tea.StringValue(inst.EipAddress.IpAddress)
	}

	return s
}
