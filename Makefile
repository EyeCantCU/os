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
MELANGE_OPTS += --keyring-append chainguard-enterprise.rsa.pub
MELANGE_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_OPTS += --repository-append https://apk.cgr.dev/chainguard-private
MELANGE_OPTS += --repository-append https://packages.cgr.dev/extras
MELANGE_OPTS += --keyring-append https://packages.cgr.dev/extras/chainguard-extras.rsa.pub
MELANGE_OPTS += --arch ${ARCH}
MELANGE_OPTS += ${MELANGE_EXTRA_OPTS}

MELANGE_BUILD_OPTS += ${MELANGE_OPTS}
MELANGE_BUILD_OPTS += --signing-key ${KEY}
MELANGE_BUILD_OPTS += --pipeline-dir ./pipelines/
MELANGE_BUILD_OPTS += --env-file build-${ARCH}.env
MELANGE_BUILD_OPTS += --namespace chainguard
MELANGE_BUILD_OPTS += --license 'NONE'
MELANGE_BUILD_OPTS += --git-repo-url 'https://github.com/chainguard-dev/enterprise-packages'

MELANGE_DEBUG_TEST_OPTS += --interactive

# Enter interactive mode on failure for debug
MELANGE_DEBUG_OPTS += --interactive
MELANGE_DEBUG_OPTS += --debug
MELANGE_DEBUG_OPTS += --package-append apk-tools
MELANGE_DEBUG_OPTS += ${MELANGE_OPTS}

# These are separate from MELANGE_OPTS because for building we need additional
# ones that are not defined for tests.
MELANGE_TEST_OPTS += ${MELANGE_OPTS}
MELANGE_TEST_OPTS += --pipeline-dirs ./pipelines/
MELANGE_TEST_OPTS += --test-package-append wolfi-base
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

apk-token:
	chainctl auth login --audience apk.cgr.dev

fetch-kernel:
	$(eval KERNEL_PKG := $(shell curl -L --silent --output - --user user:$$(chainctl auth token --audience apk.cgr.dev) https://apk.cgr.dev/chainguard-private/$(ARCH)/APKINDEX.tar.gz | \
		zcat | \
		grep -a -A1 "^P:linux" | \
		grep "^V:" | \
		sort -V | \
		tail -n1 | sed -e "s/^V://"))
	@curl -s -LSo /tmp/linux.apk --user user:$(shell chainctl auth token --audience apk.cgr.dev) https://apk.cgr.dev/chainguard-private/$(ARCH)/linux-$(KERNEL_PKG).apk
	@mkdir -p /tmp/kernel
	@tar -xf /tmp/linux.apk -C /tmp/kernel/ 2>/dev/null
	export QEMU_KERNEL_IMAGE=/tmp/kernel/boot/vmlinuz
	export MELANGE_OPTS="--runner=qemu"

package/%: apk-token
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(info pkgver $(pkgver))
	$(MAKE) yamlfile=$(yamlfile) pkgname=$* packages/$(ARCH)/$(pkgver).apk

packages/$(ARCH)/%.apk: $(KEY)
	@mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_BUILD_OPTS) --source-dir ./$(pkgname)/

debug/%: apk-token
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(info pkgver $(pkgver))
	@mkdir -p ./"$*"/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_DEBUG_OPTS) $(MELANGE_BUILD_OPTS)  --source-dir ./$(*)/

test/%:
	@mkdir -p ./$(*)/
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	@HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" $(MELANGE) test $(yamlfile) $(MELANGE_TEST_OPTS) --source-dir ./$(*)/

test-debug/%:
	@mkdir -p ./$(*)/
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	@HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" $(MELANGE) test $(yamlfile) $(MELANGE_TEST_OPTS) $(MELANGE_DEBUG_TEST_OPTS) --source-dir ./$(*)/

dev-container:
	docker run --privileged --rm -it \
			-v "${PWD}:${PWD}" \
			-v "${HOME}/.cache/wolfictl/dev-container-enterprise/root:/root" \
			-v "${HOME}/.config/chainctl:/root/.config/chainctl" \
			-w "${PWD}" \
			ghcr.io/wolfi-dev/sdk:latest@sha256:01b0a2b01db2522c23309f18db23b213b75536a3c126b49c4a83cbe714b98fe0

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
local-wolfi: ${KEY}
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	@echo "https://apk.cgr.dev/chainguard-private" >> $(TMP_REPOSITORIES_FILE)
	@echo "https://packages.cgr.dev/extras" >> $(TMP_REPOSITORIES_FILE)
	@echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	@mkdir -p ${PWD}/packages
	docker run --rm -it \
		-e HTTP_AUTH="basic:apk.cgr.dev:user:$(shell chainctl auth token --audience apk.cgr.dev)" \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange-enterprise.rsa.pub",destination="/etc/apk/keys/local-melange-enterprise.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard-private/chainguard-base:latest
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
		ghcr.io/wolfi-dev/sdk:latest@sha256:01b0a2b01db2522c23309f18db23b213b75536a3c126b49c4a83cbe714b98fe0
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
