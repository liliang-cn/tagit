GO ?= go
GO_ENV ?= GOWORK=off
BIN_DIR ?= bin
PREFIX ?= $(HOME)/.local
INSTALL_BIN_DIR ?= $(PREFIX)/bin

.PHONY: all build build-tagit build-tagitd test install clean

all: build

build: build-tagit build-tagitd

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build-tagit: | $(BIN_DIR)
	$(GO_ENV) $(GO) build -o $(BIN_DIR)/tagit ./cmd/tagit

build-tagitd: | $(BIN_DIR)
	$(GO_ENV) $(GO) build -o $(BIN_DIR)/tagitd ./cmd/tagitd

test:
	$(GO_ENV) $(GO) test -count=1 ./...

install: build
	mkdir -p $(INSTALL_BIN_DIR)
	install -m 0755 $(BIN_DIR)/tagit $(INSTALL_BIN_DIR)/tagit
	install -m 0755 $(BIN_DIR)/tagitd $(INSTALL_BIN_DIR)/tagitd

clean:
	rm -rf $(BIN_DIR)
