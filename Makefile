SHELL := /bin/bash
MAKEFLAGS += --no-print-directory

COMPOSE_FILE ?= compose.yaml
ENV_FILE ?= .env
ENV_EXAMPLE ?= .env.example

.PHONY: help env up down stop-safe restart logs status minecraft-shell backup backup-dry-run install-cli restore restore-dry-run observability dns-update dns-update-dry-run plugins-sync plugins-sync-dry-run plugins-list controller-test validate bootstrap

help:
	@echo "NetherNode runtime tasks"
	@echo "  make up                 start runtime"
	@echo "  make down               stop runtime"
	@echo "  make stop-safe          save world and stop runtime"
	@echo "  make status             compose ps"
	@echo "  make logs               tail runtime logs"
	@echo "  make backup             run backup"
	@echo "  make backup-dry-run      run backup dry-run"
	@echo "  make install-cli        install nethernode CLI on host"
	@echo "  make restore ARCHIVE=... restore from archive"
	@echo "  make observability      run health + storage checks"
	@echo "  make plugins-sync       sync managed crossplay plugins"
	@echo "  make plugins-sync-dry-run preview plugin sync"
	@echo "  make plugins-list       list managed plugins"
	@echo "  make dns-update-dry-run preview DuckDNS update"
	@echo "  make controller-test     run cloud controller mock tests"
	@echo "  make validate           run compose + script checks"

env:
	@if [[ ! -f "$(ENV_FILE)" ]]; then \
	  cp "$(ENV_EXAMPLE)" "$(ENV_FILE)"; \
	  echo "created $(ENV_FILE) from $(ENV_EXAMPLE)"; \
	else \
	  echo "$(ENV_FILE) already exists"; \
	fi

bootstrap: env
	@mkdir -p data/minecraft ops/observability
	@mkdir -p backups

up: bootstrap env
	@bash ops/start.sh

down:
	@docker compose -f "$(COMPOSE_FILE)" down

stop-safe:
	@bash ops/stop-safe.sh

restart: down up

status:
	@docker compose -f "$(COMPOSE_FILE)" ps

logs:
	@docker compose -f "$(COMPOSE_FILE)" logs -f

minecraft-shell:
	@docker compose -f "$(COMPOSE_FILE)" exec minecraft sh

backup:
	@bash ops/backup.sh

backup-dry-run:
	@bash ops/backup.sh --dry-run

install-cli:
	@sudo bash ops/install-server-cli.sh

restore:
	@if [[ -z "$(ARCHIVE)" ]]; then \
	  echo "Usage: make restore ARCHIVE=<path>"; \
	  exit 1; \
	fi
	@bash ops/restore.sh --archive "$(ARCHIVE)"

restore-dry-run:
	@if [[ -z "$(ARCHIVE)" ]]; then \
	  echo "Usage: make restore-dry-run ARCHIVE=<path>"; \
	  exit 1; \
	fi
	@bash ops/restore.sh --archive "$(ARCHIVE)" --dry-run

observability:
	@bash ops/observability.sh

plugins-sync:
	@bash ops/plugins-sync.sh

plugins-sync-dry-run:
	@bash ops/plugins-sync.sh --dry-run

plugins-list:
	@bash ops/plugins-sync.sh --list

dns-update:
	@bash ops/dns-update.sh

dns-update-dry-run:
	@bash ops/dns-update.sh --domain "$${DUCKDNS_DOMAIN:-nethernode}" --token "$${DUCKDNS_TOKEN:-dry-run-token}" --dry-run

controller-test:
	@bash scripts/tests/nethernode-controller-test.sh

validate:
	@cp -n "$(ENV_EXAMPLE)" "$(ENV_FILE)"
	@docker compose -f "$(COMPOSE_FILE)" config -q
	@if command -v shellcheck >/dev/null 2>&1; then \
	  shellcheck -x ops/*.sh ops/nethernode; \
	else \
	  echo "shellcheck missing; skipping"; \
	fi
	@bash -n ops/nethernode
	@bash ops/save-server.sh --help >/dev/null
	@bash ops/backup-server.sh --help >/dev/null
	@bash ops/nethernode help >/dev/null
	@bash ops/plugins-sync.sh --help >/dev/null
	@bash ops/check-ci-no-reset.sh >/dev/null
	@NETHERNODE_SCRIPT_DIR=ops bash ops/nethernode plugins list >/dev/null
	@bash ops/observability.sh --dry-run
	@bash ops/backup.sh --dry-run
	@bash ops/stop-safe.sh --dry-run
	@$(MAKE) controller-test
