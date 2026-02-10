package mcpserver

import (
	"context"
	"crypto/rand"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
	"github.com/oklog/ulid/v2"
)

// JobStatus represents the state of a podcast generation job.
type JobStatus string

const (
	JobStatusSubmitted    JobStatus = "submitted"
	JobStatusIngesting    JobStatus = "ingesting"
	JobStatusScripting    JobStatus = "scripting"
	JobStatusSynthesizing JobStatus = "synthesizing"
	JobStatusAssembling   JobStatus = "assembling"
	JobStatusUploading    JobStatus = "uploading"
	JobStatusComplete     JobStatus = "complete"
	JobStatusFailed       JobStatus = "failed"
)

// PodcastItem is the DynamoDB record for a podcast.
type PodcastItem struct {
	PK              string  `dynamodbav:"PK"`
	SK              string  `dynamodbav:"SK"`
	GSI1PK          string  `dynamodbav:"GSI1PK"`
	GSI1SK          string  `dynamodbav:"GSI1SK"`
	PodcastID       string  `dynamodbav:"podcastId"`
	Title           string  `dynamodbav:"title,omitempty"`
	Summary         string  `dynamodbav:"summary,omitempty"`
	Owner           string  `dynamodbav:"owner"`
	AudioKey        string  `dynamodbav:"audioKey,omitempty"`
	AudioURL        string  `dynamodbav:"audioUrl,omitempty"`
	Duration        string  `dynamodbav:"duration,omitempty"`
	FileSizeMB      float64 `dynamodbav:"fileSizeMB,omitempty"`
	SourceURL       string  `dynamodbav:"sourceUrl,omitempty"`
	Status          string  `dynamodbav:"status"`
	ProgressPercent float64 `dynamodbav:"progressPercent,omitempty"`
	StageMessage    string  `dynamodbav:"stageMessage,omitempty"`
	ErrorMessage    string  `dynamodbav:"errorMessage,omitempty"`
	Model           string  `dynamodbav:"model,omitempty"`
	TTSProvider     string  `dynamodbav:"ttsProvider,omitempty"`
	Format          string  `dynamodbav:"format,omitempty"`
	PlayCount       int     `dynamodbav:"playCount,omitempty"`
	ScriptJSON      string  `dynamodbav:"scriptJson,omitempty"`
	ScriptKey       string  `dynamodbav:"scriptKey,omitempty"`
	ScriptURL       string  `dynamodbav:"scriptUrl,omitempty"`
	CreatedAt       string  `dynamodbav:"createdAt"`

	// Usage tracking fields (set after pipeline completion)
	UserID           string  `dynamodbav:"userId,omitempty"`
	InputCharCount   int     `dynamodbav:"inputCharCount,omitempty"`
	OutputDurationSec int    `dynamodbav:"outputDurationSec,omitempty"`
	TTSCharCount     int     `dynamodbav:"ttsCharCount,omitempty"`
	EstimatedCostUSD float64 `dynamodbav:"estimatedCostUSD,omitempty"`
}

// Store handles DynamoDB operations for podcast jobs.
type Store struct {
	client    *dynamodb.Client
	tableName string
}

// NewStore creates a DynamoDB store.
func NewStore(client *dynamodb.Client, tableName string) *Store {
	return &Store{client: client, tableName: tableName}
}

// NewPodcastID generates a ULID for a new podcast.
func NewPodcastID() (string, error) {
	id, err := ulid.New(ulid.Timestamp(time.Now()), rand.Reader)
	if err != nil {
		return "", fmt.Errorf("generate ulid: %w", err)
	}
	return id.String(), nil
}

// CreateJob inserts a new podcast job with status=submitted.
func (s *Store) CreateJob(ctx context.Context, id, owner, sourceURL, model, ttsProvider, format string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	item := PodcastItem{
		PK:          "PODCAST#" + id,
		SK:          "METADATA",
		GSI1PK:      "PODCASTS",
		GSI1SK:      now + "#" + id,
		PodcastID:   id,
		Owner:       owner,
		SourceURL:   sourceURL,
		Status:      string(JobStatusSubmitted),
		Model:       model,
		TTSProvider: ttsProvider,
		Format:      format,
		CreatedAt:   now,
	}

	av, err := attributevalue.MarshalMap(item)
	if err != nil {
		return fmt.Errorf("marshal job item: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("put job item: %w", err)
	}
	return nil
}

// UpdateProgress updates the job's status, progress percent, and stage message.
func (s *Store) UpdateProgress(ctx context.Context, id string, status JobStatus, percent float64, message string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + id},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String("SET #status = :status, progressPercent = :pct, stageMessage = :msg"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(status)},
			":pct":    &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", percent)},
			":msg":    &types.AttributeValueMemberS{Value: message},
		},
	})
	if err != nil {
		return fmt.Errorf("update progress: %w", err)
	}
	return nil
}

// CompleteJob marks the job as complete with final metadata.
func (s *Store) CompleteJob(ctx context.Context, id, title, summary, audioKey, audioURL, duration, scriptJSON, scriptKey, scriptURL string, fileSizeMB float64) error {
	updateExpr := "SET #status = :status, progressPercent = :pct, stageMessage = :msg, title = :title, summary = :summary, audioKey = :akey, audioUrl = :aurl, #dur = :dur, fileSizeMB = :sz, scriptJson = :sj"
	exprValues := map[string]types.AttributeValue{
		":status":  &types.AttributeValueMemberS{Value: string(JobStatusComplete)},
		":pct":     &types.AttributeValueMemberN{Value: "1.00"},
		":msg":     &types.AttributeValueMemberS{Value: "Complete"},
		":title":   &types.AttributeValueMemberS{Value: title},
		":summary": &types.AttributeValueMemberS{Value: summary},
		":akey":    &types.AttributeValueMemberS{Value: audioKey},
		":aurl":    &types.AttributeValueMemberS{Value: audioURL},
		":dur":     &types.AttributeValueMemberS{Value: duration},
		":sz":      &types.AttributeValueMemberN{Value: fmt.Sprintf("%.2f", fileSizeMB)},
		":sj":      &types.AttributeValueMemberS{Value: scriptJSON},
	}

	if scriptKey != "" {
		updateExpr += ", scriptKey = :skey"
		exprValues[":skey"] = &types.AttributeValueMemberS{Value: scriptKey}
	}
	if scriptURL != "" {
		updateExpr += ", scriptUrl = :surl"
		exprValues[":surl"] = &types.AttributeValueMemberS{Value: scriptURL}
	}

	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + id},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String(updateExpr),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
			"#dur":    "duration",
		},
		ExpressionAttributeValues: exprValues,
	})
	if err != nil {
		return fmt.Errorf("complete job: %w", err)
	}
	return nil
}

// FailJob marks the job as failed with an error message.
func (s *Store) FailJob(ctx context.Context, id, errMsg string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + id},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String("SET #status = :status, errorMessage = :err, stageMessage = :msg"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: string(JobStatusFailed)},
			":err":    &types.AttributeValueMemberS{Value: errMsg},
			":msg":    &types.AttributeValueMemberS{Value: "Failed: " + errMsg},
		},
	})
	if err != nil {
		return fmt.Errorf("fail job: %w", err)
	}
	return nil
}

// GetPodcast retrieves a single podcast by ID.
func (s *Store) GetPodcast(ctx context.Context, id string) (*PodcastItem, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + id},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get podcast: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}

	var item PodcastItem
	if err := attributevalue.UnmarshalMap(result.Item, &item); err != nil {
		return nil, fmt.Errorf("unmarshal podcast: %w", err)
	}
	return &item, nil
}

// ListPodcasts returns podcasts ordered by creation time (newest first) via GSI1.
func (s *Store) ListPodcasts(ctx context.Context, limit int, cursor string) ([]PodcastItem, string, error) {
	if limit <= 0 {
		limit = 20
	}

	input := &dynamodb.QueryInput{
		TableName:              &s.tableName,
		IndexName:              aws.String("GSI1"),
		KeyConditionExpression: aws.String("GSI1PK = :pk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":pk": &types.AttributeValueMemberS{Value: "PODCASTS"},
		},
		ScanIndexForward: aws.Bool(false),
		Limit:            aws.Int32(int32(limit)),
	}

	if cursor != "" {
		// cursor is the full GSI1SK value ({timestamp}#{id})
		// Extract the podcast ID from the cursor to reconstruct PK
		parts := strings.SplitN(cursor, "#", 2)
		if len(parts) != 2 {
			return nil, "", fmt.Errorf("invalid cursor format")
		}
		podcastID := parts[1]
		input.ExclusiveStartKey = map[string]types.AttributeValue{
			"PK":     &types.AttributeValueMemberS{Value: "PODCAST#" + podcastID},
			"SK":     &types.AttributeValueMemberS{Value: "METADATA"},
			"GSI1PK": &types.AttributeValueMemberS{Value: "PODCASTS"},
			"GSI1SK": &types.AttributeValueMemberS{Value: cursor},
		}
	}

	result, err := s.client.Query(ctx, input)
	if err != nil {
		return nil, "", fmt.Errorf("list podcasts: %w", err)
	}

	var items []PodcastItem
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &items); err != nil {
		return nil, "", fmt.Errorf("unmarshal podcast list: %w", err)
	}

	var nextCursor string
	if result.LastEvaluatedKey != nil {
		if gsi1sk, ok := result.LastEvaluatedKey["GSI1SK"].(*types.AttributeValueMemberS); ok {
			nextCursor = gsi1sk.Value
		}
	}

	return items, nextCursor, nil
}
