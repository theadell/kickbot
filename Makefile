
.PHONY: build

mock:
	@echo "running mockgen to generate mock of SlackClient..."
	@mockgen -source=./cmd/kickbot/slack_client.go -destination=./cmd/kickbot/slack_client_mock.go -package=main
build:
	@echo "Building cmd/kickbot..."
	@go build -ldflags '-s -w' -o ./bin/kickbot ./cmd/kickbot

test: 
	@echo "Runnint Tests..."
	@go test -cover ./...

test-race: 
	@echo "Runnint Tests with race detector..."
	@go test -race -cover ./...

run: build
	@echo "Starting server on port 4000..."
	@(./bin/kickbot -port 4000)
