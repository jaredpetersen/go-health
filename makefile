GO_CMD=go
GO_BUILD=$(GO_CMD) build
GO_FMT=gofmt
GO_TEST=$(GO_CMD) test
GO_COVER=$(GO_CMD) tool cover
GO_CLEAN=$(GO_CMD) clean

all: build test
build:
	$(GO_BUILD) ./...
format:
	$(GO_FMT) -w -s .
test:
	$(GO_TEST) -coverprofile cover.out ./...
	$(GO_COVER) -html=cover.out -o cover.html
clean:
	$(GO_CLEAN)
