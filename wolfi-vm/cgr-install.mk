CGR_INSTALL = cgr-install
CGR_INSTALL_DIR = cgr-install
KEY = $(CGR_INSTALL_DIR)/local-melange.rsa
CGR_INSTALL_REPO = $(CGR_INSTALL_DIR)/packages
CGR_INSTALL_VERSION = 0.0.1-r0
CGR_INSTALL_YAML = $(CGR_INSTALL_DIR)/$(CGR_INSTALL).yaml

MELANGE ?= melange

CGR_INSTALL_APK = $(CGR_INSTALL_REPO)/$(ARCH)/$(CGR_INSTALL)-$(CGR_INSTALL_VERSION).apk

MELANGE_OPTS += --repository-append=$(CGR_INSTALL_REPO)
MELANGE_OPTS += --keyring-append=$(KEY).pub
MELANGE_OPTS += --repository-append=https://apk.cgr.dev/chainguard
MELANGE_OPTS += --repository-append=https://apk.cgr.dev/chainguard-private
MELANGE_BUILD_OPTS += $(MELANGE_OPTS)
MELANGE_BUILD_OPTS += --out-dir=$(CGR_INSTALL_REPO)
MELANGE_BUILD_OPTS += --license=NONE
MELANGE_BUILD_OPTS += --signing-key=$(KEY)
MELANGE_BUILD_OPTS += --namespace=chainguard
MELANGE_BUILD_OPTS += --git-repo-url=https://github.com/chainguard-dev/wolfi-vm/

cgr-install-apk: $(CGR_INSTALL_APK)

# the --arch substitution below is due to the % here matching 'builder-<arch>'
# but needing --arch=<arch>
$(CGR_INSTALL_REPO)/%/$(CGR_INSTALL)-$(CGR_INSTALL_VERSION).apk: $(KEY) $(CGR_INSTALL_YAML) $(wildcard $(CGR_INSTALL_DIR)/scripts/*)
	@mkdir -p ./$(dir $@)
	@echo building $@
	@sde=$$(git log -1 --pretty=%ct HEAD) && \
	  tok=$$(chainctl auth token --audience=apk.cgr.dev) && \
	  export HTTP_AUTH="basic:apk.cgr.dev:user:$$tok" SOURCE_DATE_EPOCH="$$sde" && \
	  set -- $(MELANGE) build --arch=$(lastword $(subst -, ,$*)) $(CGR_INSTALL_YAML) $(MELANGE_BUILD_OPTS) && \
	  echo "$$@" 1>&2 && "$$@"

$(KEY):
	$(MELANGE) keygen $(KEY)
