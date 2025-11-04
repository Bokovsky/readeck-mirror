# SPDX-FileCopyrightText: © 2025 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

ifneq ($(shell command -v uv),)
PYRUN=$(shell command -v uv) run
else ifeq ($(shell python3 -c "import babel" 2>/dev/null && echo ok),ok)
PYRUN=$(shell command -v python3)
endif

GO ?= go

all: compile

# Extract all the messages
extract:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py extract
else
	@echo "ERROR: python with babel not found"
	@false
endif

# Update po files
update:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py update
else
	@echo "ERROR: python with babel not found"
	@false
endif

# Generate markdown from po files.
generate:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py generate
else
	@echo "WARNING: python with babel not found, not translating docs"
endif

check:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py check
else
	@echo "ERROR: python with babel not found"
	@false
endif

build: generate
	$(GO) run ../tools/docs src assets
	$(GO) run ../tools/yaml-compose api/api.yaml assets/api.json

clean:
	rm -rf assets/*
	find src -name '*.md' -not -regex "src/en-US/.*" -delete
