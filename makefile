GO_CMD=go
GOFMT_CMD=gofmt
STATICCHECK_CMD=staticcheck

all: build test
install:
	$(GO_CMD) install honnef.co/go/tools/cmd/staticcheck@latest
build:
	$(GO_CMD) build ./...
format:
	$(GOFMT_CMD) -w -s .
check:
	$(GO_CMD) vet ./...
	$(STATICCHECK_CMD) ./...
test:
	$(GO_CMD) test -race -coverprofile cover.out ./...
	$(GO_CMD) tool cover -html=cover.out -o cover.html
clean:
	$(GO_CMD) clean
