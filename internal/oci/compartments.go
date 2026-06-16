package oci

import (
	"context"
	"fmt"
	"strings"

	"github.com/oracle/oci-go-sdk/v65/common"
	"github.com/oracle/oci-go-sdk/v65/identity"
)

// isCompartmentOCID returns true if the string looks like a compartment or tenancy OCID.
func isCompartmentOCID(s string) bool {
	return strings.HasPrefix(s, "ocid1.compartment.") || strings.HasPrefix(s, "ocid1.tenancy.")
}

// ResolveCompartmentID accepts either a compartment OCID or a display name.
// - If the value looks like an OCID (starts with ocid1.compartment. or ocid1.tenancy.), it is returned as-is.
// - Otherwise it performs a lookup by DisplayName (exact match) under the tenancy, including all nested compartments.
//
// The lookup uses the same config/profile logic as other functions in this package.
func ResolveCompartmentID(ctx context.Context, nameOrID, configFile, profile string) (string, error) {
	if nameOrID == "" {
		return "", fmt.Errorf("compartment is required")
	}
	if isCompartmentOCID(nameOrID) {
		return nameOrID, nil
	}

	// Build config provider (same pattern used everywhere)
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	idClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return "", fmt.Errorf("failed to create Identity client: %w", err)
	}

	tenancyOCID, err := configProvider.TenancyOCID()
	if err != nil {
		return "", fmt.Errorf("failed to get tenancy OCID from config: %w", err)
	}

	// List all compartments in the tenancy subtree (handles pagination)
	var all []identity.Compartment
	req := identity.ListCompartmentsRequest{
		CompartmentId:          &tenancyOCID,
		CompartmentIdInSubtree: common.Bool(true),
		AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
	}

	for {
		resp, err := idClient.ListCompartments(ctx, req)
		if err != nil {
			return "", fmt.Errorf("failed to list compartments: %w", err)
		}
		all = append(all, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		req.Page = resp.OpcNextPage
	}

	// Compartments in the Identity API use the "Name" field (this is the value users see and enter
	// in the OCI Console when creating a compartment). It must be unique within the parent compartment.
	var matches []identity.Compartment
	for _, c := range all {
		if c.Name != nil && *c.Name == nameOrID {
			matches = append(matches, c)
		}
	}

	if len(matches) == 0 {
		return "", fmt.Errorf("no compartment found with name %q (searched under tenancy)", nameOrID)
	}
	if len(matches) > 1 {
		return "", fmt.Errorf("multiple compartments found with name %q (ambiguous) — use the full OCID instead", nameOrID)
	}

	if matches[0].Id == nil {
		return "", fmt.Errorf("found compartment %q but it has no OCID", nameOrID)
	}
	return *matches[0].Id, nil
}

// CompartmentEntry represents a minimal compartment view with name and OCID.
type CompartmentEntry struct {
	Name string `json:"name"`
	OCID string `json:"ocid"`
}

// ListAllCompartments returns the tenancy (root) + all compartments in the tenancy subtree.
// Each entry contains only Name and OCID as requested.
func ListAllCompartments(ctx context.Context, configFile, profile string) ([]CompartmentEntry, error) {
	var configProvider common.ConfigurationProvider
	if configFile != "" || profile != "" {
		configProvider = common.CustomProfileConfigProvider(configFile, profile)
	} else {
		configProvider = common.DefaultConfigProvider()
	}

	idClient, err := identity.NewIdentityClientWithConfigurationProvider(configProvider)
	if err != nil {
		return nil, fmt.Errorf("failed to create Identity client: %w", err)
	}

	tenancyOCID, err := configProvider.TenancyOCID()
	if err != nil {
		return nil, fmt.Errorf("failed to get tenancy OCID from config: %w", err)
	}

	// Include the root tenancy itself (ListCompartments does not return the tenancy)
	tenancyResp, err := idClient.GetTenancy(ctx, identity.GetTenancyRequest{
		TenancyId: &tenancyOCID,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to get tenancy details: %w", err)
	}

	entries := []CompartmentEntry{}
	tenancyName := "tenancy"
	if tenancyResp.Name != nil && *tenancyResp.Name != "" {
		tenancyName = *tenancyResp.Name
	}
	entries = append(entries, CompartmentEntry{Name: tenancyName, OCID: tenancyOCID})

	// List all child compartments (handles pagination, includes nested via subtree=true)
	var all []identity.Compartment
	req := identity.ListCompartmentsRequest{
		CompartmentId:          &tenancyOCID,
		CompartmentIdInSubtree: common.Bool(true),
		AccessLevel:            identity.ListCompartmentsAccessLevelAccessible,
	}

	for {
		resp, err := idClient.ListCompartments(ctx, req)
		if err != nil {
			return nil, fmt.Errorf("failed to list compartments: %w", err)
		}
		all = append(all, resp.Items...)

		if resp.OpcNextPage == nil {
			break
		}
		req.Page = resp.OpcNextPage
	}

	for _, c := range all {
		name := ""
		if c.Name != nil {
			name = *c.Name
		}
		ocid := ""
		if c.Id != nil {
			ocid = *c.Id
		}
		if name != "" && ocid != "" {
			entries = append(entries, CompartmentEntry{Name: name, OCID: ocid})
		}
	}

	return entries, nil
}
