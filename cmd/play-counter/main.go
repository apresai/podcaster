package main

import (
	"bufio"
	"compress/gzip"
	"context"
	"fmt"
	"log"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/aws/aws-sdk-go-v2/service/s3"
)

// audioPathRegex matches GET /audio/{ULID}.mp3 requests with 200/206 status.
var audioPathRegex = regexp.MustCompile(`GET /audio/([A-Z0-9]{26})\.mp3`)

func main() {
	ctx := context.Background()

	cfg, err := awsconfig.LoadDefaultConfig(ctx)
	if err != nil {
		log.Fatalf("load aws config: %v", err)
	}

	tableName := os.Getenv("DYNAMODB_TABLE")
	logBucket := os.Getenv("LOG_BUCKET")
	logPrefix := os.Getenv("LOG_PREFIX")
	if tableName == "" || logBucket == "" {
		log.Fatal("DYNAMODB_TABLE and LOG_BUCKET environment variables are required")
	}
	if logPrefix == "" {
		logPrefix = "cf-logs/"
	}

	s3Client := s3.NewFromConfig(cfg)
	ddbClient := dynamodb.NewFromConfig(cfg)

	// Get last processed timestamp from DynamoDB
	lastProcessed := getLastProcessed(ctx, ddbClient, tableName)
	log.Printf("Last processed: %s", lastProcessed)

	// List new log files from S3
	paginator := s3.NewListObjectsV2Paginator(s3Client, &s3.ListObjectsV2Input{
		Bucket: &logBucket,
		Prefix: &logPrefix,
	})

	playCounts := make(map[string]int) // podcastID -> count
	var processedKeys []string

	for paginator.HasMorePages() {
		page, err := paginator.NextPage(ctx)
		if err != nil {
			log.Fatalf("list objects: %v", err)
		}

		for _, obj := range page.Contents {
			if obj.LastModified != nil && obj.LastModified.Format(time.RFC3339) <= lastProcessed {
				continue
			}

			counts, err := processLogFile(ctx, s3Client, logBucket, *obj.Key)
			if err != nil {
				log.Printf("process %s: %v", *obj.Key, err)
				continue
			}

			for id, count := range counts {
				playCounts[id] += count
			}
			processedKeys = append(processedKeys, *obj.Key)
		}
	}

	if len(playCounts) == 0 {
		log.Println("No new play counts to update")
		return
	}

	// Batch increment play counts in DynamoDB
	for podcastID, count := range playCounts {
		_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
			TableName: &tableName,
			Key: map[string]types.AttributeValue{
				"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + podcastID},
				"SK": &types.AttributeValueMemberS{Value: "METADATA"},
			},
			UpdateExpression: aws.String("ADD playCount :n"),
			ExpressionAttributeValues: map[string]types.AttributeValue{
				":n": &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", count)},
			},
		})
		if err != nil {
			log.Printf("update play count for %s: %v", podcastID, err)
			continue
		}
		log.Printf("Updated %s: +%d plays", podcastID, count)
	}

	// Update last processed timestamp
	setLastProcessed(ctx, ddbClient, tableName, time.Now().UTC().Format(time.RFC3339))
	log.Printf("Processed %d log files, updated %d podcasts", len(processedKeys), len(playCounts))
}

func processLogFile(ctx context.Context, client *s3.Client, bucket, key string) (map[string]int, error) {
	result, err := client.GetObject(ctx, &s3.GetObjectInput{
		Bucket: &bucket,
		Key:    &key,
	})
	if err != nil {
		return nil, fmt.Errorf("get object: %w", err)
	}
	defer result.Body.Close()

	var scanner *bufio.Scanner
	if strings.HasSuffix(key, ".gz") {
		gz, err := gzip.NewReader(result.Body)
		if err != nil {
			return nil, fmt.Errorf("gzip reader: %w", err)
		}
		defer gz.Close()
		scanner = bufio.NewScanner(gz)
	} else {
		scanner = bufio.NewScanner(result.Body)
	}

	counts := make(map[string]int)
	for scanner.Scan() {
		line := scanner.Text()
		if strings.HasPrefix(line, "#") {
			continue
		}

		// CloudFront log fields: date time x-edge-location sc-bytes c-ip cs-method cs-uri-stem sc-status ...
		fields := strings.Fields(line)
		if len(fields) < 8 {
			continue
		}

		status := fields[7]
		if status != "200" && status != "206" {
			continue
		}

		method := fields[5]
		path := fields[6]
		if method != "GET" {
			continue
		}

		matches := audioPathRegex.FindStringSubmatch("GET " + path)
		if len(matches) >= 2 {
			counts[matches[1]]++
		}
	}

	return counts, scanner.Err()
}

func getLastProcessed(ctx context.Context, client *dynamodb.Client, tableName string) string {
	result, err := client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "SYSTEM#PLAY_COUNTER"},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	})
	if err != nil || result.Item == nil {
		return "1970-01-01T00:00:00Z"
	}

	if v, ok := result.Item["lastProcessed"].(*types.AttributeValueMemberS); ok {
		return v.Value
	}
	return "1970-01-01T00:00:00Z"
}

func setLastProcessed(ctx context.Context, client *dynamodb.Client, tableName, ts string) {
	_, err := client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName: &tableName,
		Item: map[string]types.AttributeValue{
			"PK":            &types.AttributeValueMemberS{Value: "SYSTEM#PLAY_COUNTER"},
			"SK":            &types.AttributeValueMemberS{Value: "METADATA"},
			"lastProcessed": &types.AttributeValueMemberS{Value: ts},
		},
	})
	if err != nil {
		log.Printf("set last processed: %v", err)
	}
}
