# This Makefile mostly exists as an affordance for testing.
# We probably want to replace it with something else.
# Primarily, this just multiplexes commands across each subdirectory.
TOOLS_D := ./tools
STEREO := $(TOOLS_D)/stereo

ifeq (${TMPDIR}, )
	CACHEDIR = /tmp/melange-cache
else
	CACHEDIR = ${TMPDIR}/melange-cache
endif

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

.PHONY: cache
cache:
	mkdir -p ${CACHEDIR}

${CACHEDIR}/.libraries_token.txt: cache
	tmpf=$(shell mktemp); \
	chainctl auth login --audience libraries.cgr.dev; \
	chainctl auth token --audience libraries.cgr.dev > $${tmpf}; \
	mv $${tmpf} ${CACHEDIR}/.libraries_token.txt

.PHONY: lib-token
lib-token: ${CACHEDIR}/.libraries_token.txt
