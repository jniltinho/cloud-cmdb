// Package aws provides functions for querying AWS EC2 resources
// using the aws-sdk-go-v2 default credential chain.
package aws

import (
	"context"
	"fmt"

	awssdk "github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ec2"
	"github.com/aws/aws-sdk-go-v2/service/ec2/types"
)

// InstanceSummary is the flat representation of an EC2 instance used by list-instances.
// Optional fields (PublicIP, Platform, etc.) are empty strings when not set;
// callers render them as "-" for display.
type InstanceSummary struct {
	Name       string
	InstanceID string
	Type       string
	State      string
	PrivateIP  string
	PublicIP   string
	Platform   string
	AZ         string
	VPC        string
	Subnet     string
	LaunchTime string
	Region     string
}

// loadConfig builds an aws.Config from the SDK default credential chain.
// region and profile override the corresponding SDK defaults when non-empty.
func loadConfig(ctx context.Context, region, profile string) (awssdk.Config, error) {
	opts := []func(*config.LoadOptions) error{}
	if region != "" {
		opts = append(opts, config.WithRegion(region))
	}
	if profile != "" {
		opts = append(opts, config.WithSharedConfigProfile(profile))
	}
	cfg, err := config.LoadDefaultConfig(ctx, opts...)
	if err != nil {
		return awssdk.Config{}, fmt.Errorf("failed to load AWS config: %w", err)
	}
	return cfg, nil
}

// ListInstances returns all non-terminated EC2 instances in the given region.
// If region is empty, the SDK default (env/config) is used.
func ListInstances(ctx context.Context, region, profile string) ([]InstanceSummary, error) {
	cfg, err := loadConfig(ctx, region, profile)
	if err != nil {
		return nil, err
	}

	client := ec2.NewFromConfig(cfg)

	var instances []InstanceSummary
	paginator := ec2.NewDescribeInstancesPaginator(client, &ec2.DescribeInstancesInput{})

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			return nil, fmt.Errorf("failed to describe instances: %w", err)
		}

		for _, reservation := range page.Reservations {
			for _, inst := range reservation.Instances {
				if inst.State != nil && inst.State.Name == types.InstanceStateNameTerminated {
					continue
				}

				summary := InstanceSummary{
					InstanceID: awssdk.ToString(inst.InstanceId),
					Type:       string(inst.InstanceType),
					Region:     cfg.Region,
				}

				if inst.State != nil {
					summary.State = string(inst.State.Name)
				}
				if inst.PrivateIpAddress != nil {
					summary.PrivateIP = *inst.PrivateIpAddress
				}
				if inst.PublicIpAddress != nil {
					summary.PublicIP = *inst.PublicIpAddress
				}
				if inst.Placement != nil {
					summary.AZ = awssdk.ToString(inst.Placement.AvailabilityZone)
				}
				if inst.VpcId != nil {
					summary.VPC = *inst.VpcId
				}
				if inst.SubnetId != nil {
					summary.Subnet = *inst.SubnetId
				}
				if inst.PlatformDetails != nil {
					summary.Platform = *inst.PlatformDetails
				}
				if inst.LaunchTime != nil {
					summary.LaunchTime = inst.LaunchTime.Format("2006-01-02")
				}

				for _, tag := range inst.Tags {
					if awssdk.ToString(tag.Key) == "Name" {
						summary.Name = awssdk.ToString(tag.Value)
						break
					}
				}

				instances = append(instances, summary)
			}
		}
	}

	return instances, nil
}
