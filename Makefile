SHELL := /bin/sh

APP_NAME := gateway
CMD_DIR  := cmd/app

test:
	go test -count=1 ./...

test-cover:
	go test -count=1 -coverprofile=coverage.out ./...
	go tool cover -func=coverage.out
	rm -rf ./bin coverage.out

vet:
	go vet ./...

lint: fmt vet

build:
	mkdir -p bin
	go build -o ./bin/$(APP_NAME) $(CMD_DIR)

run:
	go run $(CMD_DIR)/main.go

swag_init:
	swag init -g $(CMD_DIR)/main.go --output docs

deploy:
    eval $(minikube docker-env) && docker build -t gateway:latest . && kubectl apply -f k8s