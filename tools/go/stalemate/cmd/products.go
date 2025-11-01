package cmd

import (
	"context"
	"fmt"
	"sort"
	"time"

	"chainguard.dev/wolfi-vm/tools/go/stalemate/awspub"
	"chainguard.dev/wolfi-vm/tools/go/stalemate/marketplace"
)

// ProductsData holds products categorized by their status
type ProductsData struct {
	Unpublished []ProductInfo `json:"unpublished"`
	Published   []ProductInfo `json:"published"`
	Stale       []ProductInfo `json:"stale"`
}

// ProductInfo represents a product enriched with information from AWS Marketplace
type ProductInfo struct {
	Name          string     `json:"name"`
	Architecture  string     `json:"arch"`
	EntityID      string     `json:"entity_id,omitempty"`
	VersionTitle  string     `json:"version_title,omitempty"`
	LastPublished *time.Time `json:"last_published,omitempty"`
	DaysAgo       int        `json:"days_ago,omitempty"`
}

// FindStaleProducts analyzes the product map for stale products using the Marketplace API
func FindStaleProducts(productMap awspub.ProductMap, client *marketplace.Client, staleAfterDays int) (*ProductsData, error) {
	data := &ProductsData{
		Published:   []ProductInfo{},
		Unpublished: []ProductInfo{},
		Stale:       []ProductInfo{},
	}

	ctx := context.Background()

	var productNames []string
	for name := range productMap {
		productNames = append(productNames, name)
	}
	sort.Strings(productNames)

	for _, name := range productNames {
		for arch, product := range productMap[name] {
			pi := ProductInfo{
				Name:         name,
				Architecture: arch,
			}

			if product.EntityID == "" {
				data.Unpublished = append(data.Unpublished, pi)
				continue
			}

			pi.EntityID = product.EntityID

			version, err := client.GetLatestVersion(ctx, pi.EntityID)
			if err != nil {
				return nil, fmt.Errorf("unable to get latest product version: %w", err)
			}

			pi.VersionTitle = version.VersionTitle
			pi.LastPublished = &version.CreatedDate
			pi.DaysAgo = int(time.Since(*pi.LastPublished).Hours() / 24)

			if staleAfterDays > 0 && pi.DaysAgo > staleAfterDays {
				data.Stale = append(data.Stale, pi)
			} else {
				data.Published = append(data.Published, pi)
			}
		}
	}

	return data, nil
}
