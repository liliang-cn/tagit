GO ?= go
GO_ENV ?= GOWORK=off
BIN_DIR ?= bin
PREFIX ?= $(HOME)/.local
INSTALL_BIN_DIR ?= $(PREFIX)/bin

.PHONY: all build build-tagit build-tagitd build-tagittui desktop-frontend-build desktop-build test install clean

WAILS ?= $(shell $(GO) env GOPATH)/bin/wails
DESKTOP_WAILS_TAGS ?= webkit2_41

all: build

build: build-tagit build-tagitd build-tagittui

$(BIN_DIR):
	mkdir -p $(BIN_DIR)

build-tagit: | $(BIN_DIR)
	$(GO_ENV) $(GO) build -o $(BIN_DIR)/tagit ./cmd/tagit

build-tagitd: | $(BIN_DIR)
	$(GO_ENV) $(GO) build -o $(BIN_DIR)/tagitd ./cmd/tagitd

build-tagittui: | $(BIN_DIR)
	$(GO_ENV) $(GO) build -o $(BIN_DIR)/tagittui ./cmd/tagittui

desktop-frontend-build:
	cd desktop/frontend && npm install && npm run build

desktop-build: desktop-frontend-build
	cd desktop && GOWORK=off $(WAILS) build -nopackage -m -s -tags "$(DESKTOP_WAILS_TAGS)"

test:
	$(GO_ENV) $(GO) test -count=1 ./...

install: build
	mkdir -p $(INSTALL_BIN_DIR)
	install -m 0755 $(BIN_DIR)/tagit $(INSTALL_BIN_DIR)/tagit
	install -m 0755 $(BIN_DIR)/tagitd $(INSTALL_BIN_DIR)/tagitd
	install -m 0755 $(BIN_DIR)/tagittui $(INSTALL_BIN_DIR)/tagittui

clean:
	rm -rf $(BIN_DIR)
