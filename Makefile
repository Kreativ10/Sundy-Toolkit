APP := sundy
VERSION ?= 0.1.0
LDFLAGS := -s -w -X main.version=$(VERSION)

.PHONY: build test lint clean dist

build:
	CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o bin/$(APP) ./cmd/sundy

test:
	go test ./...

lint:
	go vet ./...

dist: test
	mkdir -p dist
	GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sundy-linux-amd64 ./cmd/sundy
	GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sundy-linux-arm64 ./cmd/sundy
	GOOS=linux GOARCH=arm GOARM=7 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sundy-linux-arm ./cmd/sundy
	GOOS=linux GOARCH=riscv64 CGO_ENABLED=0 go build -trimpath -ldflags "$(LDFLAGS)" -o dist/sundy-linux-riscv64 ./cmd/sundy
	cd dist && sha256sum sundy-linux-* > checksums.txt

clean:
	rm -rf bin dist
