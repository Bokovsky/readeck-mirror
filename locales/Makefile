# SPDX-FileCopyrightText: © 2023 Olivier Meunier <olivier@neokraft.net>
#
# SPDX-License-Identifier: AGPL-3.0-only

ifneq ($(shell command -pv uv),)
PYRUN=$(shell command -v uv) run
else ifeq ($(shell python3 -c "import babel" 2>/dev/null && echo ok),ok)
PYRUN=$(shell command -v python3)
endif

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

# Compile po to mo files
compile:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py compile
else
	@echo "WARNING: python with babel not found, not compiling locales"
endif

# Check translations
check:
ifneq ($(PYRUN),)
	$(PYRUN) messages.py check
else
	@echo "ERROR: python with babel not found"
	@false
endif

# Remove .mo files
clean:
	rm -f translations/*/messages.mo
