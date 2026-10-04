.PHONY: build install test clean fmt

BINARY_NAME=terraform-provider-berth
VERSION?=0.2.1
OS_ARCH?=linux_amd64

build:
	go build -ldflags "-X main.version=${VERSION}" -o ${BINARY_NAME}

install: build
	mkdir -p ~/.terraform.d/plugins/registry.terraform.io/tech-arch1tect/berth/${VERSION}/${OS_ARCH}
	cp ${BINARY_NAME} ~/.terraform.d/plugins/registry.terraform.io/tech-arch1tect/berth/${VERSION}/${OS_ARCH}/

test:
	go test ./... -v

fmt:
	go fmt ./...

clean:
	rm -f ${BINARY_NAME}
	go clean

.DEFAULT_GOAL := build
