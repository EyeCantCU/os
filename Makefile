ARCH ?= $(shell uname -m)
ifeq (${ARCH}, arm64)
	ARCH = aarch64
endif
TARGETDIR = packages/${ARCH}

MELANGE ?= $(shell which melange)
WOLFICTL ?= $(shell which wolfictl)
KEY ?= local-melange.rsa
REPO ?= $(shell pwd)/packages

MELANGE_OPTS += --repository-append ${REPO}
MELANGE_OPTS += --keyring-append ${KEY}.pub
MELANGE_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_OPTS += --signing-key ${KEY}
MELANGE_OPTS += --arch ${ARCH}
MELANGE_OPTS += --env-file build-${ARCH}.env
MELANGE_OPTS += --namespace chainguard
MELANGE_OPTS += --generate-index false
MELANGE_OPTS += --pipeline-dir ./pipelines/
MELANGE_OPTS += ${MELANGE_EXTRA_OPTS}

ifeq (${LINT}, yes)
	MELANGE_OPTS += --fail-on-lint-warning
endif

# The list of packages to be built. The order matters.
# wolfictl determines the list and order
# set only to be called when needed, so make can be instant to run
# when it is not
PKGLISTCMD ?= $(WOLFICTL) text --dir . --type name --pipeline-dir=./pipelines/

EXTRAS_REPO ?= https://packages.cgr.dev/extras
EXTRAS_KEY ?= https://packages.cgr.dev/extras/chainguard-extras.rsa.pub

# Add the extras repository to the list of repositories
MELANGE_OPTS += -k ${EXTRAS_KEY}
MELANGE_OPTS += -r ${EXTRAS_REPO}
PKGLISTCMD += -k ${EXTRAS_KEY} -k https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
PKGLISTCMD += -r ${EXTRAS_REPO} -r https://packages.wolfi.dev/os

# Enter interactive mode on failure for debug
MELANGE_DEBUG_OPTS += --interactive
MELANGE_DEBUG_OPTS += --debug
MELANGE_DEBUG_OPTS += --package-append apk-tools
MELANGE_DEBUG_OPTS += ${MELANGE_OPTS}

# These are separate from MELANGE_OPTS because for building we need additional
# ones that are not defined for tests.
MELANGE_TEST_OPTS += --repository-append ${REPO}
MELANGE_TEST_OPTS += --keyring-append ${KEY}.pub
MELANGE_TEST_OPTS += --arch ${ARCH}
MELANGE_TEST_OPTS += --pipeline-dirs ./pipelines/
MELANGE_TEST_OPTS += --test-package-append wolfi-base
MELANGE_TEST_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_TEST_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
MELANGE_TEST_OPTS += --repository-append ${EXTRAS_REPO}
MELANGE_TEST_OPTS += --keyring-append ${EXTRAS_KEY}
MELANGE_TEST_OPTS += ${MELANGE_EXTRA_OPTS}

all: ${KEY} .build-packages
ifeq ($(MAKECMDGOALS),all)
  PKGLIST := $(addprefix package/,$(shell $(PKGLISTCMD)))
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
	$(info $(shell $(PKGLISTCMD)))
	@printf ''

list-yaml:
	$(info $(addsuffix .yaml,$(shell $(PKGLISTCMD))))
	@printf ''

package/%:
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(info pkgver $(pkgver))
	$(MAKE) yamlfile=$(yamlfile) pkgname=$* packages/$(ARCH)/$(pkgver).apk

debug/%:
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Building package $* with version $(pkgver) from file $(yamlfile)\n"
	@mkdir -p ./"$*"/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	$(info @SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS))
	@SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_DEBUG_OPTS)  --source-dir ./$(*)/

test/%:
	$(eval yamlfile := $*.yaml)
	@if [ -z "$(yamlfile)" ]; then \
		echo "Error: could not find yaml file for $*"; exit 1; \
	else \
		echo "yamlfile is $(yamlfile)"; \
	fi
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	@printf "Testing package $* with version $(pkgver) from file $(yamlfile)\n"
	$(MELANGE) test $(yamlfile) $(MELANGE_TEST_OPTS) --source-dir ./$(*)/

packages/$(ARCH)/%.apk: $(KEY)
	@mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS) --source-dir ./$(pkgname)/

dev-container:
	docker run --privileged --rm -it \
	    -v "${PWD}:${PWD}" \
	    -w "${PWD}" \
	    -e SOURCE_DATE_EPOCH=0 \
	    ghcr.io/wolfi-dev/sdk:latest@sha256:a96b9173cb5dd9d6050ecb11b6ec326f47a32d93537838845be5e558f9103148

PACKAGES_CONTAINER_FOLDER ?= /work/packages
TMP_REPOSITORIES_DIR := $(shell mktemp -d)
TMP_REPOSITORIES_FILE := $(TMP_REPOSITORIES_DIR)/repositories
# This target spins up a docker container that is helpful for testing local
# changes to the packages. It mounts the local packages folder as a read-only,
# and sets up the necessary keys for you to run `apk add` commands, and then
# test the packages however you see fit.
local-wolfi: $(KEY)
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	@echo "https://packages.cgr.dev/extras" >> $(TMP_REPOSITORIES_FILE)
	@echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	@mkdir -p ${PWD}/packages
	docker run --rm -it \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange.rsa.pub",destination="/etc/apk/keys/local-melange.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard/wolfi-base:latest
	@rm "$(TMP_REPOSITORIES_FILE)"
	@rmdir "$(TMP_REPOSITORIES_DIR)"
