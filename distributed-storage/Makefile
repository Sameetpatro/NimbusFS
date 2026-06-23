.PHONY: proto build docker-up docker-down test test-integration lint fmt clean

MODULE_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(MODULE_ROOT)bin

GO := go
PROTO_SCRIPT := $(MODULE_ROOT)scripts/generate_proto.sh

# proto generation requires protoc, protoc-gen-go, and protoc-gen-go-grpc on PATH
proto:
	chmod +x $(PROTO_SCRIPT)
	$(PROTO_SCRIPT)

build: $(BIN_DIR)/master $(BIN_DIR)/storage $(BIN_DIR)/client

$(BIN_DIR)/master:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $@ ./cmd/master

$(BIN_DIR)/storage:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $@ ./cmd/storage

$(BIN_DIR)/client:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $@ ./cmd/client

docker-up:
	docker compose -f deployments/docker-compose.yml up -d --build

docker-down:
	docker compose -f deployments/docker-compose.yml down -v

test:
	$(GO) test ./... -count=1 -short

test-integration:
	$(GO) test ./tests/integration/... -count=1 -tags=integration

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

clean:
	rm -rf $(BIN_DIR)
	$(GO) clean -cache -testcache
