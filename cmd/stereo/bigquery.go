package main

import (
	"context"
	"fmt"
	"log"

	"cloud.google.com/go/bigquery"
	"google.golang.org/api/iterator"
)

const (
	customerAPKQuery = `
SELECT DISTINCT
  REGEXP_REPLACE(p.body.package, r'^(x86_64|aarch64)-', '') AS trimmed_package
FROM
  ` + "`prod-enforce-fabc.cloudevents_enforce_prod_rec.dev_chainguard_apk_pull_v1`" + ` as p
WHERE
  p.body.proxy_uidp IS NOT NULL
  AND DATE(_PARTITIONTIME) >= DATE_SUB(CURRENT_DATE(), INTERVAL 90 DAY)
  AND p.body.when >= DATETIME_SUB(CURRENT_DATETIME(), INTERVAL 90 DAY)
ORDER BY
  trimmed_package
`
)

// fetchCustomerAPKsFromBigQuery queries BigQuery for customer APK access data
// and returns a map of APK filenames that have been pulled in the last 90 days.
func fetchCustomerAPKsFromBigQuery(ctx context.Context) (map[string]bool, error) {
	log.Println("Querying BigQuery for customer APK access data...")

	// Create BigQuery client using Application Default Credentials
	client, err := bigquery.NewClient(ctx, "prod-enforce-fabc")
	if err != nil {
		return nil, fmt.Errorf("creating BigQuery client: %w", err)
	}
	defer client.Close()

	// Execute the query
	query := client.Query(customerAPKQuery)
	it, err := query.Read(ctx)
	if err != nil {
		return nil, fmt.Errorf("executing BigQuery query: %w", err)
	}

	// Parse results
	customerAPKs := make(map[string]bool)
	var row struct {
		TrimmedPackage string `bigquery:"trimmed_package"`
	}

	for {
		err := it.Next(&row)
		if err == iterator.Done {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("reading query results: %w", err)
		}

		if row.TrimmedPackage != "" {
			customerAPKs[row.TrimmedPackage] = true
		}
	}

	log.Printf("Retrieved %d customer APK entries from BigQuery", len(customerAPKs))
	return customerAPKs, nil
}
