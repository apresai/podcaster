package mcpserver

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

// authContextKey is the context key for auth results.
type authContextKey struct{}

// AuthResult holds the result of API key validation.
type AuthResult struct {
	Authenticated bool
	UserID        string
	Role          string // "admin" or "user"
	KeyID         string // key prefix for logging
	Error         error
}

// APIKeyRecord is the DynamoDB record for an API key.
type APIKeyRecord struct {
	PK         string `dynamodbav:"PK"`         // APIKEY#{prefix}
	SK         string `dynamodbav:"SK"`         // METADATA
	UserID     string `dynamodbav:"userId"`
	KeyHash    string `dynamodbav:"keyHash"`    // SHA-256 hex
	Name       string `dynamodbav:"name"`       // user-given name
	Status     string `dynamodbav:"status"`     // active, revoked
	CreatedAt  string `dynamodbav:"createdAt"`
	LastUsedAt string `dynamodbav:"lastUsedAt,omitempty"`
}

// UserRecord is the DynamoDB record for a user.
type UserRecord struct {
	PK         string `dynamodbav:"PK"`         // USER#{userId}
	SK         string `dynamodbav:"SK"`         // PROFILE
	Email      string `dynamodbav:"email"`
	Name       string `dynamodbav:"name"`
	Status     string `dynamodbav:"status"`     // pending, active, suspended
	Role       string `dynamodbav:"role"`       // admin, user
	CreatedAt  string `dynamodbav:"createdAt"`
	ApprovedAt string `dynamodbav:"approvedAt,omitempty"`
}

// UsageRecord is a monthly usage rollup per user.
type UsageRecord struct {
	PK               string  `dynamodbav:"PK"`               // USER#{userId}
	SK               string  `dynamodbav:"SK"`               // USAGE#{YYYY-MM}
	PodcastCount     int     `dynamodbav:"podcastCount"`
	TotalDurationSec int     `dynamodbav:"totalDurationSec"`
	TotalTTSChars    int     `dynamodbav:"totalTTSChars"`
	TotalCostUSD     float64 `dynamodbav:"totalCostUSD"`
}

// WithAuthResult stores the auth result in context.
func WithAuthResult(ctx context.Context, result AuthResult) context.Context {
	return context.WithValue(ctx, authContextKey{}, result)
}

// AuthFromContext retrieves the auth result from context.
func AuthFromContext(ctx context.Context) AuthResult {
	result, ok := ctx.Value(authContextKey{}).(AuthResult)
	if !ok {
		return AuthResult{Authenticated: false}
	}
	return result
}

// ValidateAPIKey checks a bearer token against DynamoDB.
// Returns the user info if valid, error if not.
func (s *Store) ValidateAPIKey(ctx context.Context, bearerToken string) (*AuthResult, error) {
	// Extract key from "Bearer <key>" format
	token := strings.TrimPrefix(bearerToken, "Bearer ")
	token = strings.TrimSpace(token)
	if token == "" {
		return nil, fmt.Errorf("empty API key")
	}

	// Derive prefix (first 8 chars of the key, after "pk_" prefix)
	if !strings.HasPrefix(token, "pk_") {
		return nil, fmt.Errorf("invalid API key format")
	}
	prefix := token[3:11] // 8 chars after "pk_"

	// SHA-256 hash the full token
	hash := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(hash[:])

	// Look up in DynamoDB
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "APIKEY#" + prefix},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("lookup API key: %w", err)
	}
	if result.Item == nil {
		return nil, fmt.Errorf("API key not found")
	}

	var record APIKeyRecord
	if err := attributevalue.UnmarshalMap(result.Item, &record); err != nil {
		return nil, fmt.Errorf("unmarshal API key: %w", err)
	}

	// Compare hashes
	if record.KeyHash != keyHash {
		return nil, fmt.Errorf("invalid API key")
	}

	// Check status
	if record.Status != "active" {
		return nil, fmt.Errorf("API key is %s", record.Status)
	}

	// Look up user to get role and check status
	userResult, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + record.UserID},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("lookup user: %w", err)
	}
	if userResult.Item == nil {
		return nil, fmt.Errorf("user not found for API key")
	}

	var user UserRecord
	if err := attributevalue.UnmarshalMap(userResult.Item, &user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}

	if user.Status != "active" {
		return nil, fmt.Errorf("user account is %s", user.Status)
	}

	// Update lastUsedAt (best-effort, don't fail auth if this errors)
	go s.updateKeyLastUsed(context.Background(), prefix)

	return &AuthResult{
		Authenticated: true,
		UserID:        record.UserID,
		Role:          user.Role,
		KeyID:         prefix,
	}, nil
}

// updateKeyLastUsed updates the lastUsedAt timestamp. Conditional to avoid hot writes.
func (s *Store) updateKeyLastUsed(ctx context.Context, prefix string) {
	now := time.Now().UTC().Format(time.RFC3339)
	oneMinuteAgo := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)

	s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "APIKEY#" + prefix},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression:    aws.String("SET lastUsedAt = :now"),
		ConditionExpression: aws.String("attribute_not_exists(lastUsedAt) OR lastUsedAt < :threshold"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":now":       &types.AttributeValueMemberS{Value: now},
			":threshold": &types.AttributeValueMemberS{Value: oneMinuteAgo},
		},
	})
}

// CreateAPIKey generates a new API key, stores its hash, and returns the plaintext (shown once).
func (s *Store) CreateAPIKey(ctx context.Context, userID, keyName string) (plaintext, prefix string, err error) {
	// Generate 32 random bytes
	raw := make([]byte, 32)
	if _, err := rand.Read(raw); err != nil {
		return "", "", fmt.Errorf("generate random bytes: %w", err)
	}

	// Build the key: pk_ + hex-encoded random bytes
	prefix = hex.EncodeToString(raw[:4]) // 8 hex chars
	plaintext = "pk_" + hex.EncodeToString(raw)

	// SHA-256 hash for storage
	hash := sha256.Sum256([]byte(plaintext))
	keyHash := hex.EncodeToString(hash[:])

	now := time.Now().UTC().Format(time.RFC3339)

	record := APIKeyRecord{
		PK:        "APIKEY#" + prefix,
		SK:        "METADATA",
		UserID:    userID,
		KeyHash:   keyHash,
		Name:      keyName,
		Status:    "active",
		CreatedAt: now,
	}

	av, err := attributevalue.MarshalMap(record)
	if err != nil {
		return "", "", fmt.Errorf("marshal API key: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		return "", "", fmt.Errorf("store API key: %w", err)
	}

	return plaintext, prefix, nil
}

// RevokeAPIKey marks an API key as revoked.
func (s *Store) RevokeAPIKey(ctx context.Context, prefix string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "APIKEY#" + prefix},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "revoked"},
		},
	})
	if err != nil {
		return fmt.Errorf("revoke API key: %w", err)
	}
	return nil
}

// ListAPIKeys returns all API keys for a user.
func (s *Store) ListAPIKeys(ctx context.Context, userID string) ([]APIKeyRecord, error) {
	// Scan for keys belonging to this user (small table, acceptable)
	result, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &s.tableName,
		FilterExpression: aws.String("begins_with(PK, :prefix) AND userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prefix": &types.AttributeValueMemberS{Value: "APIKEY#"},
			":uid":    &types.AttributeValueMemberS{Value: userID},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list API keys: %w", err)
	}

	var keys []APIKeyRecord
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &keys); err != nil {
		return nil, fmt.Errorf("unmarshal API keys: %w", err)
	}
	return keys, nil
}

// CreateUser creates a new user record with pending status.
func (s *Store) CreateUser(ctx context.Context, userID, email, name string) error {
	now := time.Now().UTC().Format(time.RFC3339)

	record := UserRecord{
		PK:        "USER#" + userID,
		SK:        "PROFILE",
		Email:     email,
		Name:      name,
		Status:    "pending",
		Role:      "user",
		CreatedAt: now,
	}

	av, err := attributevalue.MarshalMap(record)
	if err != nil {
		return fmt.Errorf("marshal user: %w", err)
	}

	_, err = s.client.PutItem(ctx, &dynamodb.PutItemInput{
		TableName:           &s.tableName,
		Item:                av,
		ConditionExpression: aws.String("attribute_not_exists(PK)"),
	})
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// GetUser retrieves a user by ID.
func (s *Store) GetUser(ctx context.Context, userID string) (*UserRecord, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get user: %w", err)
	}
	if result.Item == nil {
		return nil, nil
	}

	var user UserRecord
	if err := attributevalue.UnmarshalMap(result.Item, &user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &user, nil
}

// GetUserByEmail looks up a user by email (scan-based, acceptable for small user base).
func (s *Store) GetUserByEmail(ctx context.Context, email string) (*UserRecord, error) {
	result, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &s.tableName,
		FilterExpression: aws.String("begins_with(PK, :prefix) AND email = :email"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prefix": &types.AttributeValueMemberS{Value: "USER#"},
			":email":  &types.AttributeValueMemberS{Value: email},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("scan user by email: %w", err)
	}
	if len(result.Items) == 0 {
		return nil, nil
	}

	var user UserRecord
	if err := attributevalue.UnmarshalMap(result.Items[0], &user); err != nil {
		return nil, fmt.Errorf("unmarshal user: %w", err)
	}
	return &user, nil
}

// ApproveUser sets a user's status to active.
func (s *Store) ApproveUser(ctx context.Context, userID string) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
		UpdateExpression: aws.String("SET #status = :status, approvedAt = :at"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "active"},
			":at":     &types.AttributeValueMemberS{Value: now},
		},
	})
	if err != nil {
		return fmt.Errorf("approve user: %w", err)
	}
	return nil
}

// SuspendUser sets a user's status to suspended.
func (s *Store) SuspendUser(ctx context.Context, userID string) error {
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
		UpdateExpression: aws.String("SET #status = :status"),
		ExpressionAttributeNames: map[string]string{
			"#status": "status",
		},
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":status": &types.AttributeValueMemberS{Value: "suspended"},
		},
	})
	if err != nil {
		return fmt.Errorf("suspend user: %w", err)
	}
	return nil
}

// ListUsers returns all users (scan-based, acceptable for small user base).
func (s *Store) ListUsers(ctx context.Context) ([]UserRecord, error) {
	result, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &s.tableName,
		FilterExpression: aws.String("begins_with(PK, :prefix) AND SK = :sk"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prefix": &types.AttributeValueMemberS{Value: "USER#"},
			":sk":     &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("list users: %w", err)
	}

	var users []UserRecord
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &users); err != nil {
		return nil, fmt.Errorf("unmarshal users: %w", err)
	}
	return users, nil
}

// EstimateCost calculates the estimated USD cost for a podcast generation.
func EstimateCost(model, ttsProvider string, inputChars, ttsChars, durationSec int) float64 {
	var cost float64

	// Script generation cost (rough estimates based on API pricing)
	inputTokens := float64(inputChars) / 4 // ~4 chars per token
	switch model {
	case "haiku":
		cost += inputTokens * 0.80 / 1_000_000  // input
		cost += inputTokens * 4.00 / 1_000_000   // output (assume ~1:1 ratio)
	case "sonnet":
		cost += inputTokens * 3.00 / 1_000_000
		cost += inputTokens * 15.00 / 1_000_000
	case "gemini-flash":
		cost += inputTokens * 0.075 / 1_000_000
		cost += inputTokens * 0.30 / 1_000_000
	case "gemini-pro":
		cost += inputTokens * 1.25 / 1_000_000
		cost += inputTokens * 10.00 / 1_000_000
	}

	// TTS cost
	ttsCharsF := float64(ttsChars)
	switch ttsProvider {
	case "gemini":
		// Gemini TTS is included in the API pricing, minimal additional cost
		cost += ttsCharsF * 0.000016 // ~$16 per 1M chars
	case "elevenlabs":
		cost += ttsCharsF * 0.00018 // ~$180 per 1M chars (Creator plan rate)
	case "google":
		cost += ttsCharsF * 0.000016 // Google Cloud TTS standard
	}

	return cost
}

// RecordUsage updates the podcast item with usage data and increments the monthly rollup.
func (s *Store) RecordUsage(ctx context.Context, podcastID, userID, model, ttsProvider string, inputChars, ttsChars, durationSec int) error {
	cost := EstimateCost(model, ttsProvider, inputChars, ttsChars, durationSec)

	// Update podcast record with usage data
	_, err := s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "PODCAST#" + podcastID},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
		UpdateExpression: aws.String("SET userId = :uid, inputCharCount = :ic, ttsCharCount = :tc, outputDurationSec = :dur, estimatedCostUSD = :cost"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":uid":  &types.AttributeValueMemberS{Value: userID},
			":ic":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", inputChars)},
			":tc":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttsChars)},
			":dur":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", durationSec)},
			":cost": &types.AttributeValueMemberN{Value: fmt.Sprintf("%.6f", cost)},
		},
	})
	if err != nil {
		return fmt.Errorf("update podcast usage: %w", err)
	}

	// Increment monthly rollup
	month := time.Now().UTC().Format("2006-01")
	_, err = s.client.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &types.AttributeValueMemberS{Value: "USAGE#" + month},
		},
		UpdateExpression: aws.String("ADD podcastCount :one, totalDurationSec :dur, totalTTSChars :tc, totalCostUSD :cost"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":one":  &types.AttributeValueMemberN{Value: "1"},
			":dur":  &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", durationSec)},
			":tc":   &types.AttributeValueMemberN{Value: fmt.Sprintf("%d", ttsChars)},
			":cost": &types.AttributeValueMemberN{Value: fmt.Sprintf("%.6f", cost)},
		},
	})
	if err != nil {
		return fmt.Errorf("update monthly rollup: %w", err)
	}

	return nil
}

// GetMonthlyUsage returns the usage rollup for a user for a given month (YYYY-MM).
func (s *Store) GetMonthlyUsage(ctx context.Context, userID, month string) (*UsageRecord, error) {
	result, err := s.client.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &s.tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + userID},
			"SK": &types.AttributeValueMemberS{Value: "USAGE#" + month},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("get monthly usage: %w", err)
	}
	if result.Item == nil {
		return &UsageRecord{}, nil
	}

	var usage UsageRecord
	if err := attributevalue.UnmarshalMap(result.Item, &usage); err != nil {
		return nil, fmt.Errorf("unmarshal usage: %w", err)
	}
	return &usage, nil
}

// ListUserPodcasts returns podcasts for a specific user.
func (s *Store) ListUserPodcasts(ctx context.Context, userID string, limit int) ([]PodcastItem, error) {
	if limit <= 0 {
		limit = 20
	}

	// Scan with filter on userId (acceptable for small dataset)
	result, err := s.client.Scan(ctx, &dynamodb.ScanInput{
		TableName:        &s.tableName,
		FilterExpression: aws.String("begins_with(PK, :prefix) AND userId = :uid"),
		ExpressionAttributeValues: map[string]types.AttributeValue{
			":prefix": &types.AttributeValueMemberS{Value: "PODCAST#"},
			":uid":    &types.AttributeValueMemberS{Value: userID},
		},
		Limit: aws.Int32(int32(limit)),
	})
	if err != nil {
		return nil, fmt.Errorf("list user podcasts: %w", err)
	}

	var items []PodcastItem
	if err := attributevalue.UnmarshalListOfMaps(result.Items, &items); err != nil {
		return nil, fmt.Errorf("unmarshal podcasts: %w", err)
	}
	return items, nil
}
