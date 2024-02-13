ARCH ?= $(shell uname -m)
ifeq (${ARCH}, arm64)
	ARCH = aarch64
else ifeq (${ARCH}, amd64)
	ARCH = x86_64
endif
TARGETDIR = packages/${ARCH}

MELANGE ?= $(shell which melange)
WOLFICTL ?= $(shell which wolfictl)
KEY ?= local-melange-enterprise.rsa
REPO ?= $(shell pwd)/packages
GCS_FETCH_BUCKET_NAME ?= gs://chainguard-enterprise-registry-destination/os/

MELANGE_OPTS += --repository-append ${REPO}
MELANGE_OPTS += --keyring-append ${KEY}.pub
MELANGE_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_OPTS += --signing-key ${KEY}
MELANGE_OPTS += --pipeline-dir ./pipelines/
MELANGE_OPTS += --arch ${ARCH}
MELANGE_OPTS += --env-file build-${ARCH}.env
MELANGE_OPTS += --namespace chainguard
MELANGE_OPTS += ${MELANGE_EXTRA_OPTS}

# These are separate from MELANGE_OPTS because for building we need additional
# ones that are not defined for tests.
MELANGE_TEST_OPTS += --repository-append ${REPO}
MELANGE_TEST_OPTS += --keyring-append ${KEY}.pub
MELANGE_TEST_OPTS += --arch ${ARCH}
MELANGE_TEST_OPTS += --pipeline-dirs ./pipelines/
MELANGE_TEST_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_TEST_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_TEST_OPTS += ${MELANGE_EXTRA_OPTS}

# The list of packages to be built. The order matters.
# wolfictl determines the list and order
# set only to be called when needed, so make can be instant to run
# when it is not
PKGLISTCMD ?= $(WOLFICTL) text --dir . --type name

all: ${KEY} .build-packages

# this ensures two things:
# 1. We only generate the graph for the list of commands that requires it
# 2. If generating the graph fails, we error out; without this, a failure in $(shell) might go unnoticed.
ifneq ($(findstring $(MAKECMDGOALS),all list list-yaml),)
  PKGNAMES := $(shell $(PKGLISTCMD) || echo "failed")
  ifeq ($(PKGNAMES),failed)
    $(error $(PKGLISTCMD) failed)
  endif
  PKGLIST := $(addprefix package/,$(PKGNAMES))
else
  PKGLIST :=
endif
.build-packages: $(PKGLIST)

${KEY}:
	${MELANGE} keygen ${KEY}

clean:
	rm -rf packages/${ARCH}

.PHONY: list list-yaml

list:
	$(info $(PKGNAMES))
	@printf ''

list-yaml:
	$(info $(addsuffix .yaml,$(PKGNAMES)))
	@printf ''

# This function parses the path from the package file. It's used to figure out
# what to mount to the container image as supporting files (patches, tests,
# etc.)
# Returns the directory of the package in the first variable passed in. In
# example below this would be ret-variable-in-calling-function. You do not need
# to explicitly declare this variable in the calling function, just add to
# argument list and it will be populated and usable.
#
# $(call get-package-dir,ret-variable-in-calling-function,package-file)
define get-package-dir
	$(info getting package dir for $(2))
	$(eval pkgdir := $(shell dirname $(2)))
	$(info For package $(1) found dir: $(pkgdir))
	$(1) := ${pkgdir}
endef

# This function tries to figure out what the 'source-dir' is for the package.
# It's complicated by the fact that it can either be './<package-name>' for
# packages before the refactoring, or it can be a relative path
# './<module>/package/', and in some cases it may not exist.
# To make it easier on the caller, it returns the entire:
# `--source-dir ./<package-name>`, or `--source-dir ./<module>/package/`, or ""
# as the first variable passed in, and this is meant to be directly passed
# to the melange build/test command.
#
#$(call get-source-dir,ret-variable-for-source-dir,package-dir,package-name)
define get-source-dir
	$(info getting source dir for package $(3) with dir $(2))
	$(1) := $(shell set -x; if [[ "." == "$(2)" ]]; then \
		echo "--source-dir ./$(3)"; \
	else \
		echo "--source-dir $(2)"; \
	fi)
endef

package/%:
	$(eval yamlfile := $(shell find . -type f \( -name "$*.yaml" -o -path "*/$*/$*.melange.yaml" \) | head -n 1))
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval $(call get-package-dir,pkgdir,$(yamlfile)))
	$(info found package dir as $(pkgdir))
	$(eval $(call get-source-dir,sourcedir,$(pkgdir),$*))
	$(info found source dir as $(sourcedir))
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(info pkgver $(pkgver))
	$(MAKE) yamlfile=$(yamlfile) srcdirflag="$(sourcedir)" pkgname=$* packages/$(ARCH)/$(pkgver).apk

packages/$(ARCH)/%.apk: $(KEY)
	@mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS) $(srcdirflag) --log-policy builtin:stderr,$(TARGETDIR)/buildlogs/$*.log

test/%:
	$(eval yamlfile := $(shell find . -type f \( -name "$*.yaml" -o -path "*/$*/$*.melange.yaml" \) | head -n 1))
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval $(call get-package-dir,pkgdir,$(yamlfile)))
	$(info found package dir as $(pkgdir))
	$(eval $(call get-source-dir,sourcedir,$(pkgdir),$*))
	$(info found source dir as $(sourcedir))
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	$(MELANGE) test $(yamlfile) $(sourcedir) $(MELANGE_TEST_OPTS) --log-policy builtin:stderr

dev-container:
	docker run --privileged --rm -it \
			-v "${PWD}:${PWD}" \
			-v "${HOME}/.cache/wolfictl/dev-container-enterprise/root:/root" \
			-w "${PWD}" \
			ghcr.io/wolfi-dev/sdk:latest@sha256:e3b083572a1bccad1929f400bc78941abbb5d1a062396b756d6b881017ee5008

# The next two targets are mostly copies from the local-wolfi and
# dev-container-wolfi targets from wolfi-dev/os:
# https://github.com/wolfi-dev/os/blob/main/Makefile

PACKAGES_CONTAINER_FOLDER ?= /work/packages
TMP_REPOSITORIES_DIR := $(shell mktemp -d)
TMP_REPOSITORIES_FILE := $(TMP_REPOSITORIES_DIR)/repositories
# This target spins up a docker container that is helpful for testing local
# changes to the packages. It mounts the local packages folder as a read-only,
# and sets up the necessary keys for you to run `apk add` commands, and then
# test the packages however you see fit.
local-wolfi:
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	@echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	docker run --rm -it \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange-enterprise.rsa.pub",destination="/etc/apk/keys/local-melange-enterprise.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard/wolfi-base:latest
	@rm "$(TMP_REPOSITORIES_FILE)"
	@rmdir "$(TMP_REPOSITORIES_DIR)"

# This target spins up a docker container that is helpful for building images
# using local packages.
# It mounts the:
#  - local packages dir (default: pwd) as a read-only, as /work/packages. This
#    is where the local packages are set up to be fetched from.
#  - local os dir (default: pwd) as a read-only, as /work/os. This is where
#    apko config files should live in. Note that this can be the current
#    directory also.
# Both of these can be overridden with PACKAGES_CONTAINER_FOLDER and OS_DIR
# respectively.
# It sets up the necessary tools, keys, and repositories for you to run
# apko to build images and then test them. Currently, the apko tool requires a
# few flags to get the image built, but we'll work on getting the viper config
# set up to make this easier.
#
# The resulting image will be in the OUT_DIR, and it is best to specify the
# OUT_DIR as a directory in the host system, so that it will persist after the
# container is done, as well as you can test / iterate with the image and run
# tests in the host.
#
# Example invocation for
# mkdir /tmp/out && OUT_DIR=/tmp/out make dev-container-wolfi
# Then in the container, you could build an image like this:
# apko -C /work/out build --keyring-append /etc/apk/keys/wolfi-signing.rsa.pub \
#  --keyring-append /etc/apk/keys/local-melange.rsa.pub --arch host \
# /work/os/conda-IMAGE.yaml conda-test:test /work/out/conda-test.tar
#
# Then from the host you can run:
# docker load -i /tmp/out/conda-test.tar
# docker run -it
OUT_LOCAL_DIR ?= /work/out
OUT_DIR ?= $(shell mktemp -d)
OS_LOCAL_DIR ?= /work/os
OS_DIR ?= ${PWD}
dev-container-wolfi:
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	@echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	docker run --rm -it \
		--mount type=bind,source="${OUT_DIR}",destination="$(OUT_LOCAL_DIR)" \
		--mount type=bind,source="${OS_DIR}",destination="$(OS_LOCAL_DIR)",readonly \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange-enterprise.rsa.pub",destination="/etc/apk/keys/local-melange-enterprise.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		ghcr.io/wolfi-dev/sdk:latest@sha256:e3b083572a1bccad1929f400bc78941abbb5d1a062396b756d6b881017ee5008
	@rm "$(TMP_REPOSITORIES_FILE)"
	@rmdir "$(TMP_REPOSITORIES_DIR)"

.PHONY: fetch-baselayout
fetch-baselayout:
	echo "Fetching baselayout from GCS..." && \
		mkdir -p ./packages/x86_64/ && \
		mkdir -p ./packages/aarch64/ && \
		gsutil cp $(GCS_FETCH_BUCKET_NAME)chainguard-enterprise.rsa.pub ./packages/ && \
        gsutil cp $(GCS_FETCH_BUCKET_NAME)x86_64/APKINDEX.tar.gz ./packages/x86_64/ && \
        gsutil -m cp -n $(GCS_FETCH_BUCKET_NAME)x86_64/chainguard-baselayout-* ./packages/x86_64/ && \
        gsutil cp $(GCS_FETCH_BUCKET_NAME)aarch64/APKINDEX.tar.gz ./packages/aarch64/ && \
        gsutil -m cp -n $(GCS_FETCH_BUCKET_NAME)aarch64/chainguard-baselayout-* ./packages/aarch64/

SINGLE_PACKAGE ?= unknown

.PHONY: fetch-single-package
fetch-single-package: fetch-baselayout
	echo "Fetching single package from GCS..." && \
		mkdir -p ./packages/x86_64/ && \
		mkdir -p ./packages/aarch64/ && \
        gsutil -m cp -n $(GCS_FETCH_BUCKET_NAME)x86_64/$(SINGLE_PACKAGE)-* ./packages/x86_64/ && \
        gsutil -m cp -n $(GCS_FETCH_BUCKET_NAME)aarch64/$(SINGLE_PACKAGE)-* ./packages/aarch64/

# List of package names to fetch, separated by spaces
PACKAGES ?= unknown

.PHONY: fetch-multiple-packages
fetch-multiple-packages: $(addprefix fetch-package-,$(PACKAGES))

fetch-package-%:
	@echo "Fetching package $* from GCS..."
	@$(MAKE) fetch-single-package SINGLE_PACKAGE=$*

.PHONY: fetch-all-packages
fetch-all-packages:
	echo "Fetching all packages from GCS..." && \
		mkdir -p ./packages/ && \
		gsutil -m cp -r -n 'gs://chainguard-enterprise-registry-destination/os/*' packages/
