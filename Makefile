# This Source Code Form is subject to the terms of the Mozilla Public
# License, version 2.0. If a copy of the MPL was not distributed with this
# file, You can obtain one at http://mozilla.org/MPL/2.0/.

GHACCOUNT := johansundell
NAME := angel
VERSION := v0.0.8

build:
	go build -ldflags "-X 'main.Version=$(VERSION)'"

compile:
	@rm -rf build/
	@gox -ldflags "-X 'main.Version=$(VERSION)'" \
	-osarch="darwin/amd64" \
	-osarch="linux/amd64" \
	-osarch="linux/arm" \
	-osarch="linux/arm64" \
	-os="windows" \
	-output "build/{{.Dir}}_$(VERSION)_{{.OS}}_{{.Arch}}/$(NAME)" \
	./...
	@for d in build/*/; do cp THIRD_PARTY_LICENSES.txt "$$d"; done

# Regenerates THIRD_PARTY_LICENSES.txt; run after changing dependencies.
licenses:
	./scripts/third-party-licenses.sh

install:
	go install -ldflags "-X main.Version=$(VERSION)"

# Installs the release tools into $(go env GOPATH)/bin (or GOBIN), which must
# be on PATH for compile, dist and release. go get no longer installs binaries.
deps:
	go install github.com/c4milo/github-release@latest
	go install github.com/mitchellh/gox@latest

dist: compile
	$(eval FILES := $(shell ls build))
	@rm -rf dist && mkdir dist
	@for f in $(FILES); do \
		(cd $(shell pwd)/build/$$f && tar -cvzf ../../dist/$$f.tar.gz *); \
		(cd $(shell pwd)/dist && shasum -a 512 $$f.tar.gz > $$f.sha512); \
		echo $$f; \
	done

docker:
	docker build --build-arg VERSION=$(VERSION) -t $(GHACCOUNT)/$(NAME):$(VERSION) .

docker-push: docker
	docker push $(GHACCOUNT)/$(NAME):$(VERSION)

docker-run:
	VERSION=$(VERSION) docker compose up -d --build

# Uses ./.env and the SQLite database in ./data (see docker-compose.local.yml).
docker-run-local:
	@test -f .env || { echo ".env not found: copy ENV_BASE to .env and fill it in" >&2; exit 1; }
	mkdir -p $${LOCAL_DATA_DIR:-data}
	VERSION=$(VERSION) LOCAL_UID=$$(id -u) LOCAL_GID=$$(id -g) docker compose -f docker-compose.local.yml up -d --build

release: dist docker-push
	@latest_tag=$$(git describe --tags `git rev-list --tags --max-count=1`); \
	comparison="$$latest_tag..HEAD"; \
	if [ -z "$$latest_tag" ]; then comparison=""; fi; \
	changelog=$$(git log $$comparison --oneline --no-merges); \
	github-release $(GHACCOUNT)/$(NAME) $(VERSION) "$$(git rev-parse --abbrev-ref HEAD)" "**Changelog**<br/>$$changelog" 'dist/*'; \
	git pull

.PHONY: build compile licenses install deps dist release docker docker-push docker-run docker-run-local
