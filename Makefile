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
PKGLISTCMD += -k ${EXTRAS_KEY}
PKGLISTCMD += -r ${EXTRAS_REPO}

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
	$(eval pkgver := $(shell $(MELANGE) package-version $(yamlfile)))
	$(MAKE) yamlfile=$(yamlfile) pkgname=$* packages/$(ARCH)/$(pkgver).apk

packages/$(ARCH)/%.apk: $(KEY)
	@mkdir -p ./$(pkgname)/
	$(eval SOURCE_DATE_EPOCH ?= $(shell git log -1 --pretty=%ct --follow $(yamlfile)))
	@SOURCE_DATE_EPOCH=$(SOURCE_DATE_EPOCH) $(MELANGE) build $(yamlfile) $(MELANGE_OPTS) --source-dir ./$(pkgname)/ --log-policy builtin:stderr,$(TARGETDIR)/buildlogs/$*.log

dev-container:
	docker run --privileged --rm -it \
	    -v "${PWD}:${PWD}" \
	    -w "${PWD}" \
	    -e SOURCE_DATE_EPOCH=0 \
	    ghcr.io/wolfi-dev/sdk:latest@sha256:d1d9112ba405077785787e5c0f97c3f544e7e2f786292040aa908e29ca48ca25
