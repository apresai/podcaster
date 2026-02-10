package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync/atomic"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

func main() {
	var (
		sourceTable = flag.String("source-table", "apresai-podcasts-prod", "Source DynamoDB table")
		destTable   = flag.String("dest-table", "podcaster-prod", "Destination DynamoDB table")
		dryRun      = flag.Bool("dry-run", false, "Scan and count but don't write")
		region      = flag.String("region", "us-east-1", "AWS region")
	)
	flag.Parse()

	logger := slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{
		Level: slog.LevelInfo,
	}))
	slog.SetDefault(logger)

	ctx := context.Background()

	// Load AWS config
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(*region))
	if err != nil {
		slog.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}

	ddbClient := dynamodb.NewFromConfig(cfg)

	if *dryRun {
		slog.Info("DRY RUN MODE - no writes will be performed")
	}

	slog.Info("Starting migration",
		"source", *sourceTable,
		"dest", *destTable,
		"region", *region,
	)

	// Counters
	var (
		totalScanned  atomic.Int64
		totalWritten  atomic.Int64
		totalRewrites atomic.Int64
	)

	// Scan parameters
	scanInput := &dynamodb.ScanInput{
		TableName: aws.String(*sourceTable),
	}

	var batch []types.WriteRequest
	const batchSize = 25

	// Paginate through all items
	paginator := dynamodb.NewScanPaginator(ddbClient, scanInput)

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			slog.Error("Scan failed", "error", err)
			os.Exit(1)
		}

		for _, item := range page.Items {
			totalScanned.Add(1)

			// Process item
			processedItem := processItem(item, &totalRewrites)

			if !*dryRun {
				// Add to batch
				batch = append(batch, types.WriteRequest{
					PutRequest: &types.PutRequest{
						Item: processedItem,
					},
				})

				// Write batch if full
				if len(batch) >= batchSize {
					if err := writeBatch(ctx, ddbClient, *destTable, batch); err != nil {
						slog.Error("Batch write failed", "error", err)
						os.Exit(1)
					}
					totalWritten.Add(int64(len(batch)))
					batch = batch[:0] // Clear batch
				}
			}

			// Log progress every 100 items
			if totalScanned.Load()%100 == 0 {
				slog.Info("Progress",
					"scanned", totalScanned.Load(),
					"written", totalWritten.Load(),
					"rewrites", totalRewrites.Load(),
				)
			}
		}
	}

	// Write remaining items in batch
	if !*dryRun && len(batch) > 0 {
		if err := writeBatch(ctx, ddbClient, *destTable, batch); err != nil {
			slog.Error("Final batch write failed", "error", err)
			os.Exit(1)
		}
		totalWritten.Add(int64(len(batch)))
	}

	// Final summary
	slog.Info("Migration complete",
		"total_scanned", totalScanned.Load(),
		"total_written", totalWritten.Load(),
		"total_rewrites", totalRewrites.Load(),
		"dry_run", *dryRun,
	)
}

// processItem processes a single DynamoDB item, rewriting audioUrl if needed
func processItem(item map[string]types.AttributeValue, rewriteCounter *atomic.Int64) map[string]types.AttributeValue {
	// Check if PK starts with "PODCAST#"
	pkAttr, ok := item["PK"]
	if !ok {
		return item
	}

	pkStr, ok := pkAttr.(*types.AttributeValueMemberS)
	if !ok || !strings.HasPrefix(pkStr.Value, "PODCAST#") {
		return item
	}

	// Check for audioUrl attribute
	audioUrlAttr, ok := item["audioUrl"]
	if !ok {
		return item
	}

	audioUrlStr, ok := audioUrlAttr.(*types.AttributeValueMemberS)
	if !ok {
		return item
	}

	// Rewrite URL: apresai.dev/audio/ → podcasts.apresai.dev/audio/
	// Skip URLs already pointing to podcasts.apresai.dev (idempotent)
	if strings.Contains(audioUrlStr.Value, "apresai.dev/audio/") &&
		!strings.Contains(audioUrlStr.Value, "podcasts.apresai.dev/audio/") {
		newUrl := strings.ReplaceAll(audioUrlStr.Value, "apresai.dev/audio/", "podcasts.apresai.dev/audio/")
		item["audioUrl"] = &types.AttributeValueMemberS{Value: newUrl}
		rewriteCounter.Add(1)
	}

	return item
}

// writeBatch writes a batch of items to the destination table
func writeBatch(ctx context.Context, client *dynamodb.Client, tableName string, batch []types.WriteRequest) error {
	if len(batch) == 0 {
		return nil
	}

	input := &dynamodb.BatchWriteItemInput{
		RequestItems: map[string][]types.WriteRequest{
			tableName: batch,
		},
	}

	result, err := client.BatchWriteItem(ctx, input)
	if err != nil {
		return fmt.Errorf("BatchWriteItem failed: %w", err)
	}

	// Handle unprocessed items (retry logic could be added here)
	if len(result.UnprocessedItems) > 0 {
		slog.Warn("Unprocessed items detected", "count", len(result.UnprocessedItems[tableName]))
		// Simple retry: just attempt once more
		retryInput := &dynamodb.BatchWriteItemInput{
			RequestItems: result.UnprocessedItems,
		}
		retryResult, err := client.BatchWriteItem(ctx, retryInput)
		if err != nil {
			return fmt.Errorf("Retry BatchWriteItem failed: %w", err)
		}
		if len(retryResult.UnprocessedItems) > 0 {
			return fmt.Errorf("Still have %d unprocessed items after retry", len(retryResult.UnprocessedItems[tableName]))
		}
	}

	return nil
}
