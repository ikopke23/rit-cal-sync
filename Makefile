BINARY      := rit-cal-sync
DESTDIR     := /usr/local/bin
GO          := $(shell which go)
GOFUMPT     := $(shell which gofumpt)
LINT        := $(shell which golangci-lint)
TESTMAP     := $(shell which gocover-testmap)

ifeq ($(GO),)
$(error Go not found in PATH)
endif

.PHONY: build install uninstall test vet lint fix fmt fmt-check coverage clean cov-map

build:
	$(GO) build -o bin/$(BINARY) ./cmd/$(BINARY)

install: build
	install -m 755 bin/$(BINARY) $(DESTDIR)/$(BINARY)

uninstall:
	rm -f $(DESTDIR)/$(BINARY)

test:
ifneq ($(TESTMAP),)
	$(TESTMAP) test -race ./...
else
	$(GO) test -race -count=1 ./...
endif

vet:
	$(GO) vet ./...

lint:
ifeq ($(LINT),)
	$(error golangci-lint not found in PATH)
endif
	$(LINT) run

fix:
ifeq ($(LINT),)
	$(error golangci-lint not found in PATH)
endif
	$(LINT) run --fix

fmt:
ifeq ($(GOFUMPT),)
	$(error gofumpt not found in PATH)
endif
	$(GOFUMPT) -l -w .

fmt-check:
ifeq ($(GOFUMPT),)
	$(error gofumpt not found in PATH)
endif
	@if [ -n "$(shell $(GOFUMPT) -l .)" ]; then \
		echo "Files not gofumpt-formatted:"; \
		$(GOFUMPT) -l .; \
		exit 1; \
	fi

coverage:
	$(GO) test -coverprofile=coverage.out ./...
	$(GO) tool cover -html=coverage.out

cov-map:
ifeq ($(TESTMAP),)
	$(error gocover-testmap not found in PATH)
endif
	$(TESTMAP) test -testmapfile=coverage.testmap -coverprofile=coverage.out ./...
	$(TESTMAP) report -i coverage.testmap

clean:
	rm -rf bin/
