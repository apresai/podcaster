.PHONY: build install clean dev

BINARY := podcaster
VERSION := 0.1.0
LDFLAGS := -ldflags "-s -w -X github.com/apresai/podcaster/internal/cli.Version=$(VERSION)"

build:
	go build $(LDFLAGS) -o $(BINARY) ./cmd/podcaster

install:
	go install $(LDFLAGS) ./cmd/podcaster

clean:
	rm -f $(BINARY)

dev: build
	./$(BINARY) generate -i docs/PRD.md -o test-episode.mp3 --script-only
