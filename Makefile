
.PHONY: mock build test test-race build-docker-image run

TAG ?= latest

mock:
	@echo "Running mockgen to generate mock of SlackClient..."
	@mockgen -source=./cmd/kickbot/slack_client.go -destination=./cmd/kickbot/slack_client_mock.go -package=main
build:
	@echo "Building cmd/kickbot..."
	@go build -ldflags '-s -w' -o ./bin/kickbot ./cmd/kickbot

test: 
	@echo "Running Tests..."
	@go test -cover ./...

test-race: 
	@echo "Running Tests with race detector..."
	@go test -race -cover ./...

build-docker-image:
	@echo "building docker image for kickbot..."
	@docker build -t kickbot .

run: build
	@echo "Starting server on port 4000..."
	@(./bin/kickbot -port 4000)
