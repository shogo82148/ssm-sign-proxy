help: ## Show this text.
	# https://marmelab.com/blog/2016/02/29/auto-documented-makefile.html
	@grep -E '^[a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-30s\033[0m %s\n", $$1, $$2}'

GOOS=$(shell go env GOOS)
GOARCH=$(shell go env GOARCH)
VERSION=$(patsubst "%",%,$(lastword $(shell grep 'const Version' version.go)))
ARTIFACTS_DIR=$(CURDIR)/artifacts/$(VERSION)
RELEASE_DIR=$(CURDIR)/release/$(VERSION)
LATEST_DIR=$(CURDIR)/release/latest
SRC_FILES=$(shell find . -type f -name '*.go')

all: build-windows-amd64 build-linux-amd64 build-darwin-amd64 build-function ## Build binaries.

.PHONY: all test clean help

clean: ## Remove built files.
	rm -rf $(CURDIR)/artifacts
	rm -rf $(CURDIR)/release

test: ## Run test.
	go test -v -race ./...
	go vet ./...

##### build settings

.PHONY: build build-windows-amd64 build-linux-amd64 build-darwin-amd64 build-function

$(ARTIFACTS_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH):
	@mkdir -p $@

$(ARTIFACTS_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH)/ssm-sign-proxy$(SUFFIX): $(ARTIFACTS_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH) $(SRC_FILES) go.mod go.sum
	@echo " * Building binary for $(GOOS)/$(GOARCH)..."
	@CGO_ENABLED=0 go build -o $@ ./cmd/ssm-sign-proxy

build: $(ARTIFACTS_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH)/ssm-sign-proxy$(SUFFIX)

build-windows-amd64:
	@$(MAKE) build GOOS=windows GOARCH=amd64 SUFFIX=.exe

build-linux-amd64:
	@$(MAKE) build GOOS=linux GOARCH=amd64

build-darwin-amd64:
	@$(MAKE) build GOOS=darwin GOARCH=amd64

build-function: $(ARTIFACTS_DIR)/ssm-sign-proxy-function/ssm-sign-proxy-function

$(ARTIFACTS_DIR)/ssm-sign-proxy-function:
	@mkdir -p $@

$(ARTIFACTS_DIR)/ssm-sign-proxy-function/ssm-sign-proxy-function: $(ARTIFACTS_DIR)/ssm-sign-proxy-function $(SRC_FILES) go.mod go.sum
	@echo " * Building binary for aws lambda function..."
	@CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o $@ ./cmd/ssm-sign-proxy-function

##### release settings

.PHONY: release-windows-amd64 release-linux-amd64 release-darwin-amd64
.PHONY: release-targz release-zip release-files release-upload

$(RELEASE_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH):
	@mkdir -p $@

release-windows-amd64:
	@$(MAKE) release-zip GOOS=windows GOARCH=amd64 SUFFIX=.exe

release-linux-amd64:
	@$(MAKE) release-targz GOOS=linux GOARCH=amd64

release-darwin-amd64:
	@$(MAKE) release-zip GOOS=darwin GOARCH=amd64

release-targz: build $(RELEASE_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH)
	@echo " * Creating tar.gz for $(GOOS)/$(GOARCH)"
	tar -czf $(RELEASE_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH).tar.gz -C $(ARTIFACTS_DIR) ssm-sign-proxy_$(GOOS)_$(GOARCH)

release-zip: build $(RELEASE_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH)
	@echo " * Creating zip for $(GOOS)/$(GOARCH)"
	cd $(ARTIFACTS_DIR) && zip -9 $(RELEASE_DIR)/ssm-sign-proxy_$(GOOS)_$(GOARCH).zip ssm-sign-proxy_$(GOOS)_$(GOARCH)/*

$(RELEASE_DIR)/ssm-sign-proxy-function:
	@mkdir -p $@

$(LATEST_DIR):
	@mkdir -p $@

$(LATEST_DIR)/ssm-sign-proxy-function.zip: $(LATEST_DIR) $(RELEASE_DIR)/ssm-sign-proxy-function
	cp $(RELEASE_DIR)/ssm-sign-proxy-function.zip $(LATEST_DIR)/ssm-sign-proxy-function.zip

$(RELEASE_DIR)/ssm-sign-proxy-function.zip: $(ARTIFACTS_DIR)/ssm-sign-proxy-function/ssm-sign-proxy-function $(RELEASE_DIR)/ssm-sign-proxy-function
	cd $(ARTIFACTS_DIR)/ssm-sign-proxy-function && zip -9 $(RELEASE_DIR)/ssm-sign-proxy-function.zip *

release-function: $(LATEST_DIR)/ssm-sign-proxy-function.zip $(RELEASE_DIR)/ssm-sign-proxy-function.zip

release-files: release-windows-amd64 release-linux-amd64 release-darwin-amd64 release-function

release-upload: release-files
	ghr -u $(GITHUB_USERNAME) --draft --replace v$(VERSION) $(RELEASE_DIR)

##### AWS SAM

.PHONY: release-sam

release-sam: $(LATEST_DIR)/ssm-sign-proxy-function.zip template.yaml ## Release the application to AWS Serverless Application Repository
	sam package \
		--template-file template.yaml \
		--output-template-file packaged.yaml \
		--s3-bucket shogo82148-sam
	sam publish \
		--template packaged.yaml \
		--region us-east-1
