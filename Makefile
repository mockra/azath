BIN_DIR := $(HOME)/bin
BINARY  := azath

.PHONY: build install uninstall test fmt vet tidy

build:
	go build -o $(BINARY) ./cmd/azath

install: build
	mkdir -p $(BIN_DIR)
	ln -sf $(CURDIR)/$(BINARY) $(BIN_DIR)/$(BINARY)
	@echo "Installed $(BIN_DIR)/$(BINARY) -> $(CURDIR)/$(BINARY)"

uninstall:
	rm -f $(BIN_DIR)/$(BINARY)

test:
	go test ./...

fmt:
	gofmt -w .

vet:
	go vet ./...

tidy:
	go mod tidy
