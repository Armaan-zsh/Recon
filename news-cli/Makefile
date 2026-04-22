BINARY_NAME=recon
GO=$(shell which go 2>/dev/null || echo "$(HOME)/go/bin/go")
INSTALL_DIR=$(HOME)/.local/bin

.PHONY: build install uninstall clean release

## Build the binary
build:
	$(GO) build -o $(BINARY_NAME) ./cmd/recon

## Build and install to ~/.local/bin
install: build
	@mkdir -p $(INSTALL_DIR)
	@cp -f $(BINARY_NAME) $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Installed $(BINARY_NAME) to $(INSTALL_DIR)/$(BINARY_NAME)"
	@echo ""
	@echo "Make sure $(INSTALL_DIR) is in your PATH:"
	@echo '  export PATH="$$HOME/.local/bin:$$PATH"'

## Remove the installed binary
uninstall:
	@rm -f $(INSTALL_DIR)/$(BINARY_NAME)
	@echo "✓ Removed $(BINARY_NAME) from $(INSTALL_DIR)"

## Clean build artifacts
clean:
	@rm -f $(BINARY_NAME)
	@echo "✓ Cleaned build artifacts"

## Build for all platforms (for GitHub releases)
release:
	@mkdir -p dist
	GOOS=linux   GOARCH=amd64 $(GO) build -o dist/$(BINARY_NAME)-linux-amd64 ./cmd/recon
	GOOS=linux   GOARCH=arm64 $(GO) build -o dist/$(BINARY_NAME)-linux-arm64 ./cmd/recon
	GOOS=darwin  GOARCH=amd64 $(GO) build -o dist/$(BINARY_NAME)-darwin-amd64 ./cmd/recon
	GOOS=darwin  GOARCH=arm64 $(GO) build -o dist/$(BINARY_NAME)-darwin-arm64 ./cmd/recon
	GOOS=windows GOARCH=amd64 $(GO) build -o dist/$(BINARY_NAME)-windows-amd64.exe ./cmd/recon
	@echo "✓ Built release binaries in dist/"
	@ls -lh dist/
