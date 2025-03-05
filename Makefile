MANAGER_PROTO_DIR := ./proto
MANAGER_OUT_DIR := ./gen/manager
MANAGER_PROTO_FILES := $(wildcard $(MANAGER_PROTO_DIR)/*.proto)

YANET_DIR := ./third_party/yanet
YANET_PROTO_DIR := $(YANET_DIR)/libprotobuf
YANET_OUT_DIR := ./gen/yanet
YANET_MODULE := yanet/;yanetpb

GOOGLE_API_DIR := ./third_party/googleapis

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

# Generate code for local .proto files
generate-local: $(GOOGLE_API_DIR)
	@mkdir -p $(MANAGER_OUT_DIR)
	@protoc -I=$(MANAGER_PROTO_DIR) -I=$(GOOGLE_API_DIR) -I=$(PROTOBUF_INCLUDE) \
		--go_out=$(MANAGER_OUT_DIR) \
		--go_opt=paths=source_relative \
		--go-grpc_out=$(MANAGER_OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		--grpc-gateway_out=$(MANAGER_OUT_DIR) \
		--grpc-gateway_opt=paths=source_relative \
		--openapiv2_out=$(MANAGER_OUT_DIR) \
		$(MANAGER_PROTO_FILES)
	@echo "✅ Local .proto files generated"

# Generate code for YANET .proto files
generate-yanet: $(YANET_DIR)
	@mkdir -p $(YANET_OUT_DIR)
	@FILES=$$(ls -1 $(YANET_PROTO_DIR)/*.proto | xargs -n1 basename); \
	GO_OPTS=; GRPC_OPTS=; \
	for file in $$FILES; do \
		GO_OPTS+="--go_opt=M$$file=$(YANET_MODULE) "; \
		GRPC_OPTS+="--go-grpc_opt=M$$file=$(YANET_MODULE) "; \
	done; \
	protoc -I. -I=$(YANET_PROTO_DIR) -I=$(PROTOBUF_INCLUDE) \
		--go_out=$(YANET_OUT_DIR) \
		--go_opt=paths=source_relative \
		$$GO_OPTS \
		--go-grpc_out=$(YANET_OUT_DIR) \
		--go-grpc_opt=paths=source_relative \
		--go-grpc_opt=require_unimplemented_servers=false \
		$$GRPC_OPTS \
		$$FILES
	@echo "✅ YANET .proto files generated"

# Generate all protobuf files
generate: generate-local generate-yanet

# Clean generated files
clean:
	@rm -rf $(MANAGER_OUT_DIR) $(GOOGLE_API_DIR) $(YANET_DIR) $(YANET_OUT_DIR)
	@rm -rf ./gen
	@rm -rf ./third_party
	@echo "🗑️  Cleanup completed"

# Build the Go application
build: generate
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

.PHONY: all generate generate-local generate-yanet clean build install-tools setup
