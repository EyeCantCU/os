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

# These are separate from MELANGE_OPTS because for building we need additional
# ones that are not defined for tests.
MELANGE_TEST_OPTS += --repository-append ${REPO}
MELANGE_TEST_OPTS += --keyring-append ${KEY}.pub
MELANGE_TEST_OPTS += --arch ${ARCH}
MELANGE_TEST_OPTS += --pipeline-dirs ./pipelines/
MELANGE_TEST_OPTS += --repository-append https://packages.wolfi.dev/os
MELANGE_TEST_OPTS += --keyring-append https://packages.wolfi.dev/os/wolfi-signing.rsa.pub
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
	$(1) := $(shell if [[ "." == "$(2)" ]]; then \
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

packages/$(ARCH)/%.apk: $(KEY)
	@mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS) $(srcdirflag) ./$(pkgname)/ --log-policy builtin:stderr,$(TARGETDIR)/buildlogs/$*.log

dev-container:
	docker run --privileged --rm -it \
	    -v "${PWD}:${PWD}" \
	    -w "${PWD}" \
	    -e SOURCE_DATE_EPOCH=0 \
	    ghcr.io/wolfi-dev/sdk:latest@sha256:8404f17e6f9a4f85dbe7702d4964d4b20a8fea645eb011575edff8eb7e4722e7

PACKAGES_CONTAINER_FOLDER ?= /work/packages
TMP_REPOSITORIES_DIR := $(shell mktemp -d)
TMP_REPOSITORIES_FILE := $(TMP_REPOSITORIES_DIR)/repositories
# This target spins up a docker container that is helpful for testing local
# changes to the packages. It mounts the local packages folder as a read-only,
# and sets up the necessary keys for you to run `apk add` commands, and then
# test the packages however you see fit.
local-wolfi:
	@echo "https://packages.wolfi.dev/os" > $(TMP_REPOSITORIES_FILE)
	@echo "https://packages.cgr.dev/extras" >> $(TMP_REPOSITORIES_FILE)
	@echo "$(PACKAGES_CONTAINER_FOLDER)" >> $(TMP_REPOSITORIES_FILE)
	docker run --rm -it \
		--mount type=bind,source="${PWD}/packages",destination="$(PACKAGES_CONTAINER_FOLDER)",readonly \
		--mount type=bind,source="${PWD}/local-melange.rsa.pub",destination="/etc/apk/keys/local-melange.rsa.pub",readonly \
		--mount type=bind,source="$(TMP_REPOSITORIES_FILE)",destination="/etc/apk/repositories",readonly \
		-w "$(PACKAGES_CONTAINER_FOLDER)" \
		cgr.dev/chainguard/wolfi-base:latest
	@rm "$(TMP_REPOSITORIES_FILE)"
	@rmdir "$(TMP_REPOSITORIES_DIR)"
