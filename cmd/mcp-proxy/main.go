//go:build lambda.norpc

package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"
	"time"

	"github.com/aws/aws-lambda-go/events"
	"github.com/aws/aws-lambda-go/lambda"
	"github.com/aws/aws-sdk-go-v2/aws"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/feature/dynamodb/attributevalue"
	"github.com/aws/aws-sdk-go-v2/service/bedrockagentcore"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb"
	"github.com/aws/aws-sdk-go-v2/service/dynamodb/types"
)

var (
	ddbClient *dynamodb.Client
	acClient  *bedrockagentcore.Client
	tableName string
	runtimeARN string
	log       *slog.Logger
)

func init() {
	log = slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	tableName = os.Getenv("DYNAMODB_TABLE")
	runtimeARN = os.Getenv("RUNTIME_ARN")

	if tableName == "" || runtimeARN == "" {
		log.Error("DYNAMODB_TABLE and RUNTIME_ARN environment variables are required")
		os.Exit(1)
	}

	cfg, err := awsconfig.LoadDefaultConfig(context.Background())
	if err != nil {
		log.Error("Failed to load AWS config", "error", err)
		os.Exit(1)
	}

	ddbClient = dynamodb.NewFromConfig(cfg)
	acClient = bedrockagentcore.NewFromConfig(cfg)
}

func main() {
	lambda.Start(handler)
}

func handler(ctx context.Context, req events.LambdaFunctionURLRequest) (events.LambdaFunctionURLResponse, error) {
	if req.RequestContext.HTTP.Method == "OPTIONS" {
		return events.LambdaFunctionURLResponse{StatusCode: 204}, nil
	}

	if req.RequestContext.HTTP.Method != "POST" {
		return jsonRPCError(405, nil, -32600, "Method not allowed"), nil
	}

	// Validate auth
	authHeader := getHeader(req.Headers, "authorization")
	if authHeader == "" {
		return jsonRPCError(401, nil, -32001, "Missing Authorization header"), nil
	}

	token := strings.TrimPrefix(authHeader, "Bearer ")
	if token == authHeader || token == "" {
		return jsonRPCError(401, nil, -32001, "Invalid Authorization format, expected: Bearer <api-key>"), nil
	}

	userID, keyID, err := validateAPIKey(ctx, token)
	if err != nil {
		log.WarnContext(ctx, "Auth failed", "error", err)
		// Distinguish user-status errors (403) from key errors (401)
		if strings.Contains(err.Error(), "user account is") {
			return jsonRPCError(403, nil, -32001, err.Error()), nil
		}
		return jsonRPCError(401, nil, -32001, "Invalid API key"), nil
	}

	log.InfoContext(ctx, "Authenticated", "user_id", userID, "key_id", keyID)

	// Parse JSON-RPC to possibly inject user context
	body := []byte(req.Body)
	body, rpcID := maybeInjectUserContext(body, userID, keyID)

	// Extract MCP session ID from request headers
	mcpSessionID := getHeader(req.Headers, "mcp-session-id")

	// Forward to AgentCore
	input := &bedrockagentcore.InvokeAgentRuntimeInput{
		AgentRuntimeArn: &runtimeARN,
		Payload:         body,
		ContentType:     aws.String("application/json"),
		Accept:          aws.String("application/json, text/event-stream"),
	}
	if mcpSessionID != "" {
		input.McpSessionId = &mcpSessionID
	}

	out, err := acClient.InvokeAgentRuntime(ctx, input)
	if err != nil {
		log.ErrorContext(ctx, "AgentCore invocation failed", "error", err)
		return jsonRPCError(502, rpcID, -32603, "Upstream server error"), nil
	}
	defer out.Response.Close()

	respBody, err := io.ReadAll(out.Response)
	if err != nil {
		log.ErrorContext(ctx, "Failed to read AgentCore response", "error", err)
		return jsonRPCError(502, rpcID, -32603, "Failed to read upstream response"), nil
	}

	respHeaders := map[string]string{
		"Content-Type": "application/json",
	}
	if out.McpSessionId != nil && *out.McpSessionId != "" {
		respHeaders["Mcp-Session-Id"] = *out.McpSessionId
	}

	return events.LambdaFunctionURLResponse{
		StatusCode: 200,
		Headers:    respHeaders,
		Body:       string(respBody),
	}, nil
}

// validateAPIKey checks the bearer token against DynamoDB.
// Returns (userID, keyPrefix, error).
func validateAPIKey(ctx context.Context, token string) (string, string, error) {
	if !strings.HasPrefix(token, "pk_") {
		return "", "", fmt.Errorf("invalid API key format")
	}
	if len(token) < 11 {
		return "", "", fmt.Errorf("API key too short")
	}

	prefix := token[3:11]

	hash := sha256.Sum256([]byte(token))
	keyHash := hex.EncodeToString(hash[:])

	// Look up API key record
	result, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "APIKEY#" + prefix},
			"SK": &types.AttributeValueMemberS{Value: "METADATA"},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("lookup API key: %w", err)
	}
	if result.Item == nil {
		return "", "", fmt.Errorf("API key not found")
	}

	var keyRecord struct {
		UserID  string `dynamodbav:"userId"`
		KeyHash string `dynamodbav:"keyHash"`
		Status  string `dynamodbav:"status"`
	}
	if err := attributevalue.UnmarshalMap(result.Item, &keyRecord); err != nil {
		return "", "", fmt.Errorf("unmarshal API key: %w", err)
	}

	if keyRecord.KeyHash != keyHash {
		return "", "", fmt.Errorf("invalid API key")
	}
	if keyRecord.Status != "active" {
		return "", "", fmt.Errorf("API key is %s", keyRecord.Status)
	}

	// Look up user record
	userResult, err := ddbClient.GetItem(ctx, &dynamodb.GetItemInput{
		TableName: &tableName,
		Key: map[string]types.AttributeValue{
			"PK": &types.AttributeValueMemberS{Value: "USER#" + keyRecord.UserID},
			"SK": &types.AttributeValueMemberS{Value: "PROFILE"},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("lookup user: %w", err)
	}
	if userResult.Item == nil {
		return "", "", fmt.Errorf("user not found for API key")
	}

	var userRecord struct {
		Status string `dynamodbav:"status"`
	}
	if err := attributevalue.UnmarshalMap(userResult.Item, &userRecord); err != nil {
		return "", "", fmt.Errorf("unmarshal user: %w", err)
	}
	if userRecord.Status != "active" {
		return "", "", fmt.Errorf("user account is %s", userRecord.Status)
	}

	// Update lastUsedAt (best-effort)
	go updateKeyLastUsed(prefix)

	return keyRecord.UserID, prefix, nil
}

// updateKeyLastUsed updates the lastUsedAt timestamp on the API key record.
// Best-effort: errors are logged but not propagated.
func updateKeyLastUsed(prefix string) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()

	now := time.Now().UTC().Format(time.RFC3339)
	oneMinuteAgo := time.Now().UTC().Add(-time.Minute).Format(time.RFC3339)

	_, err := ddbClient.UpdateItem(ctx, &dynamodb.UpdateItemInput{
		TableName: &tableName,
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
	if err != nil {
		log.Warn("Failed to update lastUsedAt", "prefix", prefix, "error", err)
	}
}

// maybeInjectUserContext parses the JSON-RPC body. If the method is "tools/call",
// it injects _user_id and _key_id into params.arguments. Returns the (possibly
// modified) body and the parsed JSON-RPC id.
func maybeInjectUserContext(body []byte, userID, keyID string) ([]byte, json.RawMessage) {
	var rpc struct {
		JSONRPC string          `json:"jsonrpc"`
		ID      json.RawMessage `json:"id"`
		Method  string          `json:"method"`
		Params  json.RawMessage `json:"params,omitempty"`
	}

	if err := json.Unmarshal(body, &rpc); err != nil {
		// Can't parse — forward as-is
		return body, nil
	}

	if rpc.Method != "tools/call" {
		return body, rpc.ID
	}

	// Parse params to inject into arguments
	var params struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments,omitempty"`
	}
	if err := json.Unmarshal(rpc.Params, &params); err != nil {
		return body, rpc.ID
	}

	// Parse existing arguments (or start with empty object)
	args := make(map[string]json.RawMessage)
	if len(params.Arguments) > 0 {
		if err := json.Unmarshal(params.Arguments, &args); err != nil {
			return body, rpc.ID
		}
	}

	// Inject user context
	userIDJSON, _ := json.Marshal(userID)
	keyIDJSON, _ := json.Marshal(keyID)
	args["_user_id"] = userIDJSON
	args["_key_id"] = keyIDJSON

	// Rebuild the JSON-RPC request
	newArgs, err := json.Marshal(args)
	if err != nil {
		return body, rpc.ID
	}

	newParams := map[string]json.RawMessage{
		"name":      mustMarshal(params.Name),
		"arguments": newArgs,
	}
	newParamsJSON, err := json.Marshal(newParams)
	if err != nil {
		return body, rpc.ID
	}

	newRPC := map[string]json.RawMessage{
		"jsonrpc": mustMarshal(rpc.JSONRPC),
		"id":      rpc.ID,
		"method":  mustMarshal(rpc.Method),
		"params":  newParamsJSON,
	}
	newBody, err := json.Marshal(newRPC)
	if err != nil {
		return body, rpc.ID
	}

	log.Info("Injected user context into tools/call", "tool", params.Name, "user_id", userID)
	return newBody, rpc.ID
}

func mustMarshal(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

// getHeader does a case-insensitive header lookup.
// Lambda Function URL headers are already lowercased, but we handle both cases.
func getHeader(headers map[string]string, key string) string {
	key = strings.ToLower(key)
	for k, v := range headers {
		if strings.ToLower(k) == key {
			return v
		}
	}
	return ""
}

// jsonRPCError builds a JSON-RPC error response with the given HTTP status code.
func jsonRPCError(httpStatus int, id json.RawMessage, code int, message string) events.LambdaFunctionURLResponse {
	if id == nil {
		id = []byte("null")
	}

	resp := map[string]any{
		"jsonrpc": "2.0",
		"id":      json.RawMessage(id),
		"error": map[string]any{
			"code":    code,
			"message": message,
		},
	}

	body, _ := json.Marshal(resp)

	return events.LambdaFunctionURLResponse{
		StatusCode: httpStatus,
		Headers: map[string]string{
			"Content-Type": "application/json",
		},
		Body: string(body),
	}
}
