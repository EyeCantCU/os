# Registry Publishing Targets
#
# This file defines Makefile targets for publishing VM disk images to OCI registries.
# Unlike cloud-specific publishing (AWS AMIs, Azure images, etc.), these targets push
# VM artifacts to OCI registries like cgr.dev/chainguard-vms.
#
# Publishing Modes:
#
# 1. Individual arch:
#    - Publishes one architecture at a time with --merge flag
#    - Merges new architecture into existing multi-arch OCI index
#    - Safe for CI/CD pipelines with separate arch-specific runners
#    - Example: Build x86_64 on one runner, aarch64 on another
#
# 2. Multi arch:
#    - Creates fresh multi-arch OCI index with all architectures
#    - Useful for local development or when both archs built sequentially
#    - Requires all architectures to be built before publishing
#
# Variables:
#   REGISTRY_REPO - Base repository path (default: cgr.dev/chainguard-vms)
#   BUILD_TIMESTAMP - Timestamp for version tags (auto-generated: YYYYMMDD-HHMM)
#   ARCH - Target architecture for incremental mode (x86_64 or aarch64)
#
# Examples (Individual arch):
#   make publish-registry-aws-base-slim                  # Merges current ARCH into index
#   ARCH=aarch64 make publish-registry-azure-docker-full # Merges aarch64 into index
#   REGISTRY_REPO=cgr.dev/custom-org make publish-registry-qemu-base
#   BUILD_TIMESTAMP=20250115-1200 make publish-registry-gcp-nginx-full
#
# Examples (Multi arch):
#   make publish-registry-multi-aws-base-slim    # Builds both archs, publishes together
#   make publish-registry-multi-qemu-base        # Fresh multi-arch index

# Create phony targets for all configs (both incremental and batch modes)
.PHONY: $(foreach name,$(names),publish-registry-$(name))
.PHONY: $(foreach name,$(names),publish-registry-multi-$(name))

REGISTRY_REPO ?= cgr.dev/chainguard-vms

# Pattern rule: publish-registry-<config-name>
# Publishes the specified config's disk images for current ARCH with --merge flag.
# This merges the new architecture into an existing multi-arch OCI index, or creates
# a new index if none exists.
$(foreach name,$(names),publish-registry-$(name)): publish-registry-%: $(ARCH_OUT_D)/%/disk.raw apkoaas
	./apkoaas publish \
		--config configs/$*/publish.yaml \
		--output-dir $(ARCH_OUT_D)/$* \
		--registry $(REGISTRY_REPO) \
		--architectures $(ARCH) \
		--timestamp $(BUILD_TIMESTAMP) \
		--sign-and-attest=true \
		--skip-if-exists \
		--merge

# Pattern rule: publish-registry-multi-<config-name>
# Builds both x86_64 and aarch64 architectures, then publishes them together
# as a multi-arch OCI index.
$(foreach name,$(names),publish-registry-multi-$(name)): publish-registry-multi-%: apkoaas
	@echo "Publishing multi-arch index for $*..."
	./apkoaas publish \
		--config configs/$*/publish.yaml \
		--output-dir output/x86_64/$* \
		--registry $(REGISTRY_REPO) \
		--architectures x86_64,aarch64 \
		--timestamp $(BUILD_TIMESTAMP) \
		--sign-and-attest=true \
		--skip-if-exists
