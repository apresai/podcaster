.PHONY: build install clean dev

BINARY := podcaster
BUILD_DIR := ./bin
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w -X github.com/apresai/podcaster/internal/cli.Version=$(VERSION)"

build:
	@mkdir -p $(BUILD_DIR)
	go build $(LDFLAGS) -o $(BUILD_DIR)/$(BINARY) ./cmd/podcaster

install:
	go install $(LDFLAGS) ./cmd/podcaster

clean:
	rm -rf $(BUILD_DIR)

dev: build
	$(BUILD_DIR)/$(BINARY) generate -i docs/PRD.md -o test-episode.mp3 --script-only
