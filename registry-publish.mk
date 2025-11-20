# Registry Publishing Targets
#
# This file defines Makefile targets for publishing VM disk images to OCI registries.
# Unlike cloud-specific publishing (AWS AMIs, Azure images, etc.), these targets push
# VM artifacts to OCI registries like cgr.dev/chainguard-vms.
#
# How Publishing Works:
#    - Publishes one architecture at a time with automatic merging
#    - Merges new architecture into existing multi-arch OCI index
#    - Safe for CI/CD pipelines with parallel arch-specific runners
#    - Creates new index if none exists
#    - Automatically handles concurrent publishes from parallel runners
#
# Variables:
#   REGISTRY_REPO - Base repository path (default: cgr.dev/chainguard-vms)
#   BUILD_TIMESTAMP - Timestamp for version tags (auto-generated: YYYYMMDD-HHMM)
#   ARCH - Target architecture (x86_64 or aarch64)
#
# Examples:
#   make publish-registry-aws-base-slim                  # Publishes current ARCH
#   ARCH=aarch64 make publish-registry-azure-docker-full # Publishes aarch64
#   REGISTRY_REPO=cgr.dev/custom-org make publish-registry-qemu-base
#   BUILD_TIMESTAMP=20250115-1200 make publish-registry-gcp-nginx-full
#
# Parallel CI/CD Example:
#   Runner 1: ARCH=x86_64 make publish-registry-aws-base-slim
#   Runner 2: ARCH=aarch64 make publish-registry-aws-base-slim
#   Result: Multi-arch index with both x86_64 and aarch64

# Create phony targets for all configs
.PHONY: $(foreach name,$(names),publish-registry-$(name))

REGISTRY_REPO ?= cgr.dev/chainguard-vms

# Pattern rule: publish-registry-<config-name>
# Publishes the specified config's disk images for current ARCH.
# Automatically merges the new architecture into an existing multi-arch OCI index,
# or creates a new index if none exists.
$(foreach name,$(names),publish-registry-$(name)): publish-registry-%: $(ARCH_OUT_D)/%/disk.raw apkoaas
	./apkoaas publish \
		--config configs/$*/publish.yaml \
		--output-dir $(ARCH_OUT_D)/$* \
		--registry $(REGISTRY_REPO) \
		--architecture $(ARCH) \
		--timestamp $(BUILD_TIMESTAMP) \
		--sign-and-attest=true \
		--skip-if-exists
