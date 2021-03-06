GO = go
BINARY_NAME = hashlink
CMD_FILES = $(filter-out cmd/%_test.go,$(wildcard cmd/*.go))

.PHONY: build test

build:
	$(GO) build -o $(BINARY_NAME) $(CMD_FILES)

test:
	$(GO) test ./...