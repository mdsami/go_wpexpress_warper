BINARY   := bin/server
SWAG     := $(shell go env GOPATH)/bin/swag
SWAG_VER := v1.16.6

.PHONY: run build docs swag-install tidy fmt vet clean

run: docs
	go run ./cmd/server

build: docs
	go build -o $(BINARY) ./cmd/server

## docs regenerates docs/ from the @-annotations in the source.
docs:
	$(SWAG) init -g main.go \
		-d ./cmd/server,./internal/customer,./internal/httpx \
		-o ./docs --parseInternal --outputTypes go,json,yaml

## swag-install installs the generator used by `make docs`.
swag-install:
	go install github.com/swaggo/swag/cmd/swag@$(SWAG_VER)

tidy:
	go mod tidy

fmt:
	go fmt ./...

vet:
	go vet ./...

clean:
	rm -rf bin
