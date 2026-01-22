# This Makefile mostly exists as an affordance for testing.
# We probably want to replace it with something else.
# Primarily, this just multiplexes commands across each subdirectory.
TOOLS_D := ./tools
STEREO := $(TOOLS_D)/stereo

ARCH ?= $(shell uname -m)
ifeq (${ARCH}, arm64)
	ARCH = aarch64
endif
PACKAGES_CONTAINER_FOLDER ?= /work/packages
DOCKER_PLATFORM_ARG := $(shell \
  case $(ARCH) in \
	(aarch64) darch=arm64;; \
	(x86_64) darch=amd64;; \
	(*) echo "unknown-docker-platform-arch-$(ARCH)"; exit 1;; \
  esac ; \
  echo "--platform=linux/$$darch" \
)

pkgs := $(shell $(STEREO) make targets)

pkg_targets = $(foreach name,$(pkgs),package/$(name))
$(pkg_targets): package/%: $(STEREO)
	$(STEREO) make package $*

test_targets = $(foreach name,$(pkgs),test/$(name))
$(test_targets): test/%: $(STEREO)
	$(STEREO) make test $*

debug_targets = $(foreach name,$(pkgs),debug/$(name))
$(debug_targets): debug/%: $(STEREO)
	$(STEREO) make debug $*

test_debug_targets = $(foreach name,$(pkgs),test-debug/$(name))
$(test_debug_targets): test-debug/%: $(STEREO)
	$(STEREO) make test-debug $*

compile_targets = $(foreach name,$(pkgs),compile/$(name))
$(compile_targets): compile/%: $(STEREO)
	@$(STEREO) make compile $*

package-list: $(STEREO)
	$(STEREO) make targets

# Archive Process Workflow Targets
.PHONY: deps-resolve deps-build deps-image deps-vm deps-seed
.PHONY: archive archive-generate withdraw validate-withdrawn
.PHONY: clean-archive archive-workflow-full package-list

# Step 1: Resolve all dependencies
deps-resolve: deps-build deps-image deps-vm deps-seed

deps-build: $(STEREO)
	@echo "Resolving build dependencies..."
	$(STEREO) build-dependencies

deps-image: $(STEREO)
	@echo "Resolving image dependencies (requires tfplan.json files)..."
	@if [ -f public-images.tfplan.json ]; then \
		cat public-images.tfplan.json | $(STEREO) image-dependencies; \
	else \
		echo "Warning: public-images.tfplan.json not found, skipping public images"; \
	fi
	@if [ -f private-images.tfplan.json ]; then \
		cat private-images.tfplan.json | $(STEREO) image-dependencies --private; \
	else \
		echo "Warning: private-images.tfplan.json not found, skipping private images"; \
	fi

deps-vm:
	@echo "Resolving VM dependencies..."
	$(STEREO) vm-dependencies

deps-seed:
	@echo "Resolving seed dependencies (optional)..."
	@if [ -f shrink/garbage-collection/archive-seeds.json ]; then \
		$(STEREO) seed-dependencies; \
	else \
		echo "Info: shrink/garbage-collection/archive-seeds.json not found, skipping seed dependencies"; \
	fi

# Step 2: Archive analysis
archive:
	@echo "Running archive analysis..."
	$(STEREO) archive --duration 365

archive-generate:
	@echo "Running archive analysis and generating withdrawn-packages.txt files..."
	$(STEREO) archive --generate-withdrawn --duration 365

# Step 3: Withdrawal process
withdraw:
	@echo "Creating modified APKINDEX files with withdrawn packages..."
	$(STEREO) withdraw --output-dir withdrawn-indexes

# Step 4: Validation testing
validate-withdrawn:
	@echo "Validating dependencies with withdrawn packages..."
	$(STEREO) build-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes
	@if [ -f public-images.tfplan.json ]; then \
		cat public-images.tfplan.json | $(STEREO) image-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi
	@if [ -f private-images.tfplan.json ]; then \
		cat private-images.tfplan.json | $(STEREO) image-dependencies --private --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi
	$(STEREO) vm-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes
	@if [ -f shrink/garbage-collection/archive-seeds.json ]; then \
		$(STEREO) seed-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi

# Complete workflow
archive-workflow-full:
	@echo "Running complete archive workflow..."
	$(MAKE) deps-resolve
	$(MAKE) archive-generate
	$(MAKE) withdraw
	$(MAKE) validate-withdrawn
	@echo "Archive workflow complete. Check withdrawn-test/ for validation results."

$(STEREO): $(wildcard cmd/stereo/*.go)
	@mkdir -p $$(dirname "$(STEREO)")
	go build -o $(STEREO) ./cmd/stereo/

# Cleanup
clean-archive:
	@echo "Cleaning archive-related outputs..."
	rm -rf archive retain resolved unresolved withdrawn-indexes withdrawn-test

.PHONY: clean
clean:
	make -C os clean
	make -C extra-packages clean
	make -C enterprise-packages clean
	$(MAKE) clean-archive

%.rsa:
	make -C $(dir $@) $(notdir $@)

.PHONY: local-wolfi
local-wolfi: os/local-melange.rsa enterprise-packages/local-melange-enterprise.rsa extra-packages/local-melange-extra.rsa
	@mkdir -p "$(PWD)/os/packages" "$(PWD)/enterprise-packages/packages" "$(PWD)/extra-packages/packages"
	@$(eval TMP_DIR := $(shell mktemp --tmpdir -d "$@.XXXXXX"))
	@$(eval TMP_REPOS_FILE := $(TMP_DIR)/repositories)
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOS_FILE)
	@echo "https://apk.cgr.dev/chainguard-private" >> $(TMP_REPOS_FILE)
	@echo "https://packages.cgr.dev/extras" >> $(TMP_REPOS_FILE)
	@(for p in os enterprise-packages extra-packages ; do \
		[ -f "$$p/packages/$(ARCH)/APKINDEX.tar.gz" ] || continue ; \
		echo "$(PACKAGES_CONTAINER_FOLDER)/$$p"; done ) >> $(TMP_REPOS_FILE)
	@trap 'rm -Rf "$(TMP_DIR)"' EXIT && \
	  tok=$$(chainctl auth token --audience=apk.cgr.dev) && \
	  ( umask 066 && printf "%s\n" "machine apk.cgr.dev" "login token" "password $$tok" > "$(TMP_DIR)/netrc" ) && \
	docker run $(DOCKER_PLATFORM_ARG) --pull=always --rm -it \
		--entrypoint="/bin/sh" \
		--mount type=bind,source="$(PWD)/os/packages",destination="$(PACKAGES_CONTAINER_FOLDER)/os",readonly \
		--mount type=bind,source="$(PWD)/enterprise-packages/packages",destination="$(PACKAGES_CONTAINER_FOLDER)/enterprise-packages",readonly \
		--mount type=bind,source="$(PWD)/extra-packages/packages",destination="$(PACKAGES_CONTAINER_FOLDER)/extra-packages",readonly \
		--mount type=bind,source="$(PWD)/os/local-melange.rsa.pub",destination="/etc/apk/keys/local-melange.rsa.pub",readonly \
		--mount type=bind,source="$(PWD)/enterprise-packages/local-melange-enterprise.rsa.pub",destination="/etc/apk/keys/local-melange-enterprise.rsa.pub",readonly \
		--mount type=bind,source="$(PWD)/extra-packages/local-melange-extra.rsa.pub",destination="/etc/apk/keys/local-melange-extra.rsa.pub",readonly \
		--mount type=bind,source="$(PWD)/extra-packages/chainguard-extras.rsa.pub",destination="/etc/apk/keys/chainguard-extras.rsa.pub,readonly" \
		--mount type=bind,source="$(TMP_REPOS_FILE)",destination="/etc/apk/repositories",readonly \
		--mount type=bind,source="$(TMP_DIR)/netrc",destination="/root/.netrc",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard/wolfi-base:latest -il
