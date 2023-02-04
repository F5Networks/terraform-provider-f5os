default: testacc

build:
	go build -v ./...

install: build
	go install -v ./...

# See https://golangci-lint.run/
lint:
	golangci-lint run

generate:
	go generate ./...

fmt:
	gofmt -s -w -e .

test:
	go test -v -cover -timeout=120s -parallel=4 ./...

testacc:
	TF_ACC=1 go test -v -parallel=1 -cover -timeout 120m ./...

clean:
	go clean -testcache

.PHONY: build install lint generate fmt test testacc