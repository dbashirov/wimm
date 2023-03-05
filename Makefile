.PHONY: build
build:
	go build -v ./cmd/app

.DEFAULT_GOAL := build 