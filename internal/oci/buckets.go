package oci

import (
	"context"
	"fmt"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/objectstorage"
)

// BucketEntry is a minimal view of a bucket for display (name, namespace, compartment name, created time).
type BucketEntry struct {
	Name            string `json:"name"`
	Namespace       string `json:"namespace"`
	CompartmentName string `json:"compartment_name"`
	Created         string `json:"created"`
}

// ListBuckets lists buckets in OCI Object Storage.
//   - If compartmentID is empty: enumerates ALL compartments (incl. root tenancy)
//     and returns buckets from every compartment (using a single namespace).
//   - If compartmentID is provided (name or OCID): lists only buckets in that compartment.
//
// The Object Storage namespace is retrieved once via GetNamespace and reused for
// all ListBuckets calls (namespace is tenancy-wide).
//
// CompartmentName is resolved via ListAllCompartments (human-friendly name).
//
// Uses the official ObjectStorage client + automatic SDK retries for rate limits.
func ListBuckets(ctx context.Context, compartmentID, configFile, profile string) ([]BucketEntry, error) {
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	osClient, err := objectstorage.NewObjectStorageClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create ObjectStorage client: %w", err)
	}

	// Enable automatic SDK retry for 429 TooManyRequests
	retryPolicy := common.DefaultRetryPolicy()
	osClient.SetCustomClientConfiguration(common.CustomClientConfiguration{
		RetryPolicy: &retryPolicy,
	})

	// Retrieve the Object Storage namespace once (required for ListBuckets).
	// This is the namespace for the configured tenancy/user.
	nsResp, err := osClient.GetNamespace(ctx, objectstorage.GetNamespaceRequest{})
	if err != nil {
		return nil, fmt.Errorf("failed to get Object Storage namespace: %w", err)
	}
	namespace := ""
	if nsResp.Value != nil {
		namespace = *nsResp.Value
	}
	if namespace == "" {
		return nil, fmt.Errorf("received empty Object Storage namespace from GetNamespace")
	}

	// Fetch all compartments upfront so we can show nice names instead of OCIDs
	comps, err := ListAllCompartments(ctx, configFile, profile)
	if err != nil {
		return nil, fmt.Errorf("failed to list compartments for bucket enumeration: %w", err)
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

	var results []BucketEntry

	for _, compID := range targetCompartments {
		compartmentName := nameByID[compID]
		if compartmentName == "" {
			compartmentName = compID // fallback
		}

		req := objectstorage.ListBucketsRequest{
			NamespaceName: &namespace,
			CompartmentId: &compID,
		}

		for {
			resp, err := osClient.ListBuckets(ctx, req)
			if err != nil {
				return nil, fmt.Errorf("failed to list buckets in compartment %s: %w", compID, err)
			}

			for _, b := range resp.Items {
				entry := BucketEntry{
					Namespace:       namespace,
					CompartmentName: compartmentName,
				}
				if b.Name != nil {
					entry.Name = *b.Name
				}
				if b.TimeCreated != nil {
					entry.Created = b.TimeCreated.Format("2006-01-02 15:04")
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
