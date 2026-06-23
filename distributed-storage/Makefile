.PHONY: proto build docker-up docker-down docker-logs docker-ps test test-unit test-integration test-race vet lint fmt demo clean

MODULE_ROOT := $(dir $(abspath $(lastword $(MAKEFILE_LIST))))
BIN_DIR := $(MODULE_ROOT)bin
GO := go
PROTO_SCRIPT := $(MODULE_ROOT)scripts/generate_proto.sh

proto:
	chmod +x $(PROTO_SCRIPT)
	PATH="$$(go env GOPATH)/bin:$$PATH" $(PROTO_SCRIPT)

build: $(BIN_DIR)/master $(BIN_DIR)/storage $(BIN_DIR)/client

$(BIN_DIR)/master $(BIN_DIR)/storage $(BIN_DIR)/client:
	@mkdir -p $(BIN_DIR)
	$(GO) build -o $(BIN_DIR)/$(notdir $@) ./cmd/$(notdir $@)

docker-up:
	docker compose -f deployments/docker-compose.yml up --build -d

docker-down:
	docker compose -f deployments/docker-compose.yml down

docker-logs:
	docker compose -f deployments/docker-compose.yml logs -f

docker-ps:
	docker compose -f deployments/docker-compose.yml ps

test: test-unit

test-unit:
	$(GO) test ./tests/unit/... -count=1

test-integration:
	$(GO) test ./tests/integration/... -count=1 -tags=integration -timeout=10m

test-race:
	$(GO) test -race ./tests/unit/... -count=1

test-cover:
	$(GO) test -coverpkg=./internal/... -coverprofile=coverage.out ./tests/unit/... ./internal/config/... -count=1
	$(GO) tool cover -func=coverage.out | tail -1

vet:
	$(GO) vet ./...

lint:
	golangci-lint run ./...

fmt:
	gofmt -w .
	goimports -w .

demo:
	./scripts/demo.sh

clean:
	rm -rf $(BIN_DIR)
	$(GO) clean -cache -testcache
