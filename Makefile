# This Makefile mostly exists as an affordance for testing.
# We probably want to replace it with something else.
# Primarily, this just multiplexes commands across each subdirectory.
pkgs := $(shell stereo make targets)

pkg_targets = $(foreach name,$(pkgs),package/$(name))
$(pkg_targets): package/%:
	stereo make package $*

test_targets = $(foreach name,$(pkgs),test/$(name))
$(test_targets): test/%:
	stereo make test $*

debug_targets = $(foreach name,$(pkgs),debug/$(name))
$(debug_targets): debug/%:
	stereo make debug $*

test_debug_targets = $(foreach name,$(pkgs),test-debug/$(name))
$(test_debug_targets): test-debug/%:
	stereo make test-debug $*

compile_targets = $(foreach name,$(pkgs),compile/$(name))
$(compile_targets): compile/%:
	@stereo make compile $*

# Archive Process Workflow Targets
.PHONY: deps-resolve deps-build deps-image deps-vm deps-seed deps-version-streams
.PHONY: archive archive-generate withdraw validate-withdrawn
.PHONY: clean-archive archive-workflow-full

# Step 1: Resolve all dependencies
deps-resolve: deps-build deps-image deps-vm deps-seed deps-version-streams

deps-build:
	@echo "Resolving build dependencies..."
	stereo build-dependencies

deps-image:
	@echo "Resolving image dependencies (requires tfplan.json files)..."
	@if [ -f public-images.tfplan.json ]; then \
		cat public-images.tfplan.json | stereo image-dependencies; \
	else \
		echo "Warning: public-images.tfplan.json not found, skipping public images"; \
	fi
	@if [ -f private-images.tfplan.json ]; then \
		cat private-images.tfplan.json | stereo image-dependencies --private; \
	else \
		echo "Warning: private-images.tfplan.json not found, skipping private images"; \
	fi

deps-vm:
	@echo "Resolving VM dependencies..."
	stereo vm-dependencies

deps-seed:
	@echo "Resolving seed dependencies (optional)..."
	@if [ -f archive-seeds.json ]; then \
		stereo seed-dependencies; \
	else \
		echo "Info: archive-seeds.json not found, skipping seed dependencies"; \
	fi

deps-version-streams:
	@echo "Resolving version-stream dependencies (optional)..."
	@if [ -d package-version-metadata ]; then \
		stereo version-stream-dependencies; \
	else \
		echo "Info: package-version-metadata directory not found, skipping version-stream dependencies"; \
	fi

# Step 2: Archive analysis
archive:
	@echo "Running archive analysis..."
	stereo archive --duration 365

archive-generate:
	@echo "Running archive analysis and generating withdrawn-packages.txt files..."
	stereo archive --generate-withdrawn --duration 365

# Step 3: Withdrawal process
withdraw:
	@echo "Creating modified APKINDEX files with withdrawn packages..."
	stereo withdraw --output-dir withdrawn-indexes

# Step 4: Validation testing
validate-withdrawn:
	@echo "Validating dependencies with withdrawn packages..."
	stereo build-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes
	@if [ -f public-images.tfplan.json ]; then \
		cat public-images.tfplan.json | stereo image-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi
	@if [ -f private-images.tfplan.json ]; then \
		cat private-images.tfplan.json | stereo image-dependencies --private --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi
	stereo vm-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes
	@if [ -f archive-seeds.json ]; then \
		stereo seed-dependencies --use-withdrawn --withdrawn-dir withdrawn-indexes; \
	fi

# Complete workflow
archive-workflow-full:
	@echo "Running complete archive workflow..."
	$(MAKE) deps-resolve
	$(MAKE) archive-generate
	$(MAKE) withdraw
	$(MAKE) validate-withdrawn
	@echo "Archive workflow complete. Check withdrawn-test/ for validation results."

# Cleanup
clean-archive:
	@echo "Cleaning archive-related outputs..."
	rm -rf archive retain resolved unresolved withdrawn-indexes withdrawn-test
	rm -f os/withdrawn-packages.txt extra-packages/withdrawn-packages.txt enterprise-packages/withdrawn-packages.txt

.PHONY: clean
clean:
	make -C os clean
	make -C extra-packages clean
	make -C enterprise-packages clean
	$(MAKE) clean-archive
