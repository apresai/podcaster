// Backfill GSI2 (global podcast index) and fix GSI1 (per-user podcast index)
// for all existing PODCAST# items in DynamoDB.
//
// Usage:
//
//	go run ./scripts/backfill-gsi2 --dry-run          # preview changes
//	go run ./scripts/backfill-gsi2                     # apply changes
//	go run ./scripts/backfill-gsi2 --table my-table    # custom table name
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var ulidRe = regexp.MustCompile(`^[0-9A-HJKMNP-TV-Z]{26}$`)

func main() {
	tableName := flag.String("table", "podcaster-prod", "DynamoDB table name")
	dryRun := flag.Bool("dry-run", false, "Preview changes without writing")
	flag.Parse()

	ctx := context.Background()
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion("us-east-1"))
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}
	client := dynamodb.NewFromConfig(cfg)

	fmt.Printf("Table: %s | Dry run: %v\n", *tableName, *dryRun)

	var lastKey map[string]types.AttributeValue
	var scanned, updated, skipped int

	for {
		input := &dynamodb.ScanInput{
			TableName:        tableName,
			FilterExpression: aws.String("begins_with(PK, :prefix)"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":prefix": &types.AttributeValueMemberS{Value: "PODCAST#"},
			},
		}
		if lastKey != nil {
			input.ExclusiveStartKey = lastKey
		}

		result, err := client.Scan(ctx, input)
		if err != nil {
			log.Fatalf("scan: %v", err)
		}

		for _, item := range result.Items {
			scanned++
			pk := attrStr(item, "PK")
			gsi1sk := attrStr(item, "GSI1SK")
			existingGSI2PK := attrStr(item, "GSI2PK")

			// Already backfilled?
			if existingGSI2PK == "PODCASTS" {
				skipped++
				continue
			}

			// Determine user ID for per-user GSI1PK
			userID := attrStr(item, "userId")
			if userID == "" {
				// Check if owner looks like a ULID (authenticated user)
				owner := attrStr(item, "owner")
				if ulidRe.MatchString(owner) {
					userID = owner
				}
			}

			// Build new GSI1PK
			newGSI1PK := "PODCASTS" // anonymous fallback
			if userID != "" {
				newGSI1PK = "USER#" + userID + "#PODCASTS"
			}

			// GSI2 sort key reuses existing GSI1SK
			gsi2sk := gsi1sk

			podcastID := strings.TrimPrefix(pk, "PODCAST#")
			action := "UPDATE"
			if *dryRun {
				action = "DRY-RUN"
			}
			fmt.Printf("[%s] %s: GSI1PK=%s GSI2PK=PODCASTS GSI2SK=%s\n", action, podcastID, newGSI1PK, gsi2sk)

			if !*dryRun {
				_, err := client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
					TableName: tableName,
					Key: map[string]types.AttributeValue{
						"PK": &types.AttributeValueMemberS{Value: pk},
						"SK": &types.AttributeValueMemberS{Value: "METADATA"},
					},
					UpdateExpression: aws.String("SET GSI1PK = :g1pk, GSI2PK = :g2pk, GSI2SK = :g2sk"),
					ExpressionAttributeValues: map[string]types.AttributeValue{
						":g1pk": &types.AttributeValueMemberS{Value: newGSI1PK},
						":g2pk": &types.AttributeValueMemberS{Value: "PODCASTS"},
						":g2sk": &types.AttributeValueMemberS{Value: gsi2sk},
					},
				})
				if err != nil {
					log.Printf("ERROR updating %s: %v", podcastID, err)
					continue
				}
				updated++
			} else {
				updated++
			}
		}

		lastKey = result.LastEvaluatedKey
		if lastKey == nil {
			break
		}
	}

	fmt.Printf("\nDone. Scanned: %d, Updated: %d, Skipped (already backfilled): %d\n", scanned, updated, skipped)
	if *dryRun {
		fmt.Println("(dry run — no changes written)")
		os.Exit(0)
	}
}

func attrStr(item map[string]types.AttributeValue, key string) string {
	if v, ok := item[key].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return ""
}
