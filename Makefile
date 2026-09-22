VERSION ?= $(shell git describe --tags --always --dirty 2>/dev/null || echo "dev")
DIST_DIR := dist
APP_NAME := vibecodeway
BUNDLE_NAME := VibeCodeWay

INSTALL_DIR := /usr/local/bin

.PHONY: setup dev build build-desktop build-cli dmg tooling-doc install-cli install-app install-all test lint vet fmt validate clean clean-frontend build-all package-desktop release db-truncate db-reinit

setup:
	cd frontend && npm install

dev: install-cli
	wails dev

build: build-desktop build-cli dmg

# -ldflags stamps the version into the binary (shown in the sidebar).
build-desktop:
	wails build -ldflags "-X main.appVersion=$(VERSION)"

build-cli:
	go build -o bin/vibecodeway-cli ./cmd/cli

# Готовый к установке образ: перетащил .app на Applications — и всё.
# hdiutil входит в macOS, никаких create-dmg и прочих зависимостей.
dmg: build-desktop
	@mkdir -p $(DIST_DIR)
	@rm -rf build/dmg && mkdir -p build/dmg
	@cp -R build/bin/$(APP_NAME).app build/dmg/$(BUNDLE_NAME).app
	@ln -s /Applications build/dmg/Applications
	@arch=$$(uname -m); \
	out="$(DIST_DIR)/$(BUNDLE_NAME)_$(VERSION)_$${arch}.dmg"; \
	rm -f "$$out"; \
	hdiutil create -volname "$(BUNDLE_NAME)" -srcfolder build/dmg -ov -format UDZO "$$out" >/dev/null; \
	rm -rf build/dmg; \
	echo "DMG → $$out"

tooling-doc: build-cli
	./bin/vibecodeway-cli agent-tool doc > docs/TOOLING.md
	@echo "wrote docs/TOOLING.md"

install-cli: build-cli
	sudo install -m 755 $(CURDIR)/bin/$(APP_NAME)-cli $(INSTALL_DIR)/$(APP_NAME)-cli
	@echo "Installed → $(INSTALL_DIR)/$(APP_NAME)-cli"

# Ставит .app прямо в /Applications — без sudo: каталог доступен группе admin.
# ditto сохраняет расширенные атрибуты и подпись (cp -r их ломает).
install-app: build-desktop
	rm -rf /Applications/$(BUNDLE_NAME).app
	ditto build/bin/$(APP_NAME).app /Applications/$(BUNDLE_NAME).app
	@echo "Installed → /Applications/$(BUNDLE_NAME).app"

install-all: install-cli install-app

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
	@sqlite3 ~/.vibecodeway/vibecodeway.db \
		"DELETE FROM slack_threads; DELETE FROM slack_channels; DELETE FROM slack_provider; DELETE FROM jira_comments; DELETE FROM jira_issues; DELETE FROM jira_provider; DELETE FROM contexts;"
	@echo "All tables truncated."

db-reinit:
	@rm -f ~/.vibecodeway/vibecodeway.db \
		~/.vibecodeway/vibecodeway.db-shm \
		~/.vibecodeway/vibecodeway.db-wal
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

CLI_PLATFORMS := darwin/amd64 darwin/arm64 linux/amd64 linux/arm64

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
	@$(MAKE) dmg

build-all:
	@rm -rf $(DIST_DIR)/
	@mkdir -p $(DIST_DIR)/
	@for platform in $(CLI_PLATFORMS); do \
		os=$$(echo $$platform | cut -d/ -f1); \
		arch=$$(echo $$platform | cut -d/ -f2); \
		out_cli="$(DIST_DIR)/$(APP_NAME)-cli_$${os}_$${arch}"; \
		echo "Building CLI $$os/$$arch..."; \
		GOOS=$$os GOARCH=$$arch go build -trimpath -ldflags="-s -w" -o $$out_cli ./cmd/cli; \
		tar -czf "$${out_cli}.tar.gz" -C $(DIST_DIR) "$(APP_NAME)-cli_$${os}_$${arch}"; \
		rm $$out_cli; \
	done
	@$(MAKE) package-desktop
	@echo "Archives written to $(DIST_DIR)/"

release:
	@read -p "New version: " version; \
	git commit --allow-empty -m "chore(release): bump version to v$$version"; \
	git tag -a "v$$version" -m "Release v$$version"; \
	git push origin main --tags; \
	$(MAKE) build-all; \
	gh release create "v$$version" $(DIST_DIR)/*.tar.gz $(DIST_DIR)/*.zip $(DIST_DIR)/*.dmg \
		--title "v$$version" \
		--generate-notes
