.PHONY: build
# This is used for release builds by .github/workflows/build.yml
build:
	@echo "--> Building Vault $(VAULT_VERSION)"
	@go build -v -o "$(OUTPUT_DIR)"/
