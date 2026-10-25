PREFIX ?= /usr/local
DESTDIR ?=
VERSION ?= 0.0.3
GURI_PREFIX = $(if $(filter /%,$(firstword $(PREFIX))),$(PREFIX),$(CURDIR)/$(PREFIX))
GURI_LIBRARY_ROOT = $(GURI_PREFIX)/share/guri
LDFLAGS = -X 'main.version=$(VERSION)' -X 'github.com/0x6d61/guri/internal/core.standardLibraryDirectory=$(GURI_LIBRARY_ROOT)/$(VERSION)/lib' -X 'github.com/0x6d61/guri/internal/core.siteLibraryDirectory=$(GURI_LIBRARY_ROOT)/site/lib'

.PHONY: build install test
build:
	mkdir -p build
	go build -ldflags "$(LDFLAGS)" -o build/guri ./cmd/guri

install: build
	install -d "$(DESTDIR)$(GURI_PREFIX)/bin" "$(DESTDIR)$(GURI_LIBRARY_ROOT)/$(VERSION)/lib" "$(DESTDIR)$(GURI_LIBRARY_ROOT)/site/lib"
	install -m 755 build/guri "$(DESTDIR)$(GURI_PREFIX)/bin/guri"

test:
	go test ./...
	go vet ./...
