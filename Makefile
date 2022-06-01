.PHONY: build
build:
	go build -v ./cmd/wimm

.DEFAULT_GOAL := build 