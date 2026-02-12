package script

import (
	"context"
	"fmt"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime/types"
)

var novaModels = map[string]string{
	"nova-lite": "us.amazon.nova-2-lite-v1:0",
}

type NovaGenerator struct {
	model  string
	client *bedrockruntime.Client
}

func NewNovaGenerator(model string) (*NovaGenerator, error) {
	cfg, err := config.LoadDefaultConfig(context.Background())
	if err != nil {
		return nil, fmt.Errorf("load AWS config: %w", err)
	}
	return &NovaGenerator{
		model:  model,
		client: bedrockruntime.NewFromConfig(cfg),
	}, nil
}

func (g *NovaGenerator) Generate(ctx context.Context, content string, opts GenerateOptions) (*Script, error) {
	personas := buildPersonaSlice(opts.Voices, opts.SpeakerNames)
	sysPrompt := buildSystemPrompt(personas)
	userPrompt := buildUserPrompt(content, opts)

	modelID := novaModels[g.model]
	if modelID == "" {
		modelID = novaModels["nova-lite"]
	}

	maxTokens := int32(maxTokensForDuration(opts.Duration))

	var lastErr error
	backoff := initialBackoff

	for attempt := 1; attempt <= maxRetries; attempt++ {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := g.client.Converse(ctx, &bedrockruntime.ConverseInput{
			ModelId: aws.String(modelID),
			System: []types.SystemContentBlock{
				&types.SystemContentBlockMemberText{Value: sysPrompt},
			},
			Messages: []types.Message{
				{
					Role: types.ConversationRoleUser,
					Content: []types.ContentBlock{
						&types.ContentBlockMemberText{Value: userPrompt},
					},
				},
			},
			InferenceConfig: &types.InferenceConfiguration{
				MaxTokens:   aws.Int32(maxTokens),
				Temperature: aws.Float32(temperature),
			},
		})
		if err != nil {
			lastErr = fmt.Errorf("Bedrock Converse error (attempt %d/%d): %w", attempt, maxRetries, err)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		text := extractNovaText(resp)
		if text == "" {
			lastErr = fmt.Errorf("empty response from Bedrock (attempt %d/%d)", attempt, maxRetries)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		script, err := parseScript(text, personas)
		if err != nil {
			lastErr = fmt.Errorf("failed to parse script JSON (attempt %d/%d): %w", attempt, maxRetries, err)
			if attempt < maxRetries {
				select {
				case <-ctx.Done():
					return nil, ctx.Err()
				case <-time.After(backoff):
				}
				backoff *= time.Duration(backoffMult)
			}
			continue
		}

		return script, nil
	}

	return nil, lastErr
}

func extractNovaText(resp *bedrockruntime.ConverseOutput) string {
	if resp.Output == nil {
		return ""
	}
	msg, ok := resp.Output.(*types.ConverseOutputMemberMessage)
	if !ok {
		return ""
	}
	for _, block := range msg.Value.Content {
		if tb, ok := block.(*types.ContentBlockMemberText); ok {
			return tb.Value
		}
	}
	return ""
}
