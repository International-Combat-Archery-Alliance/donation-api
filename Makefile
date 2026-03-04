.PHONY: build
build:
	cd api && go generate
	go build -o bootstrap ./cmd

.PHONY: build-sam
build-sam:
	cd api && go generate
	sam build --parameter-overrides architecture=x86_64

.PHONY: local
local: build-sam
	sam local start-api --parameter-overrides architecture=x86_64 --warm-containers EAGER

.PHONY: test
test:
	go test ./...

.PHONY: test-verbose
test-verbose:
	go test -v ./...

.PHONY: fmt
fmt:
	go fmt ./...

.PHONY: vet
vet:
	go vet ./...

.PHONY: tidy
tidy:
	go mod tidy

.PHONY: download
download:
	go mod download

.PHONY: generate
generate:
	cd api && go generate

.PHONY: clean
clean:
	rm -f bootstrap
	rm -rf .aws-sam/
