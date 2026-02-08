.PHONY: build install clean dev build-mcp-server build-play-counter docker-build docker-push deploy-infra create-secrets deploy-agentcore update-agentcore

BINARY := podcaster
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w -X github.com/apresai/podcaster/internal/cli.Version=$(VERSION)"

# AWS settings
AWS_REGION ?= us-east-1
AWS_ACCOUNT_ID := $(shell aws sts get-caller-identity --query Account --output text 2>/dev/null)
ECR_REPO := podcaster-mcp-server
ECR_URI := $(AWS_ACCOUNT_ID).dkr.ecr.$(AWS_REGION).amazonaws.com/$(ECR_REPO)

# --- CLI ---

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/podcaster

install:
	go install $(LDFLAGS) ./cmd/podcaster

clean:
	rm -f $(BINARY) mcp-server play-counter bootstrap
	rm -rf deploy/lambda-build deploy/sdk

dev: build
	./$(BINARY) generate -i docs/PRD.md -o test-episode.mp3 --script-only

# --- MCP Server ---

build-mcp-server:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o mcp-server ./cmd/mcp-server

build-play-counter:
	mkdir -p deploy/lambda-build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -tags lambda.norpc -o deploy/lambda-build/bootstrap ./cmd/play-counter

docker-build:
	@# Copy SDK into build context temporarily
	rm -rf deploy/sdk
	cp -r ../apresai.dev/sdk deploy/sdk
	docker buildx build --platform linux/arm64 -f deploy/Dockerfile -t $(ECR_REPO):latest --load .
	rm -rf deploy/sdk

docker-push: docker-build
	aws ecr get-login-password --region $(AWS_REGION) | docker login --username AWS --password-stdin $(ECR_URI)
	docker tag $(ECR_REPO):latest $(ECR_URI):latest
	docker push $(ECR_URI):latest

# --- Infrastructure ---

deploy-infra: build-play-counter
	cd deploy/infrastructure && npm install && npx cdk deploy --all --require-approval never

# --- Secrets ---

create-secrets:
	@echo "Creating Secrets Manager secrets for MCP server..."
	@for key in ANTHROPIC_API_KEY GEMINI_API_KEY ELEVENLABS_API_KEY; do \
		echo "  Creating /podcaster/mcp/$$key"; \
		aws secretsmanager create-secret \
			--name "/podcaster/mcp/$$key" \
			--secret-string "$$(printenv $$key)" \
			--region $(AWS_REGION) 2>/dev/null || \
		aws secretsmanager put-secret-value \
			--secret-id "/podcaster/mcp/$$key" \
			--secret-string "$$(printenv $$key)" \
			--region $(AWS_REGION); \
	done
	@echo "Done."

# --- AgentCore ---

AGENTCORE_ROLE_ARN := $(shell aws cloudformation describe-stacks --stack-name PodcasterMcpStack --query "Stacks[0].Outputs[?OutputKey=='AgentCoreRoleArn'].OutputValue" --output text 2>/dev/null)
DYNAMODB_TABLE := apresai-podcasts-prod
S3_BUCKET := apresai-podcasts-$(AWS_ACCOUNT_ID)
CDN_BASE_URL := https://podcasts.apresai.dev

deploy-agentcore:
	aws bedrock-agentcore-control create-agent-runtime \
		--agent-runtime-name podcaster_mcp \
		--description "Podcast generator MCP server - converts URLs/text to podcast audio" \
		--agent-runtime-artifact containerConfiguration={containerUri="$(ECR_URI):latest"} \
		--role-arn $(AGENTCORE_ROLE_ARN) \
		--network-configuration networkMode=PUBLIC \
		--protocol-configuration serverProtocol=MCP \
		--environment-variables 'DYNAMODB_TABLE=$(DYNAMODB_TABLE),S3_BUCKET=$(S3_BUCKET),CDN_BASE_URL=$(CDN_BASE_URL),SECRET_PREFIX=/podcaster/mcp/' \
		--region $(AWS_REGION)

update-agentcore:
	@RUNTIME_ID=$$(aws bedrock-agentcore-control list-agent-runtimes --query "agentRuntimeSummaries[?agentRuntimeName=='podcaster_mcp'].agentRuntimeId" --output text --region $(AWS_REGION)); \
	aws bedrock-agentcore-control update-agent-runtime \
		--agent-runtime-id $$RUNTIME_ID \
		--agent-runtime-artifact containerConfiguration={containerUri="$(ECR_URI):latest"} \
		--region $(AWS_REGION)
