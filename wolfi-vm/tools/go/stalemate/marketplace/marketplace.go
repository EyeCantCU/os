package marketplace

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/marketplacecatalog"
)

// Client wraps the AWS Marketplace Catalog client
type Client struct {
	client *marketplacecatalog.Client
}

// NewClient creates a new marketplace client
func NewClient(ctx context.Context) (*Client, error) {
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("unable to load AWS config: %w", err)
	}

	return &Client{
		client: marketplacecatalog.NewFromConfig(cfg),
	}, nil
}

// ProductVersion represents a version of a product
type ProductVersion struct {
	VersionTitle string
	CreatedDate  time.Time
}

// GetLatestVersion fetches the latest version information for a product
func (m *Client) GetLatestVersion(ctx context.Context, entityID string) (*ProductVersion, error) {
	catalog := "AWSMarketplace"
	describeResp, err := m.client.DescribeEntity(ctx, &marketplacecatalog.DescribeEntityInput{
		EntityId: &entityID,
		Catalog:  &catalog,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to describe entity %s: %w", entityID, err)
	}

	// Parse the Details field to extract version information
	// The Details field contains JSON data about the product
	var details map[string]interface{}
	if err := json.Unmarshal([]byte(*describeResp.Details), &details); err != nil {
		return nil, fmt.Errorf("failed to parse entity details: %w", err)
	}

	versions, err := extractVersions(details)
	if err != nil {
		return nil, fmt.Errorf("failed to extract versions: %w", err)
	}

	if len(versions) == 0 {
		return nil, fmt.Errorf("no versions found for entity %s", entityID)
	}

	// Sort versions by date and return the latest
	sort.Slice(versions, func(i, j int) bool {
		return versions[i].CreatedDate.After(versions[j].CreatedDate)
	})

	return &versions[0], nil
}

func extractVersions(details map[string]interface{}) ([]ProductVersion, error) {
	var versions []ProductVersion

	versionList, ok := details["Versions"].([]interface{})
	if !ok {
		return nil, fmt.Errorf("cannot find 'Versions' key in product details")
	}

	for _, v := range versionList {
		vMap, ok := v.(map[string]interface{})
		if !ok {
			return nil, fmt.Errorf("product version is not an object")
		}

		version := ProductVersion{}

		if title, ok := vMap["VersionTitle"].(string); ok {
			version.VersionTitle = title
		} else {
			return nil, fmt.Errorf("product version 'VersionTitle' is not a string")
		}

		if dateStr, ok := vMap["CreationDate"].(string); ok {
			if parsed, err := time.Parse(time.RFC3339, dateStr); err == nil {
				version.CreatedDate = parsed
			} else {
				return nil, fmt.Errorf(
					"product version 'CreationDate' is not a valid date, got: %v", dateStr)
			}
		} else {
			return nil, fmt.Errorf("product version 'CreationDate' is not a string")
		}

		versions = append(versions, version)
	}

	return versions, nil
}
