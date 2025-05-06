ARCH ?= $(shell uname -m)
ifeq (${ARCH}, arm64)
	ARCH = aarch64
endif
ifeq (${TMPDIR}, )
        CACHEDIR = /tmp/melange-cache
else
        CACHEDIR = ${TMPDIR}/melange-cache
endif
TARGETDIR = packages/${ARCH}

MELANGE ?= $(shell which melange)
WOLFICTL ?= $(shell which wolfictl)
KEY ?= local-melange-extra.rsa
REPO ?= $(shell pwd)/packages
QEMU_KERNEL_REPO := https://apk.cgr.dev/chainguard-private/

ifneq (${MELANGE_RUNNER},)
	MELANGE_OPTS += --runner ${MELANGE_RUNNER}
endif
QEMU_KERNEL_IMAGE ?= kernel/boot/vmlinuz
ifeq (${MELANGE_RUNNER},qemu)
	QEMU_KERNEL_DEP = ${QEMU_KERNEL_IMAGE}
	export QEMU_KERNEL_IMAGE
endif

MELANGE_OPTS += --repository-append ${REPO}
MELANGE_OPTS += --keyring-append ${KEY}.pub
MELANGE_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_OPTS += --signing-key ${KEY}
MELANGE_OPTS += --arch ${ARCH}
MELANGE_OPTS += --license 'Apache-2.0'
MELANGE_OPTS += --git-repo-url 'https://github.com/chainguard-dev/extra-packages'
MELANGE_OPTS += --env-file build-${ARCH}.env
MELANGE_OPTS += --namespace chainguard
MELANGE_OPTS += --generate-index false
MELANGE_OPTS += --cache-dir ${CACHEDIR}
MELANGE_OPTS += --pipeline-dir ./pipelines/
MELANGE_OPTS += ${MELANGE_EXTRA_OPTS}

ifeq (${LINT}, yes)
	MELANGE_OPTS += --fail-on-lint-warning
endif

EXTRAS_REPO ?= https://apk.cgr.dev/extra-packages

# Add the extras repository to the list of repositories
MELANGE_OPTS += -r ${EXTRAS_REPO}

# Enter interactive mode on failure for debug
MELANGE_DEBUG_OPTS += --interactive
MELANGE_DEBUG_OPTS += --debug
MELANGE_DEBUG_OPTS += --package-append apk-tools
MELANGE_DEBUG_OPTS += ${MELANGE_OPTS}

# Enter interactive mode on test failure for debug
MELANGE_DEBUG_TEST_OPTS += --interactive

# These are separate from MELANGE_OPTS because for building we need additional
# ones that are not defined for tests.
MELANGE_TEST_OPTS += --repository-append ${REPO}
MELANGE_TEST_OPTS += --keyring-append ${KEY}.pub
MELANGE_TEST_OPTS += --arch ${ARCH}
MELANGE_TEST_OPTS += --pipeline-dirs ./pipelines/
MELANGE_TEST_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_TEST_OPTS += --repository-append ${EXTRAS_REPO}
MELANGE_TEST_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_TEST_OPTS += --test-package-append wolfi-base
MELANGE_TEST_OPTS += --debug
MELANGE_TEST_OPTS += ${MELANGE_EXTRA_OPTS}

${KEY}:
	${MELANGE} keygen ${KEY}

.PHONY: cache
cache:
	mkdir -p ${CACHEDIR}

.PHONY: clean
clean:
	rm -rf packages/${ARCH}
	rm -rf kernel/

.PHONY: clean-cache
clean-cache:
	rm -rf ${CACHEDIR}

.PHONY: fetch-kernel
fetch-kernel:
	rm -rf kernel
	$(MAKE) kernel/boot/vmlinuz

kernel/APKINDEX.tar.gz:
	@$(call authget,apk.cgr.dev,$@,$(QEMU_KERNEL_REPO)/$(ARCH)/APKINDEX.tar.gz)

kernel/APKINDEX: kernel/APKINDEX.tar.gz
	tar -x -C kernel -f $< $(notdir $@)
	touch $@

kernel/chosen: kernel/APKINDEX
	# Extract lines with 'P:linux' and the following line that contains the version
	# This approach is compatible with both GNU and BSD sed
	awk '/^P:linux$$/ {print; getline; print}' $< > kernel/available
	grep '^V:' kernel/available | sed 's/V://' | \
	  sort -V | tail -n1 > $@.tmp
	# Sanity check that this looks like an apk version
	grep -E '^([0-9]+\.)+[0-9]+-r[0-9]+$$' $@.tmp
	mv $@.tmp $@

kernel/linux.apk: kernel/chosen
	@$(call authget,apk.cgr.dev,$@,$(QEMU_KERNEL_REPO)/$(ARCH)/linux-$(shell cat kernel/chosen).apk)

kernel/boot/vmlinuz: kernel/linux.apk
	tar -x -C kernel -f $< boot/ 2> /dev/null
	touch $@

${CACHEDIR}/.libraries_token.txt: cache
	tmpf=$(shell mktemp); \
	chainctl auth login --audience libraries.cgr.dev; \
	chainctl auth token --audience libraries.cgr.dev > $${tmpf}; \
	mv $${tmpf} ${CACHEDIR}/.libraries_token.txt

.PHONY: lib-token
lib-token: ${CACHEDIR}/.libraries_token.txt

yamls := $(wildcard *.yaml)
pkgs := $(subst .yaml,,$(yamls))
pkg_targets = $(foreach name,$(pkgs),package/$(name))
$(pkg_targets): package/%:
	$(eval yamlfile := $*.yaml)
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(info pkgver $(pkgver))
	$(MAKE) yamlfile=$(yamlfile) pkgname=$* packages/$(ARCH)/$(pkgver).apk

dbg_targets = $(foreach name,$(pkgs),debug/$(name))
$(dbg_targets): debug/%: cache $(KEY) $(QEMU_KERNEL_DEP)
	$(eval yamlfile := $*.yaml)
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Building package $* with version $(pkgver) from file $(yamlfile)\n"
	mkdir -p ./"$*"/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	$(info @SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS))
	SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_DEBUG_OPTS)  --source-dir ./$(*)/

test_targets = $(foreach name,$(pkgs),test/$(name))
$(test_targets): test/%: cache $(KEY)
	mkdir -p ./$(*)/
	$(eval yamlfile := $*.yaml)
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	$(MELANGE) test $(yamlfile) $(MELANGE_TEST_OPTS) --source-dir ./$(*)/

testdbg_targets = $(foreach name,$(pkgs),test-debug/$(name))
$(testdbg_targets): test-debug/%: cache $(KEY)
	mkdir -p ./$(*)/
	$(eval yamlfile := $*.yaml)
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	$(MELANGE) test $(yamlfile) $(MELANGE_TEST_OPTS) $(MELANGE_DEBUG_TEST_OPTS) --source-dir ./$(*)/

packages/$(ARCH)/%.apk: cache $(KEY) $(QEMU_KERNEL_DEP)
	mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS) --source-dir ./$(pkgname)/

.PHONY: dev-container
dev-container:
	docker run --pull=always --privileged --rm -it \
	    -v "${PWD}:${PWD}" \
	    -w "${PWD}" \
	    -e SOURCE_DATE_EPOCH=0 \
	    ghcr.io/wolfi-dev/sdk:latest

PACKAGES_CONTAINER_FOLDER ?= /work/packages
TMP_REPOSITORIES_DIR := $(shell mktemp -d)
TMP_REPOSITORIES_FILE := $(TMP_REPOSITORIES_DIR)/repositories
# This target spins up a docker container that is helpful for testing local
# changes to the packages. It mounts the local packages folder as a read-only,
# and sets up the necessary keys for you to run `apk add` commands, and then
# test the packages however you see fit.
.PHONY: local-wolfi
local-wolfi: $(KEY)
	echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	echo "https://apk.cgr.dev/extra-packages" >> $(TMP_REPOSITORIES_FILE)
	echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	mkdir -p ${PWD}/packages
	docker run --pull=always --rm -it \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange.rsa.pub",destination="/etc/apk/keys/local-melange.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard-private/chainguard-base:latest
	rm "$(TMP_REPOSITORIES_FILE)"
	rmdir "$(TMP_REPOSITORIES_DIR)"

authget = tok=$$(chainctl auth token --audience=$(1)) || \
  { echo "failed token from $(1) for target $@"; exit 1; }; \
  mkdir -p $$(dirname $(2)) && \
  echo "auth-download[$(1)] to $(2) from $(3)" && \
  curl -LS --silent -o $(2).tmp --user "user:$$tok" $(3) && \
	mv "$(2).tmp" "$(2)"
