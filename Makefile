.PHONY: build install clean dev build-mcp-server build-play-counter build-proxy docker-build docker-push deploy-infra create-secrets deploy-agentcore update-agentcore force-update-agentcore deploy verify-deploy smoke-test smoke-test-local smoke-test-proxy build-portal create-admin-user create-test-apikey

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
	rm -rf deploy/lambda-build deploy/proxy-build deploy/sdk

dev: build
	./$(BINARY) generate -i docs/PRD.md -o test-episode.mp3 --script-only

# --- MCP Server ---

build-mcp-server:
	CGO_ENABLED=0 go build -ldflags="-s -w" -o mcp-server ./cmd/mcp-server

build-play-counter:
	mkdir -p deploy/lambda-build
	CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -ldflags="-s -w" -tags lambda.norpc -o deploy/lambda-build/bootstrap ./cmd/play-counter

build-proxy:
	mkdir -p deploy/proxy-build
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -tags lambda.norpc -ldflags="-s -w" -o deploy/proxy-build/bootstrap ./cmd/mcp-proxy

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

deploy-infra: build-play-counter build-proxy build-portal
	cd deploy/infrastructure && npm install && npx cdk deploy --all --require-approval never

# --- Secrets ---

create-secrets:
	@echo "Creating Secrets Manager secrets for MCP server..."
	@for key in ANTHROPIC_API_KEY GEMINI_API_KEY ELEVENLABS_API_KEY VERTEX_AI_API_KEY; do \
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
	@if [ -n "$$GCP_SERVICE_ACCOUNT_FILE" ]; then \
		echo "  Creating /podcaster/mcp/GCP_SERVICE_ACCOUNT_JSON from $$GCP_SERVICE_ACCOUNT_FILE..."; \
		aws secretsmanager create-secret \
			--name "/podcaster/mcp/GCP_SERVICE_ACCOUNT_JSON" \
			--secret-string "file://$$GCP_SERVICE_ACCOUNT_FILE" \
			--region $(AWS_REGION) 2>/dev/null || \
		aws secretsmanager put-secret-value \
			--secret-id "/podcaster/mcp/GCP_SERVICE_ACCOUNT_JSON" \
			--secret-string "file://$$GCP_SERVICE_ACCOUNT_FILE" \
			--region $(AWS_REGION); \
	fi
	@echo "Done."

# --- AgentCore ---

AGENTCORE_ROLE_ARN := $(shell aws cloudformation describe-stacks --stack-name PodcasterMcpStack --query "Stacks[0].Outputs[?OutputKey=='AgentCoreRoleArn'].OutputValue" --output text 2>/dev/null)
DYNAMODB_TABLE := podcaster-prod
S3_BUCKET := podcaster-audio-$(AWS_ACCOUNT_ID)
CDN_BASE_URL := https://podcasts.apresai.dev

deploy-agentcore:
	aws bedrock-agentcore-control create-agent-runtime \
		--agent-runtime-name podcaster_mcp \
		--description "Podcast generator MCP server - converts URLs/text to podcast audio" \
		--agent-runtime-artifact containerConfiguration={containerUri="$(ECR_URI):latest"} \
		--role-arn $(AGENTCORE_ROLE_ARN) \
		--network-configuration networkMode=PUBLIC \
		--protocol-configuration serverProtocol=MCP \
		--environment-variables 'DYNAMODB_TABLE=$(DYNAMODB_TABLE),S3_BUCKET=$(S3_BUCKET),CDN_BASE_URL=$(CDN_BASE_URL),SECRET_PREFIX=/podcaster/mcp/,OTEL_SERVICE_NAME=podcaster-mcp,OTEL_TRACES_EXPORTER=otlp,OTEL_EXPORTER_OTLP_PROTOCOL=grpc' \
		--region $(AWS_REGION)

update-agentcore:
	@RUNTIME_ID=$$(aws bedrock-agentcore-control list-agent-runtimes --query "agentRuntimes[?agentRuntimeName=='podcaster_mcp'].agentRuntimeId" --output text --region $(AWS_REGION)); \
	echo "Updating runtime $$RUNTIME_ID..."; \
	aws bedrock-agentcore-control update-agent-runtime \
		--agent-runtime-id $$RUNTIME_ID \
		--agent-runtime-artifact '{"containerConfiguration":{"containerUri":"$(ECR_URI):latest"}}' \
		--role-arn $(AGENTCORE_ROLE_ARN) \
		--network-configuration '{"networkMode":"PUBLIC"}' \
		--protocol-configuration '{"serverProtocol":"MCP"}' \
		--environment-variables '{"DYNAMODB_TABLE":"$(DYNAMODB_TABLE)","S3_BUCKET":"$(S3_BUCKET)","CDN_BASE_URL":"$(CDN_BASE_URL)","SECRET_PREFIX":"/podcaster/mcp/","OTEL_SERVICE_NAME":"podcaster-mcp","OTEL_TRACES_EXPORTER":"otlp","OTEL_EXPORTER_OTLP_PROTOCOL":"grpc"}' \
		--region $(AWS_REGION)

# Force-update ALL AgentCore runtimes by re-applying their current config (pulls latest container image)
force-update-agentcore:
	@echo "Force-updating all AgentCore runtimes in $(AWS_REGION)..."
	@RUNTIME_IDS=$$(aws bedrock-agentcore-control list-agent-runtimes \
		--query "agentRuntimes[].agentRuntimeId" --output text --region $(AWS_REGION)); \
	if [ -z "$$RUNTIME_IDS" ]; then \
		echo "No runtimes found."; exit 0; \
	fi; \
	for RTID in $$RUNTIME_IDS; do \
		echo "--- Fetching config for $$RTID..."; \
		CONFIG=$$(aws bedrock-agentcore-control get-agent-runtime \
			--agent-runtime-id $$RTID --region $(AWS_REGION) --output json); \
		ROLE=$$(echo "$$CONFIG" | python3 -c "import sys,json; print(json.load(sys.stdin)['roleArn'])"); \
		ARTIFACT=$$(echo "$$CONFIG" | python3 -c "import sys,json; d=json.load(sys.stdin)['agentRuntimeArtifact']; print(json.dumps(d))"); \
		NETWORK=$$(echo "$$CONFIG" | python3 -c "import sys,json; d=json.load(sys.stdin)['networkConfiguration']; print(json.dumps(d))"); \
		PROTOCOL=$$(echo "$$CONFIG" | python3 -c "import sys,json; d=json.load(sys.stdin).get('protocolConfiguration',{}); print(json.dumps(d))"); \
		ENVVARS=$$(echo "$$CONFIG" | python3 -c "import sys,json; d=json.load(sys.stdin).get('environmentVariables',{}); print(json.dumps(d))"); \
		echo "--- Updating $$RTID (role=$$ROLE)..."; \
		aws bedrock-agentcore-control update-agent-runtime \
			--agent-runtime-id $$RTID \
			--agent-runtime-artifact "$$ARTIFACT" \
			--role-arn "$$ROLE" \
			--network-configuration "$$NETWORK" \
			--protocol-configuration "$$PROTOCOL" \
			--environment-variables "$$ENVVARS" \
			--region $(AWS_REGION); \
		echo "--- $$RTID update triggered."; \
	done; \
	echo "All runtimes updated."

# --- Full Deploy Pipeline ---

deploy: clean build-play-counter build-proxy build-portal deploy-infra docker-push force-update-agentcore verify-deploy

# --- Verification ---

verify-deploy:
	@echo "Waiting for AgentCore runtime to be READY..."
	@RUNTIME_ID=$$(aws bedrock-agentcore-control list-agent-runtimes \
		--query "agentRuntimes[?agentRuntimeName=='podcaster_mcp'].agentRuntimeId" \
		--output text --region $(AWS_REGION)); \
	if [ -z "$$RUNTIME_ID" ]; then \
		echo "ERROR: No runtime named podcaster_mcp found"; exit 1; \
	fi; \
	echo "Runtime ID: $$RUNTIME_ID"; \
	ELAPSED=0; \
	while [ $$ELAPSED -lt 180 ]; do \
		STATUS=$$(aws bedrock-agentcore-control get-agent-runtime \
			--agent-runtime-id $$RUNTIME_ID --region $(AWS_REGION) \
			--query "status" --output text 2>/dev/null); \
		echo "  [$$ELAPSED""s] Status: $$STATUS"; \
		if [ "$$STATUS" = "READY" ]; then \
			echo "Runtime is READY."; exit 0; \
		fi; \
		if [ "$$STATUS" = "FAILED" ]; then \
			echo "ERROR: Runtime entered FAILED state."; exit 1; \
		fi; \
		sleep 5; \
		ELAPSED=$$((ELAPSED + 5)); \
	done; \
	echo "ERROR: Timed out after 180s waiting for READY state (last: $$STATUS)"; exit 1

smoke-test:
	@echo "Smoke-testing deployed AgentCore MCP server..."
	@RUNTIME_ARN=$$(aws bedrock-agentcore-control list-agent-runtimes \
		--query "agentRuntimes[?agentRuntimeName=='podcaster_mcp'].agentRuntimeArn" \
		--output text --region $(AWS_REGION)); \
	if [ -z "$$RUNTIME_ARN" ]; then \
		echo "ERROR: No runtime named podcaster_mcp found"; exit 1; \
	fi; \
	echo "Runtime ARN: $$RUNTIME_ARN"; \
	echo "--- Sending initialize..."; \
	aws bedrock-agentcore invoke-agent-runtime \
		--agent-runtime-arn $$RUNTIME_ARN \
		--region $(AWS_REGION) \
		--cli-binary-format raw-in-base64-out \
		--accept "application/json, text/event-stream" \
		--payload '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"smoke-test"},"capabilities":{}}}' \
		/tmp/podcaster-smoke-init.json >/dev/null 2>&1; \
	cat /tmp/podcaster-smoke-init.json | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'podcaster' in d.get('result',{}).get('serverInfo',{}).get('name',''), f'Bad init: {d}'; print('  init OK: server=' + d['result']['serverInfo']['name'])" || { echo "FAIL: initialize"; cat /tmp/podcaster-smoke-init.json; exit 1; }; \
	echo "--- Sending tools/list..."; \
	aws bedrock-agentcore invoke-agent-runtime \
		--agent-runtime-arn $$RUNTIME_ARN \
		--region $(AWS_REGION) \
		--cli-binary-format raw-in-base64-out \
		--accept "application/json, text/event-stream" \
		--payload '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' \
		/tmp/podcaster-smoke-tools.json >/dev/null 2>&1; \
	cat /tmp/podcaster-smoke-tools.json | python3 -c "\
import sys,json; d=json.load(sys.stdin); \
tools=[t['name'] for t in d.get('result',{}).get('tools',[])]; \
expected={'generate_podcast','get_podcast','list_podcasts'}; \
found=set(tools) & expected; \
assert found==expected, f'Missing tools: {expected-found}, got: {tools}'; \
print('  tools OK: ' + ', '.join(sorted(tools)))" || { echo "FAIL: tools/list"; cat /tmp/podcaster-smoke-tools.json; exit 1; }; \
	echo "Smoke test PASSED."

smoke-test-proxy:
	@echo "Smoke-testing MCP proxy at https://podcasts.apresai.dev/mcp..."
	@if [ -z "$(API_KEY)" ]; then echo "Usage: make smoke-test-proxy API_KEY=pk_..."; exit 1; fi
	@echo "--- Sending initialize..."
	@RESP=$$(curl -sf https://podcasts.apresai.dev/mcp \
		-H "Authorization: Bearer $(API_KEY)" \
		-H 'Content-Type: application/json' \
		-d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"smoke-test"},"capabilities":{}}}'); \
	echo "$$RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'podcaster' in d.get('result',{}).get('serverInfo',{}).get('name',''), f'Bad init: {d}'; print('  init OK: server=' + d['result']['serverInfo']['name'])" || { echo "FAIL"; echo "$$RESP"; exit 1; }; \
	echo "--- Sending tools/list..."; \
	TOOLS_RESP=$$(curl -sf https://podcasts.apresai.dev/mcp \
		-H "Authorization: Bearer $(API_KEY)" \
		-H 'Content-Type: application/json' \
		-H "Mcp-Session-Id: $$(echo $$RESP | python3 -c 'import sys,json; print(json.load(sys.stdin).get(\"sessionId\",\"\"))')" \
		-d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'); \
	echo "$$TOOLS_RESP" | python3 -c "\
import sys,json; d=json.load(sys.stdin); \
tools=[t['name'] for t in d.get('result',{}).get('tools',[])]; \
expected={'generate_podcast','get_podcast','list_podcasts'}; \
found=set(tools) & expected; \
assert found==expected, f'Missing tools: {expected-found}, got: {tools}'; \
print('  tools OK: ' + ', '.join(sorted(tools)))" || { echo "FAIL: tools/list"; echo "$$TOOLS_RESP"; exit 1; }; \
	echo "Smoke test PASSED."

# --- Portal ---

build-portal:
	cd portal && npm install && npx --yes open-next build

# --- Admin Tools ---

# Create admin user in DynamoDB for testing (run with EMAIL=your@email.com)
create-admin-user:
	@if [ -z "$(EMAIL)" ]; then echo "Usage: make create-admin-user EMAIL=you@example.com"; exit 1; fi
	@USER_ID=$$(python3 -c "import uuid; print(str(uuid.uuid4()))"); \
	NOW=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	echo "Creating admin user: $(EMAIL) (id=$$USER_ID)"; \
	aws dynamodb put-item \
		--table-name $(DYNAMODB_TABLE) \
		--item '{"PK":{"S":"USER#'"$$USER_ID"'"},"SK":{"S":"PROFILE"},"email":{"S":"$(EMAIL)"},"name":{"S":"Admin"},"status":{"S":"active"},"role":{"S":"admin"},"createdAt":{"S":"'"$$NOW"'"},"approvedAt":{"S":"'"$$NOW"'"}}' \
		--region $(AWS_REGION); \
	echo "Admin user created: $$USER_ID"

# Create an API key for testing (run with USER_ID=uuid)
create-test-apikey:
	@if [ -z "$(USER_ID)" ]; then echo "Usage: make create-test-apikey USER_ID=<uuid>"; exit 1; fi
	@KEY=$$(python3 -c "import secrets; print('pk_' + secrets.token_hex(32))"); \
	PREFIX=$$(echo $$KEY | cut -c4-11); \
	HASH=$$(python3 -c "import hashlib,sys; print(hashlib.sha256(sys.argv[1].encode()).hexdigest())" "$$KEY"); \
	NOW=$$(date -u +%Y-%m-%dT%H:%M:%SZ); \
	echo "Creating API key: $$PREFIX..."; \
	aws dynamodb put-item \
		--table-name $(DYNAMODB_TABLE) \
		--item '{"PK":{"S":"APIKEY#'"$$PREFIX"'"},"SK":{"S":"METADATA"},"userId":{"S":"$(USER_ID)"},"keyHash":{"S":"'"$$HASH"'"},"name":{"S":"test-key"},"status":{"S":"active"},"createdAt":{"S":"'"$$NOW"'"}}' \
		--region $(AWS_REGION); \
	echo ""; \
	echo "API Key (save this, shown once):"; \
	echo "  $$KEY"; \
	echo ""; \
	echo "Key prefix: $$PREFIX"

smoke-test-local:
	@echo "Smoke-testing local MCP server at http://localhost:8000/mcp..."
	@echo "--- Sending initialize..."; \
	INIT_RESP=$$(curl -sf http://localhost:8000/mcp \
		-H 'Content-Type: application/json' \
		-d '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{"protocolVersion":"2024-11-05","clientInfo":{"name":"smoke-test"},"capabilities":{}}}'); \
	echo "$$INIT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); assert 'podcaster' in d.get('result',{}).get('serverInfo',{}).get('name',''), f'Bad init: {d}'; print('  init OK: server=' + d['result']['serverInfo']['name'])" || { echo "FAIL: initialize"; exit 1; }; \
	SESSION_ID=$$(echo "$$INIT_RESP" | python3 -c "import sys,json; d=json.load(sys.stdin); print(d.get('sessionId',''))"); \
	echo "--- Sending tools/list..."; \
	TOOLS_RESP=$$(curl -sf http://localhost:8000/mcp \
		-H 'Content-Type: application/json' \
		-H "Mcp-Session-Id: $$SESSION_ID" \
		-d '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}'); \
	echo "$$TOOLS_RESP" | python3 -c "\
import sys,json; d=json.load(sys.stdin); \
tools=[t['name'] for t in d.get('result',{}).get('tools',[])]; \
expected={'generate_podcast','get_podcast','list_podcasts'}; \
found=set(tools) & expected; \
assert found==expected, f'Missing tools: {expected-found}, got: {tools}'; \
print('  tools OK: ' + ', '.join(sorted(tools)))" || { echo "FAIL: tools/list"; exit 1; }; \
	echo "Smoke test PASSED."
