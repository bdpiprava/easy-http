NAME=easy-http
REPO=github.com/bdpiprava/${NAME}

BUILD_DIR=build

NO_COLOR=\033[0m
OK_COLOR=\033[32;01m
ERROR_COLOR=\033[31;01m
WARN_COLOR=\033[33;01m

## Run tests
tests:
	@echo "$(OK_COLOR)==> Running tests...$(NO_COLOR)"
	@go install gotest.tools/gotestsum@latest
	@gotestsum --format=testname -- -v -race=1 -coverprofile=coverage_unit.txt -coverpkg=./... ./...

## Remove build and vendor directory
clean:
	@echo "$(OK_COLOR)==> Running clean...$(NO_COLOR)"
	@rm -rf build/
	@rm -rf vendor/

## Build the binary
build:
	@echo "$(OK_COLOR)==> Running build...$(NO_COLOR)"
	@go build -o build/ -v ./...

## Install dependencies
deps:
	@go mod tidy
	@go mod vendor
	@go mod download

## Install the binary
install:
	@go install ${REPO}

## Generate static documentation
generate-doc:
	@echo "$(OK_COLOR)==> Generating documentation...$(NO_COLOR)"
	@mkdir -p ${BUILD_DIR}/docs
	@go run ./docs/main.go -generate
	@echo "$(OK_COLOR)==> Documentation generated in ${BUILD_DIR}/docs$(NO_COLOR)"