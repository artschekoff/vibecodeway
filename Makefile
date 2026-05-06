VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DIST_DIR := dist
APP_NAME := context-consolidator
BUNDLE_NAME := ContextConsolidator

INSTALL_DIR := /usr/local/bin

.PHONY: setup dev build build-desktop build-mcp build-cli install-mcp install-cli install-app install-all test lint vet fmt validate clean clean-frontend build-all package-desktop release db-truncate db-reinit

setup:
	cd frontend && npm install

dev: install-cli
	wails dev

build: build-desktop build-mcp build-cli

build-desktop:
	wails build

build-mcp:
	go build -o bin/context-consolidator-mcp ./cmd/mcp

build-cli:
	go build -o bin/context-consolidator-cli ./cmd/cli

install-mcp: build-mcp
	sudo install -m 755 $(CURDIR)/bin/$(APP_NAME)-mcp $(INSTALL_DIR)/$(APP_NAME)-mcp
	@echo "Installed → $(INSTALL_DIR)/$(APP_NAME)-mcp"

install-cli: build-cli
	sudo install -m 755 $(CURDIR)/bin/$(APP_NAME)-cli $(INSTALL_DIR)/$(APP_NAME)-cli
	@echo "Installed → $(INSTALL_DIR)/$(APP_NAME)-cli"

install-app: build-desktop
	sudo rm -rf /Applications/$(BUNDLE_NAME).app
	sudo cp -r build/bin/$(APP_NAME).app /Applications/$(BUNDLE_NAME).app
	@echo "Installed → /Applications/$(BUNDLE_NAME).app"

install-all: install-mcp install-cli install-app

test:
	go test ./... -race -count=1

lint:
	golangci-lint run

fmt:
	gofumpt -l -w .

vet:
	go vet ./...

validate: fmt vet lint test

db-truncate:
	@sqlite3 ~/.context-consolidator/context_consolidator.db \
		"DELETE FROM slack_threads; DELETE FROM slack_channels; DELETE FROM slack_provider; DELETE FROM jira_comments; DELETE FROM jira_issues; DELETE FROM jira_provider; DELETE FROM contexts;"
	@echo "All tables truncated."

db-reinit:
	@rm -f ~/.context-consolidator/context_consolidator.db \
		~/.context-consolidator/context_consolidator.db-shm \
		~/.context-consolidator/context_consolidator.db-wal
	@echo "Database removed. Migrations will run automatically on next start."

clean:
	rm -rf build/bin bin/ $(DIST_DIR)/

clean-frontend:
	@if ! rm -rf frontend/dist 2>/dev/null; then \
		echo ""; \
		echo "  Permission denied — frontend/dist is root-owned (was built with sudo)."; \
		echo "  Fix once with: sudo rm -rf frontend/dist"; \
		echo ""; \
		exit 1; \
	fi

rulesync-install:
	cp commands/*.md .rulesync/commands/
	rulesync generate --targets cursor --features "*"

MCP_PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

package-desktop:
	@mkdir -p $(DIST_DIR)/
	@echo "Building desktop app..."
	@wails build
	@arch=$$(uname -m | sed 's/x86_64/amd64/;s/arm64/arm64/'); \
	out="$(DIST_DIR)/$(APP_NAME)-desktop_darwin_$${arch}.zip"; \
	cp -r build/bin/$(APP_NAME).app build/bin/$(BUNDLE_NAME).app && \
	zip -qr "$$out" build/bin/$(BUNDLE_NAME).app && \
	rm -rf build/bin/$(BUNDLE_NAME).app; \
	echo "Desktop archive → $$out"

build-all:
	@rm -rf $(DIST_DIR)/
	@mkdir -p $(DIST_DIR)/
	@for platform in $(MCP_PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		out_mcp="$(DIST_DIR)/$(APP_NAME)-mcp_$${os}_$${arch}"; \
		out_cli="$(DIST_DIR)/$(APP_NAME)-cli_$${os}_$${arch}"; \
		echo "Building MCP $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o $$out_mcp ./cmd/mcp; \
		tar -czf "$${out_mcp}.tar.gz" -C $(DIST_DIR) "$(APP_NAME)-mcp_$${os}_$${arch}"; \
		rm $$out_mcp; \
		echo "Building CLI $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o $$out_cli ./cmd/cli; \
		tar -czf "$${out_cli}.tar.gz" -C $(DIST_DIR) "$(APP_NAME)-cli_$${os}_$${arch}"; \
		rm $$out_cli; \
	done
	@$(MAKE) package-desktop
	@echo "Archives written to $(DIST_DIR)/"

release:
	@prev=$$(sed -n 's/.*serverVersion = "\([^"]*\)".*/\1/p' cmd/mcp/main.go); \
	echo "Current version: $$prev"; \
	read -p "New version: " version; \
	sed -i '' "s/serverVersion = \"$$prev\"/serverVersion = \"$$version\"/" cmd/mcp/main.go; \
	git add cmd/mcp/main.go; \
	git commit -m "chore(release): bump version to v$$version"; \
	git tag -a "v$$version" -m "Release v$$version"; \
	git push origin main --tags; \
	$(MAKE) build-all; \
	gh release create "v$$version" $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip \
		--title "v$$version" \
		--generate-notes
