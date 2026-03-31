BINARY := devc
VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
LDFLAGS := -ldflags "-s -w -X main.version=$(VERSION)"

.PHONY: build test clean install lint release

build:
	go build $(LDFLAGS) -o bin/$(BINARY) .

install:
	go install $(LDFLAGS) .

test:
	go test ./...

lint:
	go vet ./...

release:
	$(eval TAG := v$(shell date +%Y.%m.%d))
	git tag -fa $(TAG) -m "Release $(TAG)"
	git push origin $(TAG) --force

clean:
	rm -rf bin/
