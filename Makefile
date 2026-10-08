.PHONY: run test test-race tidy build docker-up docker-down

run:
	go run ./cmd/api

test:
	go test ./... -cover

test-race:
	go test ./... -race -cover

tidy:
	go mod tidy

build:
	go build -o bin/api ./cmd/api

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
