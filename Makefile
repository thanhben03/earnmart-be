.PHONY: run test test-race tidy build migrate-up migrate-down migrate-version docker-up docker-down

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

migrate-up:
	go run ./cmd/migrate up

migrate-down:
	go run ./cmd/migrate down 1

migrate-version:
	go run ./cmd/migrate version

docker-up:
	docker compose up --build -d

docker-down:
	docker compose down
