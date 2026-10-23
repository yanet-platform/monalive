MONALIVE_PROJECT := github.com/yanet-platform/monalive

MANAGER_PROTO_DIR := monalivepb/v1
MANAGER_PROTO_FILES := $(wildcard $(MANAGER_PROTO_DIR)/*.proto)
MANAGER_GENERATED_FILES := $(MANAGER_PROTO_FILES:.proto=.pb.go) \
	$(MANAGER_PROTO_FILES:.proto=_grpc.pb.go) \
	$(MANAGER_PROTO_FILES:.proto=.pb.gw.go) \
	$(MANAGER_PROTO_FILES:.proto=.swagger.json)

YANET_DIR := third_party/yanet
YANET_PROTO_DIR := libprotobuf
YANET_OUT_DIR := gen/yanet
YANET_MODULE := $(MONALIVE_PROJECT)/$(YANET_OUT_DIR)/$(YANET_PROTO_DIR);yanetpb

YANET2_DIR := third_party/yanet2
YANET2_OUT_DIR := gen/yanet2
YANET2_BALANCER_PROTO_DIR := modules/balancer/controlplane/balancerpb
YANET2_BALANCER_MODULE := $(MONALIVE_PROJECT)/$(YANET2_OUT_DIR)/$(YANET2_BALANCER_PROTO_DIR);balancerpb
YANET2_COMMON_PROTO_DIR := common/commonpb
YANET2_COMMON_MODULE := $(MONALIVE_PROJECT)/$(YANET2_OUT_DIR)/$(YANET2_COMMON_PROTO_DIR);commonpb

GOOGLE_API_DIR := third_party/googleapis

# Go module path for locating standard .proto files
PROTOBUF_INCLUDE := $(shell go list -m -f "{{.Dir}}" google.golang.org/protobuf)

# Default target
all: generate

# Clone Google APIs repository if not present
$(GOOGLE_API_DIR):
	@git clone --depth 1 https://github.com/googleapis/googleapis.git $(GOOGLE_API_DIR)

# Clone YANET API repository if not present
$(YANET_DIR):
	@git clone --depth 1 https://github.com/yanet-platform/yanet.git $(YANET_DIR)
	@cd $(YANET_DIR) && git sparse-checkout init --cone
	@cd $(YANET_DIR) && git sparse-checkout set libprotobuf

# Clone YANET2 Balancer API repository if not present
$(YANET2_DIR):
	@git clone --depth 1 https://github.com/yanet-platform/yanet2.git $(YANET2_DIR)
	@cd $(YANET2_DIR) && git sparse-checkout init --cone
	@cd $(YANET2_DIR) && git sparse-checkout set \
		$(YANET2_BALANCER_PROTO_DIR) \
		$(YANET2_COMMON_PROTO_DIR)

# Generate local API code next to its .proto sources, as in YANET2.
generate-local: $(GOOGLE_API_DIR)
	@protoc -I=. -I=$(GOOGLE_API_DIR) -I=$(PROTOBUF_INCLUDE) \
		--go_out=paths=source_relative:. \
		--go-grpc_out=paths=source_relative:. \
		--go-grpc_opt=require_unimplemented_servers=false \
		--grpc-gateway_out=paths=source_relative:. \
		--openapiv2_out=. \
		$(MANAGER_PROTO_FILES)
	@echo "✅ Local .proto files generated"

# Generate code for YANET .proto files
generate-yanet: $(YANET_DIR)
	@mkdir -p $(YANET_OUT_DIR)
	@set -e; GO_OPTS=; GRPC_OPTS=; \
	for file in $(YANET_DIR)/$(YANET_PROTO_DIR)/*.proto; do \
		file=$${file#$(YANET_DIR)/}; \
		GO_OPTS=$$GO_OPTS"--go_opt=M$$file=$(YANET_MODULE) "; \
		GRPC_OPTS=$$GRPC_OPTS"--go-grpc_opt=M$$file=$(YANET_MODULE) "; \
	done; \
	protoc -I=$(YANET_DIR) -I=$(PROTOBUF_INCLUDE) \
		--go_out=$(YANET_OUT_DIR) \
		--go_opt=paths=source_relative \
		$$GO_OPTS \
		--go-grpc_out=$(YANET_OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		$$GRPC_OPTS \
		$$(ls -1 $(YANET_DIR)/$(YANET_PROTO_DIR)/*.proto)
	@echo "✅ YANET .proto files generated"

# Generate code for YANET2 balancer .proto files
generate-yanet2: $(YANET2_DIR)
	@mkdir -p $(YANET2_OUT_DIR)
	@set -e; GO_OPTS=; GRPC_OPTS=; \
	for file in $(YANET2_DIR)/$(YANET2_COMMON_PROTO_DIR)/*.proto; do \
		file=$${file#$(YANET2_DIR)/}; \
		GO_OPTS=$$GO_OPTS"--go_opt=M$$file=$(YANET2_COMMON_MODULE) "; \
		GRPC_OPTS=$$GRPC_OPTS"--go-grpc_opt=M$$file=$(YANET2_COMMON_MODULE) "; \
	done; \
	for file in $(YANET2_DIR)/$(YANET2_BALANCER_PROTO_DIR)/*.proto; do \
		file=$${file#$(YANET2_DIR)/}; \
		GO_OPTS=$$GO_OPTS"--go_opt=M$$file=$(YANET2_BALANCER_MODULE) "; \
		GRPC_OPTS=$$GRPC_OPTS"--go-grpc_opt=M$$file=$(YANET2_BALANCER_MODULE) "; \
	done; \
	protoc -I=$(YANET2_DIR) -I=$(PROTOBUF_INCLUDE) \
		--go_out=$(YANET2_OUT_DIR) \
		--go_opt=paths=source_relative \
		$$GO_OPTS \
		--go-grpc_out=$(YANET2_OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		$$GRPC_OPTS \
		$$(ls -1 $(YANET2_DIR)/$(YANET2_COMMON_PROTO_DIR)/*.proto $(YANET2_DIR)/$(YANET2_BALANCER_PROTO_DIR)/*.proto)
	@echo "✅ YANET2 balancer .proto files generated"

# Generate all Go protobuf bindings, including the external backend APIs.
proto-go: generate-local generate-yanet generate-yanet2

# Backwards-compatible entry point for existing build scripts.
generate: proto-go

# Clean generated files
clean:
	@rm -f $(MANAGER_GENERATED_FILES)
	@rm -rf ./gen
	@rm -rf ./third_party
	@echo "🗑️  Cleanup completed"

# Build the Go application
build: proto-go
	@GOOS=linux GOARCH=amd64 go build -o monalive ./cmd/monalive
	@echo "🚀 Build completed: ./monalive"

# Install necessary tools
install-tools:
	@go install google.golang.org/protobuf/cmd/protoc-gen-go@latest
	@go install google.golang.org/grpc/cmd/protoc-gen-go-grpc@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-grpc-gateway@latest
	@go install github.com/grpc-ecosystem/grpc-gateway/v2/protoc-gen-openapiv2@latest
	@echo "🔧 Tools installed"

# Full setup: install dependencies, generate code, and build the application
setup: install-tools generate build

.PHONY: all proto-go generate generate-local generate-yanet generate-yanet2 clean build install-tools setup
