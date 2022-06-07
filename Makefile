.PHONY: build
# This is used for release builds by .github/workflows/build.yml
build:
	@go build -v -o "$(OUTPUT_PATH)"
