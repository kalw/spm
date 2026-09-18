# spm — build, test, and release helpers. Run `make help` for the list.

BIN        := spm
PKG        := ./cmd/spm
DIST       := dist
VERSION    ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo dev)
LDFLAGS    := -s -w -X main.version=$(VERSION)
PLATFORMS  := linux/amd64 linux/arm64 darwin/amd64 darwin/arm64 \
              freebsd/amd64 freebsd/arm64 openbsd/amd64 openbsd/arm64

# MinIO settings for `make integration`.
# quay.io is used (not Docker Hub) so CI runners can pull anonymously.
MINIO_IMAGE  ?= quay.io/minio/minio
MINIO_NAME   := spm-minio
MINIO_PORT   := 9000
MINIO_USER   := minioadmin
MINIO_PASS   := minioadmin
MINIO_BUCKET := spm-it

.DEFAULT_GOAL := build

.PHONY: help
help: ## Show this help
	@grep -hE '^[a-zA-Z0-9_-]+:.*?## ' $(MAKEFILE_LIST) | \
	  awk 'BEGIN{FS=":.*?## "}{printf "  \033[36m%-16s\033[0m %s\n", $$1, $$2}'

.PHONY: build
build: ## Build the binary into ./$(BIN)
	go build -trimpath -ldflags "$(LDFLAGS)" -o $(BIN) $(PKG)

.PHONY: install
install: ## go install the binary
	go install -trimpath -ldflags "$(LDFLAGS)" $(PKG)

.PHONY: test
test: ## Run unit tests
	go test ./...

.PHONY: cover
cover: ## Run unit tests with a coverage summary
	go test -cover ./...

.PHONY: vet
vet: ## Run go vet
	go vet ./...

.PHONY: fmt
fmt: ## Format the code
	gofmt -s -w .

.PHONY: tidy
tidy: ## Tidy go.mod/go.sum
	go mod tidy

.PHONY: check
check: vet test ## Vet + unit tests

.PHONY: cross
cross: ## Cross-compile all platforms into $(DIST)/
	@mkdir -p $(DIST)
	@for p in $(PLATFORMS); do \
	  os=$${p%/*}; arch=$${p#*/}; out="$(DIST)/$(BIN)-$$os-$$arch"; \
	  echo "building $$out"; \
	  CGO_ENABLED=0 GOOS=$$os GOARCH=$$arch \
	    go build -trimpath -ldflags "$(LDFLAGS)" -o "$$out" $(PKG) || exit 1; \
	done

.PHONY: dist
dist: cross ## Cross-compile, tar.gz each target, and write SHA256SUMS
	@cd $(DIST) && for f in $(BIN)-*; do tar -czf "$$f.tar.gz" "$$f" && rm -f "$$f"; done
	@cd $(DIST) && (sha256sum *.tar.gz 2>/dev/null || shasum -a 256 *.tar.gz) > SHA256SUMS
	@echo "artifacts in $(DIST)/:" && ls -1 $(DIST)

.PHONY: clean
clean: ## Remove build artifacts
	rm -rf $(BIN) $(DIST)

# ---------------------------------------------------------------------------
# Integration tests (MinIO via Docker). Requires docker + a network.
# ---------------------------------------------------------------------------

.PHONY: minio-up
minio-up: ## Start a local MinIO container
	@docker run -d --rm --name $(MINIO_NAME) -p $(MINIO_PORT):9000 \
	  -e MINIO_ROOT_USER=$(MINIO_USER) -e MINIO_ROOT_PASSWORD=$(MINIO_PASS) \
	  $(MINIO_IMAGE) server /data >/dev/null
	@echo -n "waiting for MinIO"; \
	for i in $$(seq 1 30); do \
	  curl -fs http://127.0.0.1:$(MINIO_PORT)/minio/health/ready >/dev/null 2>&1 && { echo " ready"; break; }; \
	  echo -n "."; sleep 1; \
	done

.PHONY: minio-down
minio-down: ## Stop the local MinIO container
	@docker rm -f $(MINIO_NAME) >/dev/null 2>&1 || true

.PHONY: integration
integration: ## Run MinIO-backed integration tests (starts/stops MinIO)
	@$(MAKE) minio-up
	@set +e; \
	  AWS_ACCESS_KEY_ID=$(MINIO_USER) AWS_SECRET_ACCESS_KEY=$(MINIO_PASS) \
	  SPM_IT_S3_ENDPOINT=http://127.0.0.1:$(MINIO_PORT) \
	  SPM_IT_S3_BUCKET=$(MINIO_BUCKET) SPM_IT_S3_REGION=us-east-1 \
	  go test -tags integration -count=1 ./internal/storage/...; \
	  status=$$?; \
	  $(MAKE) minio-down; \
	  exit $$status
