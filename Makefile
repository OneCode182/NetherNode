SHELL := /bin/bash
MAKEFLAGS += --no-print-directory

COMPOSE_FILE ?= compose.yaml
ENV_FILE ?= .env
ENV_EXAMPLE ?= .env.example

.PHONY: help env up down stop-safe restart logs status minecraft-shell backup backup-dry-run restore restore-dry-run observability dns-update dns-update-dry-run validate bootstrap

help:
	@echo "NetherNode runtime tasks"
	@echo "  make up                 start runtime"
	@echo "  make down               stop runtime"
	@echo "  make stop-safe          save world and stop runtime"
	@echo "  make status             compose ps"
	@echo "  make logs               tail runtime logs"
	@echo "  make backup             run backup"
	@echo "  make backup-dry-run      run backup dry-run"
	@echo "  make restore ARCHIVE=... restore from archive"
	@echo "  make observability      run health + storage checks"
	@echo "  make dns-update-dry-run preview DuckDNS update"
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

dns-update:
	@bash ops/dns-update.sh

dns-update-dry-run:
	@bash ops/dns-update.sh --domain "$${DUCKDNS_DOMAIN:-nethernode}" --token "$${DUCKDNS_TOKEN:-dry-run-token}" --dry-run

validate:
	@cp -n "$(ENV_EXAMPLE)" "$(ENV_FILE)"
	@docker compose -f "$(COMPOSE_FILE)" config -q
	@if command -v shellcheck >/dev/null 2>&1; then \
	  shellcheck -x ops/*.sh; \
	else \
	  echo "shellcheck missing; skipping"; \
	fi
	@bash ops/observability.sh --dry-run
	@bash ops/backup.sh --dry-run
	@bash ops/stop-safe.sh --dry-run
